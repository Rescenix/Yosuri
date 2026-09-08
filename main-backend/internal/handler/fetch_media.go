// fetch_media.go —— 网络素材抓取工具（2026-09-08）
//
// 通用媒体采集：agent 给一个网络 URL（图片/音频/视频/文档），工具下载到本地
// media 目录并按类型产出对应工件，前端聊天内嵌展示（音频播放条/视频播放块/
// 图片内联/文件交付卡）。用途：把用户提供的链接素材、或从网页里扒到的媒体
// 直接落到对话里，不用再让用户手动下载上传。
//
// 类型判定：优先 Content-Type，其次扩展名兜底。图片转 base64 内联
// （前端 image 工件契约），音/视频走 /api/media 本地 URL。

package handler

import (
	"context"
	"encoding/base64"
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

// fetchMediaToolDef 素材抓取工具定义（模型可见）。
var fetchMediaToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        "fetch_media",
		Description: "下载网络素材到本地并内嵌到聊天：给定一个 http(s) 图片/音频/视频/文件 URL，工具下载后按类型展示（图片直接内联、音乐/视频可播放、文档作为交付文件）。适合用户给了素材链接、或你在网页里发现素材想让对方直接看/听/用时使用。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"url": {
					Type:        "string",
					Description: "素材 URL（必填）：http(s) 直链，图片/音频/视频/文档均可",
				},
				"caption": {
					Type:        "string",
					Description: "可选，展示说明（来源/用途一句话）",
				},
			},
			Required: []string{"url"},
		},
	},
}

// callNativeFetchMedia 处理 fetch_media 工具调用。
func callNativeFetchMedia(ctx context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		URL     string `json:"url"`
		Caption string `json:"caption"`
	}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}
	}
	rawURL := strings.TrimSpace(args.URL)
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return nativeToolResult{}, fmt.Errorf("url 必须是 http(s) 地址")
	}
	caption := strings.TrimSpace(args.Caption)
	if caption == "" {
		caption = "来源于网络素材"
	}

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Yosuri Agent)")
	resp, err := client.Do(req)
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nativeToolResult{}, fmt.Errorf("下载失败（HTTP %d）", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 30<<20)) // 30MB 上限
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("读取素材失败: %w", err)
	}
	if len(data) == 0 {
		return nativeToolResult{}, fmt.Errorf("素材为空")
	}

	// 类型判定：Content-Type 优先，扩展名兜底
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	ct = strings.Split(ct, ";")[0]
	ext := extFromURL(rawURL, ct)

	if err := os.MkdirAll(mediaDir(), 0o755); err != nil {
		return nativeToolResult{}, fmt.Errorf("创建媒体目录失败: %w", err)
	}
	name := fmt.Sprintf("media_%d%s", time.Now().Unix(), ext)
	path := filepath.Join(mediaDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nativeToolResult{}, fmt.Errorf("素材落盘失败: %w", err)
	}

	kind := classifyMedia(ct, ext)
	url := "/api/media/" + name

	switch kind {
	case "image":
		return nativeToolResult{
			Text: fmt.Sprintf("已获取图片素材\n本地路径: %s", path),
			Images: []mcpImageArtifact{{
				Data:     base64.StdEncoding.EncodeToString(data),
				MimeType: ct,
			}},
		}, nil
	case "video":
		return nativeToolResult{
			Text: fmt.Sprintf("已获取视频素材\n本地文件: %s\n预览: %s\n%s", path, url, caption),
			Videos: []mcpVideoArtifact{{
				URL: url, File: path, Mime: ct, Size: fmt.Sprintf("%d", len(data)),
			}},
			Files: []fileDeliverable{{
				Path: path, Name: name, Ext: ext, Size: int64(len(data)),
			}},
		}, nil
	case "audio":
		return nativeToolResult{
			Text: fmt.Sprintf("已获取音频素材\n本地文件: %s\n预览: %s\n%s", path, url, caption),
			Audios: []mcpAudioArtifact{{
				URL: url, File: path, Mime: ct, Size: fmt.Sprintf("%d", len(data)),
			}},
			Files: []fileDeliverable{{
				Path: path, Name: name, Ext: ext, Size: int64(len(data)),
			}},
		}, nil
	default:
		return nativeToolResult{
			Text: fmt.Sprintf("已获取文件素材\n本地路径: %s\n%s", path, caption),
			Files: []fileDeliverable{{
				Path: path, Name: name, Ext: ext, Size: int64(len(data)),
			}},
		}, nil
	}
}

// classifyMedia 按 Content-Type 和扩展名判素材类型。
func classifyMedia(ct, ext string) string {
	ct = strings.ToLower(ct)
	ext = strings.ToLower(ext)
	if strings.HasPrefix(ct, "image/") || map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true, ".svg": true,
	}[ext] {
		return "image"
	}
	if strings.HasPrefix(ct, "video/") || ext == ".mp4" || ext == ".webm" || ext == ".mov" {
		return "video"
	}
	if strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "application/ogg") ||
		ext == ".mp3" || ext == ".wav" || ext == ".ogg" || ext == ".m4a" || ext == ".flac" {
		return "audio"
	}
	return "file"
}

// extFromURL 从 URL 路径/Content-Type 推断扩展名。
func extFromURL(rawURL, ct string) string {
	clean := strings.Split(rawURL, "?")[0]
	lower := strings.ToLower(clean)
	for _, e := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg",
		".mp4", ".webm", ".mov", ".mp3", ".wav", ".ogg", ".m4a", ".flac",
		".pdf", ".docx", ".pptx", ".xlsx", ".md", ".txt", ".html"} {
		if strings.HasSuffix(lower, e) {
			return e
		}
	}
	switch {
	case strings.HasPrefix(ct, "image/png"):
		return ".png"
	case strings.HasPrefix(ct, "image/jpeg"):
		return ".jpg"
	case strings.HasPrefix(ct, "image/webp"):
		return ".webp"
	case strings.HasPrefix(ct, "audio/mpeg"):
		return ".mp3"
	case strings.HasPrefix(ct, "audio/wav"):
		return ".wav"
	case strings.HasPrefix(ct, "video/mp4"):
		return ".mp4"
	case strings.HasPrefix(ct, "video/webm"):
		return ".webm"
	case strings.HasPrefix(ct, "application/pdf"):
		return ".pdf"
	}
	return ".bin"
}