package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	nethtml "golang.org/x/net/html"
)

func callNativeCommand(parent context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
	}
	if strings.TrimSpace(args.Command) == "" {
		return nativeToolResult{}, fmt.Errorf("command 不能为空")
	}
	if args.Timeout <= 0 {
		args.Timeout = 120
	}
	if args.Timeout > 600 {
		args.Timeout = 600
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(args.Timeout)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", args.Command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-lc", args.Command)
	}
	cmd.Dir = filepath.Clean(absAgainstRoot("."))
	stdout := newCappedCommandBuffer(1024 * 1024)
	stderr := newCappedCommandBuffer(1024 * 1024)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nativeToolResult{}, fmt.Errorf("命令超时（%ds）", args.Timeout)
	}
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			return nativeToolResult{}, err
		}
	}
	out := fmt.Sprintf("退出码: %d", exitCode)
	if stdout.Len() > 0 {
		out += "\nstdout:\n" + truncateChars(stdout.String(), codeResultMaxChars/2)
	}
	if stderr.Len() > 0 {
		out += "\nstderr:\n" + truncateChars(stderr.String(), codeResultMaxChars/2)
	}
	if exitCode != 0 {
		return nativeToolResult{}, fmt.Errorf("%s", out)
	}
	return nativeToolResult{Text: out}, nil
}

type cappedCommandBuffer struct {
	buf       bytes.Buffer
	remaining int
	truncated bool
}

func newCappedCommandBuffer(maxBytes int) cappedCommandBuffer {
	return cappedCommandBuffer{remaining: maxBytes}
}

func (b *cappedCommandBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	if originalLen > b.remaining {
		b.truncated = true
	}
	if b.remaining > 0 {
		keep := len(p)
		if keep > b.remaining {
			keep = b.remaining
		}
		_, _ = b.buf.Write(p[:keep])
		b.remaining -= keep
	}
	return originalLen, nil
}

func (b *cappedCommandBuffer) Len() int {
	return b.buf.Len()
}

func (b *cappedCommandBuffer) String() string {
	out := b.buf.String()
	if b.truncated {
		out += "\n…（命令输出已在 1MB 处截断）"
	}
	return out
}

func callNativeWebFetch(parent context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		URL      string `json:"url"`
		MaxChars int    `json:"max_chars"`
	}
	if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
	}
	u, err := url.Parse(args.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nativeToolResult{}, fmt.Errorf("url 必须是有效的 http(s) 地址")
	}
	if args.MaxChars <= 0 {
		args.MaxChars = 8000
	}
	if args.MaxChars > 30000 {
		args.MaxChars = 30000
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", "Rescene/1.0 (+https://github.com/Rescenix/re0)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nativeToolResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1000))
		return nativeToolResult{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nativeToolResult{}, err
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	text := string(body)
	if strings.Contains(contentType, "html") || strings.Contains(strings.ToLower(text[:minInt(len(text), 256)]), "<html") {
		text = readableHTMLText(body)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nativeToolResult{Text: "页面未提取到可读正文（可能依赖 JavaScript 渲染）"}, nil
	}
	return nativeToolResult{Text: truncateChars(text, args.MaxChars)}, nil
}

func readableHTMLText(body []byte) string {
	doc, err := nethtml.Parse(bytes.NewReader(body))
	if err != nil {
		return string(body)
	}
	var b strings.Builder
	var walk func(*nethtml.Node, bool)
	walk = func(n *nethtml.Node, hidden bool) {
		if n.Type == nethtml.ElementNode {
			switch strings.ToLower(n.Data) {
			case "script", "style", "noscript", "svg":
				hidden = true
			case "p", "div", "section", "article", "main", "header", "footer", "li", "h1", "h2", "h3", "h4", "h5", "h6", "br":
				b.WriteByte('\n')
			}
		}
		if n.Type == nethtml.TextNode && !hidden {
			t := strings.Join(strings.Fields(n.Data), " ")
			if t != "" {
				b.WriteString(t)
				b.WriteByte(' ')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, hidden)
		}
	}
	walk(doc, false)
	lines := strings.Split(b.String(), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.Join(strings.Fields(line), " "))
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func callNativeViewImage(parent context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		Path        string `json:"path"`
		ImageURL    string `json:"image_url"`
		ImageBase64 string `json:"image_base64"`
		Question    string `json:"question"`
	}
	if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
	}
	imageBase64 := args.ImageBase64
	if comma := strings.Index(imageBase64, ","); strings.HasPrefix(imageBase64, "data:") && comma >= 0 {
		imageBase64 = imageBase64[comma+1:]
	}
	if imageBase64 == "" && args.Path != "" {
		path, err := nativeAbsPath(args.Path)
		if err != nil {
			return nativeToolResult{}, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nativeToolResult{}, err
		}
		if len(data) > 20*1024*1024 {
			return nativeToolResult{}, fmt.Errorf("图片超过 20MB")
		}
		imageBase64 = base64.StdEncoding.EncodeToString(data)
	}
	if imageBase64 == "" && args.ImageURL != "" {
		ctx, cancel := context.WithTimeout(parent, 30*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, args.ImageURL, nil)
		if err != nil {
			return nativeToolResult{}, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nativeToolResult{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nativeToolResult{}, fmt.Errorf("下载图片返回 HTTP %d", resp.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 20*1024*1024+1))
		if err != nil {
			return nativeToolResult{}, err
		}
		if len(data) > 20*1024*1024 {
			return nativeToolResult{}, fmt.Errorf("图片超过 20MB")
		}
		imageBase64 = base64.StdEncoding.EncodeToString(data)
	}
	if imageBase64 == "" {
		return nativeToolResult{}, fmt.Errorf("path、image_url、image_base64 至少提供一个")
	}
	if args.Question == "" {
		args.Question = "请详细描述这张图片的内容"
	}
	text, err := AnalyzeImage(imageBase64, args.Question, nil)
	if err != nil {
		return nativeToolResult{}, err
	}
	return nativeToolResult{Text: text}, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
