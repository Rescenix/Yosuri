package handler

// mambo_tool.go —— 曼波视频一键生成工具（2026-08-06）
//
// mambo_video 工具：输入主题（可带分段文案），一键产出曼波风格竖屏短视频。
// 引擎是 Python 脚本 scripts/mambo_video.py（edge-tts 配音 + 精确 SRT 字幕 +
// 素材池关键词匹配 + ffmpeg 竖屏合成），Go 侧负责参数透传与结果返回：
//   - 模型侧：拿到视频/字幕/manifest 路径 JSON，可向用户汇报产物；
//   - 素材匹配：按每句关键词在素材池目录（默认 main-backend/assets/mambo/）里
//     匹配图片/视频文件名，命中即用；无命中则动态渐变背景 + 大字兜底。
//
// 产物：<out>.mp4（1080x1920）+ <out>.srt（字幕，与语音对齐）+ <out>.manifest.json。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"backend/internal/ai/core"
)

// mamboToolDef mambo_video 工具定义（随 nativeOnDemandToolDefs 按需加载）。
var mamboToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        "mambo_video",
		Description: "曼波视频一键生成：输入主题（可选分段文案、素材池目录），自动完成 配音（edge-tts 中文快节奏曼波音色）→ 精确字幕（SRT，与语音对齐）→ 按每句关键词匹配素材（本地素材池，无命中用动态渐变背景+大字兜底）→ ffmpeg 竖屏 1080x1920 合成。返回视频/字幕/manifest 的绝对路径 JSON。适合做口播短视频、梗视频、卡点视频素材。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"topic": {
					Type:        "string",
					Description: "视频主题（必填），如「曼波」「考研上岸」「加班」",
				},
				"text": {
					Type:        "string",
					Description: "可选：分段文案，用 | 分隔（如「曼波曼波，跳舞啦！|音乐响起，节奏飞起！」）。缺省时引擎用内置曼波模板（主题自动嵌入）",
				},
				"media_dir": {
					Type:        "string",
					Description: "可选：素材池目录（图片 jpg/png/webp/gif、视频 mp4/webm/mov，按文件名关键词匹配）。缺省 main-backend/assets/mambo/",
				},
				"voice": {
					Type:        "string",
					Description: "可选：edge-tts 中文音色，默认 zh-CN-XiaoxiaoNeural（晓晓·温暖女声，科普耐听），可选 zh-CN-XiaoyiNeural（晓伊·活泼女声）、zh-CN-YunxiNeural（云希·男声）",
				},
				"rate": {
					Type:        "string",
					Description: "可选：语速，默认 +30%（曼波节奏感），如 +10% / -10%",
				},
				"out": {
					Type:        "string",
					Description: "可选：输出 mp4 路径，默认 test_output/mambo_<主题>.mp4",
				},
				"pexels_key": {
					Type:        "string",
					Description: "可选：Pexels API Key（有 key 时优先用 Pexels 搜真实视频素材，免 key 降级 Pixabay 页面爬取）",
				},
			},
			Required: []string{"topic"},
		},
	},
}

// backendRoot 定位 main-backend 根目录（scripts/ 在其下）
func backendRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// 从 cwd 逐级向上找 scripts/mambo_video.py
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "scripts", "mambo_video.py")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// 兜底：用本文件所在目录推算（internal/handler/../..）
	_, file, _, _ := runtime.Caller(0)
	base := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	if _, err := os.Stat(filepath.Join(base, "scripts", "mambo_video.py")); err == nil {
		return base, nil
	}
	return "", fmt.Errorf("找不到 scripts/mambo_video.py（cwd=%s）", cwd)
}

// findPython 定位带 edge-tts 的 python：优先 PATH 里的 python，其次 hermes venv
func findPython() (string, error) {
	if p, err := exec.LookPath("python"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("python3"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("找不到 python（需要 edge-tts 库）")
}

// callMamboVideo mambo_video 工具实现：透传参数调 Python 引擎
func callMamboVideo(ctx context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		Topic    string `json:"topic"`
		Text     string `json:"text"`
		MediaDir string `json:"media_dir"`
		Voice    string `json:"voice"`
		Rate     string `json:"rate"`
		Out      string `json:"out"`
		PexelsKey string `json:"pexels_key"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
	}
	if strings.TrimSpace(args.Topic) == "" {
		return nativeToolResult{}, fmt.Errorf("topic 必填")
	}

	root, err := backendRoot()
	if err != nil {
		return nativeToolResult{}, err
	}
	py, err := findPython()
	if err != nil {
		return nativeToolResult{}, err
	}
	script := filepath.Join(root, "scripts", "mambo_video.py")

	cmdArgs := []string{script, "--topic", args.Topic}
	if args.Text != "" {
		cmdArgs = append(cmdArgs, "--text", args.Text)
	}
	if args.MediaDir != "" {
		cmdArgs = append(cmdArgs, "--media", args.MediaDir)
	}
	if args.Voice != "" {
		cmdArgs = append(cmdArgs, "--voice", args.Voice)
	}
	if args.Rate != "" {
		cmdArgs = append(cmdArgs, "--rate", args.Rate)
	}
	if args.Out != "" {
		cmdArgs = append(cmdArgs, "--out", args.Out)
	}
	if args.PexelsKey != "" {
		cmdArgs = append(cmdArgs, "--pexels-key", args.PexelsKey)
	}

	cmd := hiddenCommandContext(ctx, py, cmdArgs...)
	cmd.Dir = root // 引擎内部相对路径以 main-backend 为基准
	// stdout 必须是纯 JSON（引擎日志走 stderr），失败时取 stderr 尾部报错
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nativeToolResult{}, fmt.Errorf("曼波视频生成失败: %s",
				truncateTail(string(ee.Stderr), 600))
		}
		return nativeToolResult{}, fmt.Errorf("曼波视频生成失败: %w", err)
	}
	return nativeToolResult{Text: string(out)}, nil
}

func truncateTail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
