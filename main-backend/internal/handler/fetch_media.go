// fetch_media.go —— 本地媒体素材内嵌工具（2026-09-08 初版，09-14 改为只收本地路径）
//
// 通用媒体采集：agent 给一个本地文件路径（图片/音频/视频/文档），工具把文件
// 归入媒体目录并按类型产出对应工件，前端聊天内嵌展示（音频播放条/视频播放块/
// 图片内联/文件交付卡）。用途：把本地已有素材（bash/curl 下载的、用户放的、
// 工具生成的）直接落到对话里。
//
// 输入形态：本地绝对路径或相对路径（经 nativeAbsPath 解析）。只收本地文件——
// 联网视频说到底也要先落到本地，下载是 bash/curl 的职责，fetch_media 不再收
// http(s) URL（曾收 URL 导致 agent 绕 python -m http.server 把本地文件暴露成
// 地址再抓一遍，用户每回都要起服务，09-14 砍掉）。
//
// 类型判定：http.DetectContentType 优先，扩展名兜底。图片转 base64 内联
// （前端 image 工件契约），音/视频走 /api/media 本地 URL。

package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend/internal/ai/core"
)

// fetchMediaToolDef 素材内嵌工具定义（模型可见）。
var fetchMediaToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        "fetch_media",
		Description: "把本地媒体文件内嵌到聊天：给定一个本地文件路径（图片/音频/视频/文档），工具归入媒体目录后按类型展示（图片直接内联、音乐/视频可播放、文档作为交付文件）。路径支持绝对路径或相对路径；远程素材请先用 bash/curl 下载到本地再传路径（联网素材说到底也要落到本地，下载交给 bash）。适合把本地素材（下载的视频、生成的音频、用户给的文件）让对方直接看/听/用时使用。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"path": {
					Type:        "string",
					Description: "本地文件路径（必填）：绝对或相对路径，图片/音频/视频/文档均可",
				},
				"caption": {
					Type:        "string",
					Description: "可选，展示说明（来源/用途一句话）",
				},
			},
			Required: []string{"path"},
		},
	},
}

// callNativeFetchMedia 处理 fetch_media 工具调用。
func callNativeFetchMedia(ctx context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		Path    string `json:"path"`
		Caption string `json:"caption"`
	}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}
	}
	rawPath := strings.TrimSpace(args.Path)
	if rawPath == "" {
		return nativeToolResult{}, fmt.Errorf("path 必填：本地文件路径")
	}
	if strings.HasPrefix(rawPath, "http://") || strings.HasPrefix(rawPath, "https://") {
		return nativeToolResult{}, fmt.Errorf("fetch_media 只收本地文件路径：远程素材请先用 bash/curl 下载到本地再传路径（联网素材说到底也要落到本地，下载交给 bash，别起 HTTP 服务绕圈）")
	}
	caption := strings.TrimSpace(args.Caption)
	if caption == "" {
		caption = "本地素材"
	}

	localPath, err := nativeAbsPath(rawPath)
	if err != nil {
		return nativeToolResult{}, err
	}
	fi, err := os.Stat(localPath)
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("本地文件不存在: %s（%w）", localPath, err)
	}
	if fi.IsDir() {
		return nativeToolResult{}, fmt.Errorf("path 指向目录，需要文件: %s", localPath)
	}
	if fi.Size() > 30<<20 {
		return nativeToolResult{}, fmt.Errorf("文件超过 30MB 上限（%.1fMB）", float64(fi.Size())/(1<<20))
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("读取文件失败: %w", err)
	}
	if len(data) == 0 {
		return nativeToolResult{}, fmt.Errorf("文件为空")
	}

	// 类型判定：内容嗅探优先，扩展名兜底
	ct := http.DetectContentType(data)
	ct = strings.Split(ct, ";")[0]
	ext := extFromURL(localPath, ct)

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
			Text: fmt.Sprintf("已获取图片素材\n本地路径: %s", localPath),
			Images: []mcpImageArtifact{{
				Data:     base64.StdEncoding.EncodeToString(data),
				MimeType: ct,
			}},
		}, nil
	case "video":
		// 视频已有内嵌播放块，不再重复弹交付卡片（卡片走 /api/agent/file 绝对路径必 400）
		return nativeToolResult{
			Text: fmt.Sprintf("已获取视频素材\n本地文件: %s\n预览: %s\n%s", path, url, caption),
			Videos: []mcpVideoArtifact{{
				URL: url, File: path, Mime: ct, Size: fmt.Sprintf("%d", len(data)),
			}},
		}, nil
	case "audio":
		// 音频已有内嵌播放条，不再重复弹交付卡片
		return nativeToolResult{
			Text: fmt.Sprintf("已获取音频素材\n本地文件: %s\n预览: %s\n%s", path, url, caption),
			Audios: []mcpAudioArtifact{{
				URL: url, File: path, Mime: ct, Size: fmt.Sprintf("%d", len(data)),
			}},
		}, nil
	default:
		// 文档类交付卡：媒体目录在工作目录之外，绝对路径过不了 /api/agent/file
		// 审批注册表（必 400），改带 /api/media 静态直链供前端预览/下载。
		return nativeToolResult{
			Text: fmt.Sprintf("已获取文件素材\n本地路径: %s\n%s", path, caption),
			Files: []fileDeliverable{{
				Path: path, Name: name, Ext: ext, Size: int64(len(data)), URL: url,
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

// extFromURL 从路径/URL/Content-Type 推断扩展名。
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
