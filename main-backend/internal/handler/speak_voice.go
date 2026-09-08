// speak_voice.go —— AI 语音工具（2026-09-08）
//
// Hermes 同款：agent 调 speak 文本转语音 → 本地 edge-tts 合成 mp3 →
// 落盘 media 目录 → 音频工件，前端聊天内嵌播放条（同 video 块模式）。
// 零 key 零下载：edge-tts 是微软免费 TTS，纯本机合成，不依赖云端。

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend/internal/ai/core"
)

// speakVoiceToolDef 语音工具定义（模型可见）。
var speakVoiceToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        "speak",
		Description: "TTS 语音合成：把一段文本转成语音音频（mp3），内嵌到聊天里可直接播放。适合用户要「读出来/语音版/语音提示」或想听内容朗读时。使用本地免费引擎，无需 API key。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"text": {
					Type:        "string",
					Description: "要朗读的文本（必填），情绪语调用标点表达",
				},
				"voice": {
					Type:        "string",
					Description: "可选，音色：zh-CN-XiaoxiaoNeural（默认，晓晓）/ zh-CN-XiaoyiNeural（晓伊）/ zh-CN-YunxiNeural（晓希男声）/ en-US-AriaNeural（英语）",
				},
			},
			Required: []string{"text"},
		},
	},
}

// callNativeSpeakVoice 处理 speak 工具调用：edge-tts 合成 → 落盘 → 音频工件。
func callNativeSpeakVoice(ctx context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		Text  string `json:"text"`
		Voice string `json:"voice"`
	}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}
	}
	text := strings.TrimSpace(args.Text)
	if text == "" {
		return nativeToolResult{}, fmt.Errorf("speak 需要 text（要朗读的内容）")
	}
	if len([]rune(text)) > 800 {
		return nativeToolResult{}, fmt.Errorf("文本太长了（最多 800 字），建议分段朗读")
	}
	voice := strings.TrimSpace(args.Voice)
	if voice == "" {
		voice = "zh-CN-XiaoxiaoNeural"
	}

	dir := mediaDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nativeToolResult{}, fmt.Errorf("创建媒体目录失败: %w", err)
	}
	name := fmt.Sprintf("speak_%d.mp3", time.Now().Unix())
	outPath := filepath.Join(dir, name)

	// 本机 edge-tts：找可用的 python + edge-tts 模块
	py, err := findEdgeTTSPython()
	if err != nil {
		return nativeToolResult{}, err
	}
	cmd := hiddenCommandContext(ctx, py, "-m", "edge_tts",
		"--voice", voice,
		"--text", text,
		"--write-media", outPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nativeToolResult{}, fmt.Errorf("语音合成失败: %s", strings.TrimSpace(string(out)))
	}
	info, err := os.Stat(outPath)
	if err != nil || info.Size() == 0 {
		return nativeToolResult{}, fmt.Errorf("语音合成失败：未生成音频文件")
	}

	url := "/api/media/" + name
	res := nativeToolResult{
		Text: fmt.Sprintf("已生成配音（%s）\n本地文件: %s\n预览: %s", voice, outPath, url),
		Audios: []mcpAudioArtifact{{
			URL:  url,
			File: outPath,
			Mime: "audio/mpeg",
			Size: fmt.Sprintf("%d", info.Size()),
		}},
	}
	// 配音是交付物：同时挂到文件交付卡（可下载、可复用到视频/剪辑），
	// 而不仅仅是聊天里的播放条——用户要的是「配音成品」，不是「听一下」。
	res.Files = []fileDeliverable{{
		Path: outPath,
		Name: name,
		Ext:  ".mp3",
		Size: info.Size(),
	}}
	return res, nil
}

// findEdgeTTSPython 定位能跑 edge_tts 的 python：hermes venv → 系统 python → uv。
func findEdgeTTSPython() (string, error) {
	candidates := []string{
		"python", // PATH 里的（含 venv 激活态）
	}
	for _, c := range candidates {
		cmd := hiddenCommand(c, "-c", "import edge_tts")
		if cmd.Run() == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("未找到可用的 Python+edge-tts（本机语音合成引擎缺失）")
}