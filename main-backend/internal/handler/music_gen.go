// music_gen.go —— AI 音乐生成工具（2026-09-08）
//
// 架构：用户零 key 零配置。LLM 调 music_generate → 本处理器把请求转发到
// ResceneCloud 云端代理（/api/music/generate，key 只在云后端 env 持有）→
// 拿到音频字节流 → 落盘本地媒体目录 → 返回音频工件，
// 前端在聊天里内嵌可播放的音频卡片（同视频工件模式）。
//
// 云端不可用/超时 → 返回可读中文错误，绝不伪造音频。

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend/internal/ai/core"
)

// musicGenToolDef 音乐生成工具定义（模型可见）。
var musicGenToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        "music_generate",
		Description: "AI 生成音乐并内嵌到聊天里（云端代理，无需 API key）：输入风格/情绪/乐器描述，生成一段音乐（默认纯音乐 BGM）。可带歌词生成完整歌曲。适合用户要背景音乐、旋律、歌曲或任何「来一段音乐」的场景。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"prompt": {
					Type:        "string",
					Description: "音乐描述（必填）：风格+情绪+乐器+速度。如「古风 笛子 慢节奏 思念」「轻快 电子 咖啡馆 钢琴」",
				},
				"lyrics": {
					Type:        "string",
					Description: "可选，歌词（可用 [verse]/[chorus] 分段）；不填=纯音乐 BGM",
				},
				"title": {
					Type:        "string",
					Description: "可选，歌名",
				},
				"type": {
					Type:        "string",
					Description: "可选：bgm（默认，纯音乐）/ song（带词成曲）/ instrumental",
				},
			},
			Required: []string{"prompt"},
		},
	},
}

// musicCloudURL 云端音乐代理地址（key 在云端 env，re0 零密钥）。
func musicCloudURL() string {
	return cloudAuthBase() + "/api/music/generate"
}

// mediaDir 媒体落盘目录（与图片/视频同根：~/rescene_data/media）。
func mediaDir() string {
	if root := strings.TrimSpace(os.Getenv("RESCENE_MEDIA_DIR")); root != "" {
		return root
	}
	return filepath.Join(resceneUserDataDir(), "media")
}

// cloudBaseURL 云端服务根（与 cloud_auth.go 同一来源，开放 re0 直连）。
func cloudBaseURL() string {
	return cloudAuthBase()
}

// callNativeMusicGenerate 处理 music_generate 工具调用。
func callNativeMusicGenerate(ctx context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		Prompt string `json:"prompt"`
		Lyrics string `json:"lyrics"`
		Title  string `json:"title"`
		Type   string `json:"type"`
	}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}
	}
	prompt := strings.TrimSpace(args.Prompt)
	if prompt == "" {
		return nativeToolResult{}, fmt.Errorf("music_generate 需要 prompt（音乐描述）")
	}

	payload, _ := json.Marshal(map[string]any{
		"prompt": prompt,
		"lyrics": args.Lyrics,
		"title":  args.Title,
		"type":   args.Type,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, musicCloudURL(), strings.NewReader(string(payload)))
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// 云端音乐接口是 AuthOrGuest（公益共享）：无需用户 token 即可调用，
	// 滥用在云端限流，re0 侧不持有/不透传任何密钥。
	_ = req // 保留结构清晰

	client := &http.Client{Timeout: 150 * time.Second} // 音乐生成慢，宽超时
	resp, err := client.Do(req)
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("音乐生成服务请求失败（云端不可达？）: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		return nativeToolResult{}, fmt.Errorf("音乐生成失败（云端 HTTP %d）: %s", resp.StatusCode, truncate(string(body), 300))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 30<<20)) // 30MB 上限
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("读取音频失败: %w", err)
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "audio/mpeg"
	}
	ext := ".mp3"
	if strings.Contains(ct, "wav") {
		ext = ".wav"
	} else if strings.Contains(ct, "ogg") {
		ext = ".ogg"
	}

	// 落盘本地媒体目录
	dir := mediaDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nativeToolResult{}, fmt.Errorf("创建媒体目录失败: %w", err)
	}
	name := fmt.Sprintf("music_%d%s", time.Now().Unix(), ext)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nativeToolResult{}, fmt.Errorf("音频落盘失败: %w", err)
	}

	// 前端可播放路径（静态服务挂载媒体目录）
	url := "/api/media/" + name

	text := fmt.Sprintf("已生成音乐（%s）\n本地路径: %s\n预览地址: %s",
		strings.TrimSpace(prompt), path, url)

	return nativeToolResult{
		Text: text,
		Audios: []mcpAudioArtifact{{
			URL:  url,
			File: path,
			Mime: ct,
			Size: fmt.Sprintf("%d", len(data)),
		}},
	}, nil
}