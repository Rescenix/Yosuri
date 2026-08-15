package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type pptSource struct {
	Topic  string     `json:"topic"`
	Slides []pptSlide `json:"slides"`
}

type pptSlide struct {
	Title  string   `json:"title"`
	Points []string `json:"points"`
}

func parsePPTMarkdown(topic, content string) pptSource {
	source := pptSource{Topic: strings.TrimSpace(topic)}
	var current *pptSlide
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "## ") {
			source.Slides = append(source.Slides, pptSlide{Title: strings.TrimSpace(strings.TrimPrefix(line, "## "))})
			current = &source.Slides[len(source.Slides)-1]
			continue
		}
		if current != nil && (strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ")) {
			current.Points = append(current.Points, strings.TrimSpace(line[2:]))
		}
	}
	return source
}

func findRepoFile(parts ...string) string {
	dir, _ := os.Getwd()
	for {
		candidate := filepath.Join(append([]string{dir}, parts...)...)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func renderPPTX(outDir, topic, content string) (string, error) {
	source := parsePPTMarkdown(topic, content)
	if len(source.Slides) == 0 {
		return "", fmt.Errorf("PPT 大纲没有可渲染页面")
	}
	script := findRepoFile("main-frontend", "beneficial-belt", "scripts", "render-agent-pptx.mjs")
	if script == "" {
		return "", fmt.Errorf("找不到 PPTX 渲染器")
	}
	stamp := fmt.Sprintf("%s-%02d", time.Now().Format("2006-01-02"), time.Now().Unix()%100)
	input := filepath.Join(outDir, ".ppt-source-"+stamp+".json")
	data, _ := json.Marshal(source)
	if err := os.WriteFile(input, data, 0o600); err != nil {
		return "", err
	}
	defer os.Remove(input)
	name := "PPT-" + stamp + ".pptx"
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script, input, filepath.Join(outDir, name))
	cmd.Dir = filepath.Dir(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("PPTX 渲染失败: %s", strings.TrimSpace(string(output)))
	}
	return name, nil
}

var narrationLine = regexp.MustCompile(`(?m)^\s*-\s*旁白[:：]\s*(.+?)\s*$`)

func renderPV(outDir, topic, scriptText string) (string, error) {
	return renderPVWithMedia(outDir, topic, scriptText, "")
}

func renderPVWithMedia(outDir, topic, scriptText, mediaDir string) (string, error) {
	engine := findRepoFile("main-backend", "scripts", "mambo_video.py")
	if engine == "" {
		return "", fmt.Errorf("找不到视频渲染引擎")
	}
	lines := narrationLine.FindAllStringSubmatch(scriptText, -1)
	segments := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) > 1 && strings.TrimSpace(line[1]) != "" {
			segments = append(segments, strings.TrimSpace(line[1]))
		}
	}
	if len(segments) == 0 {
		return "", fmt.Errorf("PV 脚本没有可配音的旁白")
	}
	stamp := fmt.Sprintf("%s-%02d", time.Now().Format("2006-01-02"), time.Now().Unix()%100)
	name := "PV-" + stamp + ".mp4"
	python := "python"
	if _, err := exec.LookPath(python); err != nil {
		python = "python3"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	args := []string{engine, "--topic", topic, "--text", strings.Join(segments, "|"), "--out", filepath.Join(outDir, name), "--no-online"}
	if strings.TrimSpace(mediaDir) != "" {
		args = append(args, "--media", mediaDir, "--ordered-media", "--width", "1920", "--height", "1080")
	}
	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Dir = filepath.Dir(filepath.Dir(engine))
	if output, err := cmd.CombinedOutput(); err != nil {
		text := strings.TrimSpace(string(output))
		if len(text) > 700 {
			text = text[len(text)-700:]
		}
		return "", fmt.Errorf("PV 渲染失败: %s", text)
	}
	return name, nil
}

// renderPPTXSlides 使用本机 PowerPoint 将真实幻灯片逐页导出，供会议回放按顺序使用。
func renderPPTXSlides(pptxPath, outputDir string) error {
	if strings.TrimSpace(pptxPath) == "" {
		return fmt.Errorf("PPTX 路径为空")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	scriptPath := filepath.Join(outputDir, ".export-slides.ps1")
	script := `param([string]$Deck,[string]$Out)
$ErrorActionPreference='Stop'
$powerpoint=$null
$presentation=$null
try {
  $powerpoint=New-Object -ComObject PowerPoint.Application
  $presentation=$powerpoint.Presentations.Open($Deck,$true,$false,$false)
  foreach($slide in $presentation.Slides){
    $name=('slide-{0:D3}.png' -f [int]$slide.SlideIndex)
    $slide.Export((Join-Path $Out $name),'PNG',1920,1080)
  }
} finally {
  if($presentation -ne $null){$presentation.Close()}
  if($powerpoint -ne $null){$powerpoint.Quit()}
}`
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return err
	}
	defer os.Remove(scriptPath)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-File", scriptPath, pptxPath, outputDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("PowerPoint 逐页导出失败: %s", strings.TrimSpace(string(output)))
	}
	images, _ := filepath.Glob(filepath.Join(outputDir, "slide-*.png"))
	if len(images) == 0 {
		return fmt.Errorf("PowerPoint 未导出任何幻灯片")
	}
	return nil
}
