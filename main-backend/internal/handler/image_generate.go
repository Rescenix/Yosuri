package handler

// image_generate.go —— 免费、无 key 的原生生图。
//
// 之前生图只有两条路：本地 SD WebUI（要装几个 G 的模型、启动几分钟），
// 或者外部 MCP server（要用户自己配 mcp.json）。设置面板里那个
// 「生图提供商 = Pollinations（免费，无 key）」的选项，落到 Go 侧只是给
// MCP 工具注参数——没配 MCP 就等于没有生图。
//
// 这里把 Pollinations 直接做进 Go：不需要 key、不需要模型、不需要 Python，
// HTTP GET 一个 URL 就是一张图。SD 在线时作为兜底。

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	imageGenMaxBytes = 16 << 20 // 单张图上限 16MB，防坏响应把内存吃穿
	imageGenTimeout  = 3 * time.Minute
)

var imageGenHTTPClient = &http.Client{Timeout: imageGenTimeout}

// imageGenSpec 一次生图请求。Provider 留空时用用户在设置面板选的默认值。
type imageGenSpec struct {
	Prompt   string
	Negative string
	Width    int
	Height   int
	Seed     int64
	Model    string // pollinations: flux / turbo
	Provider string // pollinations / sd
	OutDir   string // 落盘目录，默认 imageOutputDir()
	Name     string // 文件名（不含扩展名），默认按时间戳生成
}

// imageGenResult 出图结果。URL 是前端可直接 <img src> 的路径。
type imageGenResult struct {
	Provider string `json:"provider"`
	File     string `json:"file"`
	URL      string `json:"imageUrl"`
	Mime     string `json:"mime"`
	Seed     int64  `json:"seed"`
	Bytes    []byte `json:"-"`
}

// imageOutputDir 默认出图目录，和记忆/技能一样落在 ~/rescene_data 下。
func imageOutputDir() string {
	if root := strings.TrimSpace(os.Getenv("RESCENE_IMAGES_DIR")); root != "" {
		return root
	}
	return filepath.Join(resceneUserDataDir(), "images")
}

// generateImage 出一张图并落盘。Pollinations 失败且本地 SD 在线时自动兜底。
func generateImage(ctx context.Context, spec imageGenSpec) (imageGenResult, error) {
	spec.Prompt = strings.TrimSpace(spec.Prompt)
	if spec.Prompt == "" {
		return imageGenResult{}, fmt.Errorf("prompt 不能为空")
	}
	spec.Width = clampImageSide(spec.Width)
	spec.Height = clampImageSide(spec.Height)
	if spec.Seed <= 0 {
		spec.Seed = rand.Int63n(1 << 31)
	}
	provider := strings.ToLower(strings.TrimSpace(spec.Provider))
	if provider == "" {
		provider = currentImageProvider
	}

	var (
		data []byte
		mime string
		err  error
	)
	switch provider {
	case "sd":
		data, mime, err = generateImageSD(ctx, spec)
	case "custom":
		data, mime, err = generateImageCustom(ctx, spec)
	case "mcp":
		data, mime, err = generateImageMCP(ctx, spec)
	default:
		provider = "pollinations"
		data, mime, err = generateImagePollinations(ctx, spec)
		if err != nil {
			// 免费公共服务偶尔抽风；本地 SD 已经起着就顺手兜一下，
			// 不在线就照实报错，不去替用户启动那几分钟的模型加载。
			if online, _ := comicSDOnline(); online {
				if sdData, sdMime, sdErr := generateImageSD(ctx, spec); sdErr == nil {
					data, mime, err, provider = sdData, sdMime, nil, "sd"
				}
			}
		}
	}
	if err != nil {
		return imageGenResult{}, err
	}

	dir := strings.TrimSpace(spec.OutDir)
	if dir == "" {
		dir = imageOutputDir()
	}
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return imageGenResult{}, fmt.Errorf("创建出图目录失败: %w", mkErr)
	}
	name := sanitizeImageName(spec.Name)
	if name == "" {
		name = fmt.Sprintf("img-%d-%d", time.Now().UnixMilli(), spec.Seed)
	}
	ext := ".png"
	if strings.Contains(mime, "jpeg") || strings.Contains(mime, "jpg") {
		ext = ".jpg"
	}
	file := filepath.Join(dir, name+ext)
	if wErr := os.WriteFile(file, data, 0o644); wErr != nil {
		return imageGenResult{}, fmt.Errorf("保存图片失败: %w", wErr)
	}

	result := imageGenResult{Provider: provider, File: file, Mime: mime, Seed: spec.Seed, Bytes: data}
	if rel, relErr := filepath.Rel(imageOutputDir(), file); relErr == nil && !strings.HasPrefix(rel, "..") {
		result.URL = "/api/image/file/" + filepath.ToSlash(rel)
	}
	return result, nil
}

// generateImagePollinations 免费无 key：GET 一个 URL 就是一张图。
func generateImagePollinations(ctx context.Context, spec imageGenSpec) ([]byte, string, error) {
	model := strings.TrimSpace(spec.Model)
	if model == "" {
		model = "flux"
	}
	prompt := spec.Prompt
	if neg := strings.TrimSpace(spec.Negative); neg != "" {
		// Pollinations 没有独立的负面词参数，按官方推荐并进正面提示词。
		prompt += ". Avoid: " + neg
	}
	endpoint := "https://image.pollinations.ai/prompt/" + url.PathEscape(prompt)
	query := url.Values{}
	query.Set("width", fmt.Sprint(spec.Width))
	query.Set("height", fmt.Sprint(spec.Height))
	query.Set("seed", fmt.Sprint(spec.Seed))
	query.Set("model", model)
	query.Set("nologo", "true")
	query.Set("referrer", "rescene")
	endpoint += "?" + query.Encode()

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, "", ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, "", err
		}
		req.Header.Set("User-Agent", "Rescene/1.0")
		resp, err := imageGenHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, imageGenMaxBytes))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("Pollinations HTTP %d: %s", resp.StatusCode, truncateChars(strings.TrimSpace(string(body)), 180))
			continue
		}
		mime := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(mime, "image/") {
			lastErr = fmt.Errorf("Pollinations 返回的不是图片（%s）", mime)
			continue
		}
		if len(body) == 0 {
			lastErr = fmt.Errorf("Pollinations 返回空图片")
			continue
		}
		return body, mime, nil
	}
	return nil, "", fmt.Errorf("免费生图失败: %v", lastErr)
}

// generateImageSD 兜底路径：本机 SD WebUI 的 txt2img。
func generateImageSD(ctx context.Context, spec imageGenSpec) ([]byte, string, error) {
	negative := strings.TrimSpace(spec.Negative)
	if negative == "" {
		negative = "lowres, bad anatomy, bad hands, text, watermark, blurry, nsfw, extra fingers"
	}
	payload, _ := json.Marshal(map[string]any{
		"prompt":          spec.Prompt,
		"negative_prompt": negative,
		"steps":           20,
		"width":           spec.Width,
		"height":          spec.Height,
		"batch_size":      1,
		"cfg_scale":       7,
		"seed":            spec.Seed,
		"sampler_name":    "Euler a",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, comicSDBaseURL()+"/sdapi/v1/txt2img", bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := imageGenHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("SD WebUI 连接失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, imageGenMaxBytes))
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("SD WebUI HTTP %d: %s", resp.StatusCode, truncateChars(strings.TrimSpace(string(body)), 180))
	}
	var out struct {
		Images []string `json:"images"`
	}
	if err := json.Unmarshal(body, &out); err != nil || len(out.Images) == 0 {
		return nil, "", fmt.Errorf("SD 出图失败")
	}
	raw := out.Images[0]
	if idx := strings.Index(raw, ","); strings.HasPrefix(raw, "data:") && idx > 0 {
		raw = raw[idx+1:]
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, "", fmt.Errorf("SD 图片数据无效: %w", err)
	}
	return data, "image/png", nil
}

// generateImageCustom 走用户自定义的 OpenAI 兼容生图端点（/v1/images/generations）。
// 配置在设置面板 → 模型 → 生图提供商 → 自定义模型（Endpoint/API Key/模型名）。
func generateImageCustom(ctx context.Context, spec imageGenSpec) ([]byte, string, error) {
	entry, ok := capabilityEntry(imageCapabilityID)
	if !ok || strings.TrimSpace(entry.Endpoint) == "" {
		return nil, "", fmt.Errorf("未配置自定义生图：打开设置 → 模型 → 生图提供商 → 自定义模型，填写 Endpoint")
	}
	model := strings.TrimSpace(entry.DefaultModel)
	if model == "" {
		return nil, "", fmt.Errorf("未配置自定义生图模型名：打开设置 → 模型 → 生图提供商 → 自定义模型，填写模型名")
	}
	key := strings.TrimSpace(entry.APIKey)
	endpoint := strings.TrimRight(strings.TrimSpace(entry.Endpoint), "/")
	if !strings.HasSuffix(endpoint, "/v1") {
		endpoint += "/v1"
	}
	payload, _ := json.Marshal(map[string]any{
		"model":  model,
		"prompt": spec.Prompt,
		"n":      1,
		"size":   fmt.Sprintf("%dx%d", spec.Width, spec.Height),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/images/generations", bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := imageGenHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("自定义生图连接失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, imageGenMaxBytes))
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("自定义生图 HTTP %d: %s", resp.StatusCode, truncateChars(strings.TrimSpace(string(body)), 300))
	}
	var out struct {
		Data []struct {
			URL     string `json:"url"`
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil || len(out.Data) == 0 {
		return nil, "", fmt.Errorf("自定义生图响应无效（无 data 字段）")
	}
	item := out.Data[0]
	if item.B64JSON != "" {
		data, derr := base64.StdEncoding.DecodeString(item.B64JSON)
		if derr != nil {
			return nil, "", fmt.Errorf("自定义生图 base64 无效: %w", derr)
		}
		return data, "image/png", nil
	}
	u := strings.TrimSpace(item.URL)
	if u == "" {
		return nil, "", fmt.Errorf("自定义生图响应里没有图片 URL")
	}
	return downloadImageBytes(ctx, u)
}

// generateImageMCP 把生图委托给用户指定的已装 MCP 工具（mcp__server__tool）。
// 配置在设置面板 → 模型 → 生图提供商 → MCP 工具。
func generateImageMCP(ctx context.Context, spec imageGenSpec) ([]byte, string, error) {
	entry, ok := capabilityEntry(imageCapabilityID)
	if !ok || strings.TrimSpace(entry.Extra["mcp_tool"]) == "" {
		return nil, "", fmt.Errorf("未选择 MCP 生图工具：打开设置 → 模型 → 生图提供商 → MCP 工具")
	}
	tool := strings.TrimSpace(entry.Extra["mcp_tool"])
	args, _ := json.Marshal(map[string]any{
		"prompt": spec.Prompt, "negative": spec.Negative,
		"width": spec.Width, "height": spec.Height, "seed": spec.Seed,
	})
	result, err := callMCPToolWithArtifacts(tool, string(args))
	if err != nil {
		return nil, "", fmt.Errorf("MCP 生图失败: %w", err)
	}
	if len(result.Images) == 0 {
		return nil, "", fmt.Errorf("MCP 生图工具没有返回图片：%s", truncateChars(result.Text, 200))
	}
	img := result.Images[0]
	data, derr := base64.StdEncoding.DecodeString(img.Data)
	if derr != nil {
		return nil, "", fmt.Errorf("MCP 图片数据无效: %w", derr)
	}
	mime := strings.TrimSpace(img.MimeType)
	if mime == "" {
		mime = "image/png"
	}
	return data, mime, nil
}

// downloadImageBytes 抓取生图端返回的图片 URL（http(s) 或 data: 两种形态）。
func downloadImageBytes(ctx context.Context, u string) ([]byte, string, error) {
	if strings.HasPrefix(u, "data:") {
		// data:image/png;base64,xxxx
		idx := strings.Index(u, ",")
		mime := "image/png"
		if strings.HasPrefix(u, "data:") && idx > 5 {
			meta := u[5:idx]
			if semi := strings.Index(meta, ";"); semi > 0 {
				mime = meta[:semi]
			}
		}
		if idx < 0 {
			return nil, "", fmt.Errorf("data URL 无效")
		}
		data, err := base64.StdEncoding.DecodeString(u[idx+1:])
		if err != nil {
			return nil, "", fmt.Errorf("data URL base64 无效: %w", err)
		}
		return data, mime, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Rescene/1.0")
	resp, err := imageGenHTTPClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("下载生图结果失败: %w", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, imageGenMaxBytes))
	if readErr != nil {
		return nil, "", readErr
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("下载生图结果 HTTP %d", resp.StatusCode)
	}
	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/png"
	}
	return body, mime, nil
}

func clampImageSide(v int) int {
	if v <= 0 {
		return 1024
	}
	if v < 256 {
		return 256
	}
	if v > 1536 {
		return 1536
	}
	return v
}

// sanitizeImageName 只保留文件名里安全的字符，杜绝 ../ 之类越权落盘。
func sanitizeImageName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = filepath.Base(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// HandleImageGenerate POST /api/image/generate —— 免费无 key 生图的 HTTP 面。
func HandleImageGenerate(c *gin.Context) {
	var req struct {
		Prompt   string `json:"prompt" binding:"required"`
		Negative string `json:"negative"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Seed     int64  `json:"seed"`
		Model    string `json:"model"`
		Provider string `json:"provider"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	result, err := generateImage(c.Request.Context(), imageGenSpec{
		Prompt:   req.Prompt,
		Negative: req.Negative,
		Width:    req.Width,
		Height:   req.Height,
		Seed:     req.Seed,
		Model:    req.Model,
		Provider: req.Provider,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"provider": result.Provider,
		"imageUrl": result.URL,
		"file":     result.File,
		"seed":     result.Seed,
	})
}

// callNativeImageGenerate 内置工具 image_generate：模型直接会画画，不依赖 MCP。
func callNativeImageGenerate(ctx context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		Prompt   string `json:"prompt"`
		Negative string `json:"negative"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Seed     int64  `json:"seed"`
	}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}
	}
	result, err := generateImage(ctx, imageGenSpec{
		Prompt:   args.Prompt,
		Negative: args.Negative,
		Width:    args.Width,
		Height:   args.Height,
		Seed:     args.Seed,
	})
	if err != nil {
		return nativeToolResult{}, err
	}
	text := fmt.Sprintf("已生成图片（%s，seed=%d）\n本地路径: %s\n预览地址: %s", result.Provider, result.Seed, result.File, result.URL)
	return nativeToolResult{
		Text: text,
		Images: []mcpImageArtifact{{
			Data:     base64.StdEncoding.EncodeToString(result.Bytes),
			MimeType: result.Mime,
		}},
	}, nil
}
