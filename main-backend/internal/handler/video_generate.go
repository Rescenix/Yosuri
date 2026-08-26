package handler

// video_generate.go —— Agnes AI 免费生视频（2026-08-25 实测打通）
//
// 免费模型池原来一个生视频的都没有。Agnes Video 系列（agnes-video-v2.0 /
// agnes-video-2.5 / agnes-video-2.5-flash）是当前 $0/秒 的全模态免费 API
// （标准价 $0.005/秒，官方文档当前价 $0）。异步任务制：
//   POST /v1/videos 提交任务 → GET /agnesapi?video_id= 轮询 → 完成取
//   metadata.url（实测顶层 url 字段）下载落盘。
//
// 2026-08-25 实测：5s 720p h264+aac 2.8MB 出片成功，无可见水印。
// 注意：RPM 20 限流；输出分辨率按 480p/720p/1080p 档位归一化，
// 以响应 size 字段为准。

import (
	"bytes"
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

const (
	agnesVideoBaseURL   = "https://apihub.agnes-ai.com"
	agnesVideoTimeout   = 6 * time.Minute // 视频推理可能 60-90s+，轮询上限放宽
	agnesVideoPollEvery = 10 * time.Second
	agnesVideoMaxBytes  = 128 << 20 // 单视频上限 128MB
)

// videoGenSpec 一次生视频请求。
type videoGenSpec struct {
	Prompt    string
	Model     string // agnes-video-2.5-flash（默认，$0/秒） / agnes-video-v2.0（1080p 免费）
	Width     int    // 2.0 用：默认 1920（1080p 档）
	Height    int    // 2.0 用：默认 1080
	NumFrames int    // 2.0 用：8n+1 规则，≤441；默认 121 ≈ 5s@24fps
	FrameRate int    // 2.0 用：1-60，默认 24
	Seconds   string // 2.5-flash 用：时长字符串 "4"-"12"，默认 "5"
	Size      string // 2.5-flash 用：固定 "720P"
	Ratio     string // 2.5-flash 用：aspect_ratio，默认 16:9
	Seed      int64
	Negative  string // 仅 2.0 支持
	ImageURL  string // 图生视频（2.0 用 image 字段）
	OutDir    string // 落盘目录，默认 videoOutputDir()
	Name      string // 文件名（不含扩展名），默认按时间戳
	Style     string // 风格模板：anime（动漫）/ real（真人写实）/ anime_live（动漫真人化，默认）
}

// videoGenResult 生视频结果。
type videoGenResult struct {
	Model    string `json:"model"`
	File     string `json:"file"`
	URL      string `json:"videoUrl"` // 前端可播放路径
	Size     string `json:"size"`     // 如 1088x832
	Seconds  string `json:"seconds"`
	Bytes    []byte `json:"-"`
	MimeType string `json:"mime"`
}

// videoOutputDir 默认出视频目录，与图片同根（~/rescene_data/videos）。
func videoOutputDir() string {
	if root := strings.TrimSpace(os.Getenv("RESCENE_VIDEOS_DIR")); root != "" {
		return root
	}
	return filepath.Join(resceneUserDataDir(), "videos")
}

// agnesVideoAPIKey 优先用户设置（user_configs id=agnes），环境变量 AGNES_API_KEY 兜底。
func agnesVideoAPIKey() string {
	if entries, err := loadModelConfigs(""); err == nil {
		for _, e := range entries {
			if e.ID == "agnes" && strings.TrimSpace(e.APIKey) != "" {
				return strings.TrimSpace(e.APIKey)
			}
		}
	}
	return strings.TrimSpace(os.Getenv("AGNES_API_KEY"))
}

// videoGenToolDef video_generate 工具定义（随 nativeOnDemandToolDefs 按需加载）。
var videoGenToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        "video_generate",
		Description: "AI 生视频（Agnes 免费 API，当前 $0/秒）：输入画面描述，异步生成短视频（默认 5s 720P，最长 12s）。默认模型 agnes-video-2.5-flash（免费）；可选 agnes-video-v2.0（支持 1080p 与图生视频）。style 参数一键切风格：anime（动漫）/ real（真人写实）/ anime_live（动漫真人化，默认，锁日系不跑欧美）。适合做动漫分镜、短片素材、短视频片段。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"prompt": {
					Type:        "string",
					Description: "画面描述（必填）：主体+动作+场景+镜头+光线。如 'A young girl with silver hair standing on a rainy night Tokyo street, neon lights reflecting on wet asphalt'",
				},
				"style": {
					Type:        "string",
					Description: "可选，风格：anime_live（默认，动漫角色真人化，锁日系）/ anime（纯动漫）/ real（真人写实）。自动拼风格词与负面词",
				},
				"model": {
					Type:        "string",
					Description: "可选：agnes-video-2.5-flash（默认，$0/秒，720P）/ agnes-video-v2.0（$0/秒，支持 1080p+图生视频）/ agnes-video-2.5（收费）",
				},
				"seconds": {
					Type:        "string",
					Description: "可选，时长（2.5 系列）：'4'-'12'，默认 '5'",
				},
				"size": {
					Type:        "string",
					Description: "可选，分辨率（2.5 系列）：720P（默认，flash 固定）/ 960P / 2K（仅 2.5 收费版）",
				},
				"aspect_ratio": {
					Type:        "string",
					Description: "可选，画幅（2.5 系列）：16:9（默认）/ 9:16 / 1:1 / 4:3 / 3:4 / 21:9",
				},
				"width": {
					Type:        "integer",
					Description: "可选（仅 2.0）：宽度，默认 1920（1080p 档）",
				},
				"height": {
					Type:        "integer",
					Description: "可选（仅 2.0）：高度，默认 1080",
				},
				"num_frames": {
					Type:        "integer",
					Description: "可选（仅 2.0）：总帧数，8n+1 且 ≤441。121≈5s、241≈10s、441≈18s@24fps",
				},
				"frame_rate": {
					Type:        "integer",
					Description: "可选（仅 2.0）：帧率 1-60，默认 24",
				},
				"seed": {
					Type:        "integer",
					Description: "可选随机种子；同 seed+同 prompt 出片稳定，用于角色/场景一致",
				},
				"negative": {
					Type:        "string",
					Description: "可选负面词（仅 2.0）；style 已自动生成默认负面词，一般不用填",
				},
				"image_url": {
					Type:        "string",
					Description: "可选，图生视频：一张公网可访问的图片 URL，画面以此为起点动起来（先图后视频可锁角色脸）",
				},
			},
			Required: []string{"prompt"},
		},
	},
}

// generateVideo 出一段视频并落盘。Agnes 免费 API 失败时返回明确错误。
func generateVideo(ctx context.Context, spec videoGenSpec) (videoGenResult, error) {
	spec.Prompt = strings.TrimSpace(spec.Prompt)
	if spec.Prompt == "" {
		return videoGenResult{}, fmt.Errorf("prompt 不能为空")
	}
	key := agnesVideoAPIKey()
	if key == "" {
		return videoGenResult{}, fmt.Errorf("未配置 Agnes API Key：打开设置 → 模型 → 填「Agnes API Key」（platform.agnes-ai.com 免费获取），或设置环境变量 AGNES_API_KEY")
	}
	// 默认 2.5-flash（$0/秒限时免费，继承 2.5 能力）；2.0 支持 1080p 免费
	if spec.Model == "" {
		spec.Model = "agnes-video-2.5-flash"
	}
	// 风格模板：把用户描述强化成对应风格的 prompt
	applyVideoStyle(&spec)

	// 2.5 系列（2.5 / 2.5-flash）：seconds/mode/size/aspect_ratio 参数格式
	if strings.Contains(spec.Model, "2.5") {
		return generateVideoV25(ctx, key, spec)
	}
	// 2.0：width/height/num_frames/frame_rate/negative_prompt 参数格式
	return generateVideoV20(ctx, key, spec)
}

// applyVideoStyle 按 Style 模板强化 prompt（实测 2026-08-25）：
// 模型默认偏动漫/插画；要真人写实必须写死 photorealistic + negative。
func applyVideoStyle(spec *videoGenSpec) {
	switch strings.ToLower(strings.TrimSpace(spec.Style)) {
	case "anime":
		spec.Prompt = spec.Prompt + ", anime style, cel shading, 2D illustration, vibrant colors, detailed lineart"
		if spec.Negative == "" {
			spec.Negative = "photorealistic, live-action, 3D render, western face, ugly, deformed"
		}
	case "real":
		spec.Prompt = spec.Prompt + ", photorealistic, realistic skin texture, live-action film look, cinematic, shallow depth of field, natural lighting"
		if spec.Negative == "" {
			spec.Negative = "anime, cartoon, illustration, 2D, 3D render, CGI, digital art, painting, unrealistic, deformed, plastic skin"
		}
	case "", "anime_live":
		// 动漫真人化（默认）：二次元角色特征 + 真人质感，锁日系不跑欧美
		spec.Prompt = spec.Prompt + ", live-action adaptation of anime style, photorealistic human skin, real human face, Japanese actress likeness, J-drama aesthetic, young youthful face, cinematic shallow depth of field, soft natural lighting, NOT cartoon NOT 2D NOT illustration NOT western face"
		if spec.Negative == "" {
			spec.Negative = "cartoon, 2D, illustration, anime drawing, cel shading, western face, caucasian, american, european, old woman, elderly, wrinkles, unrealistic, deformed, plastic skin, CGI render"
		}
	}
}

// generateVideoV20 走 agnes-video-v2.0：$0/秒，支持 1080p + 图生视频 + negative。
func generateVideoV20(ctx context.Context, key string, spec videoGenSpec) (videoGenResult, error) {
	if spec.Width <= 0 {
		spec.Width = 1920 // 1080p 档（实测 1920x1080 → 输出 1920x1088）
	}
	if spec.Height <= 0 {
		spec.Height = 1080
	}
	if spec.NumFrames <= 0 {
		spec.NumFrames = 121
	}
	if spec.FrameRate <= 0 {
		spec.FrameRate = 24
	}

	payload := map[string]any{
		"model":      spec.Model,
		"prompt":     spec.Prompt,
		"width":      spec.Width,
		"height":     spec.Height,
		"num_frames": spec.NumFrames,
		"frame_rate": spec.FrameRate,
	}
	if spec.Seed > 0 {
		payload["seed"] = spec.Seed
	}
	if strings.TrimSpace(spec.Negative) != "" {
		payload["negative_prompt"] = spec.Negative
	}
	if strings.TrimSpace(spec.ImageURL) != "" {
		payload["image"] = spec.ImageURL
	}
	return agnesSubmitVideo(ctx, key, payload)
}

// generateVideoV25 走 agnes-video-2.5 / agnes-video-2.5-flash：$0/秒（flash 限时）。
// 参数格式不同：seconds/mode/size/aspect_ratio；无 negative_prompt（2.5 禁该字段）。
func generateVideoV25(ctx context.Context, key string, spec videoGenSpec) (videoGenResult, error) {
	payload := map[string]any{
		"model":  spec.Model,
		"prompt": spec.Prompt,
		"mode":   "text",
	}
	seconds := strings.TrimSpace(spec.Seconds)
	if seconds == "" {
		seconds = "5"
	}
	payload["seconds"] = seconds
	payload["size"] = "720P" // flash 固定 720P；2.5 支持 720P/960P/2K
	if strings.TrimSpace(spec.Size) != "" {
		payload["size"] = strings.TrimSpace(spec.Size)
	}
	if strings.TrimSpace(spec.Ratio) != "" {
		payload["aspect_ratio"] = strings.TrimSpace(spec.Ratio)
	} else {
		payload["aspect_ratio"] = "16:9"
	}
	if spec.Seed > 0 {
		payload["seed"] = spec.Seed
	}
	// 图生视频：2.5 系列走 reference 模式
	if strings.TrimSpace(spec.ImageURL) != "" {
		payload["mode"] = "reference"
		payload["images"] = []string{spec.ImageURL}
	}
	return agnesSubmitVideo(ctx, key, payload)
}

// agnesSubmitVideo 公共提交+轮询+下载。
func agnesSubmitVideo(ctx context.Context, key string, payload map[string]any) (videoGenResult, error) {
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", agnesVideoBaseURL+"/v1/videos", bytes.NewReader(body))
	if err != nil {
		return videoGenResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return videoGenResult{}, fmt.Errorf("提交视频任务失败: %w", err)
	}
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return videoGenResult{}, fmt.Errorf("Agnes 提交任务 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var task struct {
		VideoID string `json:"video_id"`
		Status  string `json:"status"`
		Model   string `json:"model"`
	}
	if err := json.Unmarshal(respBody, &task); err != nil || task.VideoID == "" {
		return videoGenResult{}, fmt.Errorf("Agnes 响应解析失败: %s", strings.TrimSpace(string(respBody)))
	}
	model := task.Model
	if model == "" {
		model, _ = payload["model"].(string)
	}

	// 轮询结果（video_id + model_name 双保险；text 模式 model_name 可省）
	queryURL := agnesVideoBaseURL + "/agnesapi?video_id=" + urlQueryEscape(task.VideoID)
	if model != "" {
		queryURL += "&model_name=" + urlQueryEscape(model)
	}
	deadline := time.Now().Add(agnesVideoTimeout)
	var (
		videoURL   string
		status     string
		sizeStr    string
		secondsStr string
	)
	for {
		if time.Now().After(deadline) {
			return videoGenResult{}, fmt.Errorf("视频生成超时（>%v），video_id=%s，可稍后用同一 ID 查询", agnesVideoTimeout, task.VideoID)
		}
		select {
		case <-ctx.Done():
			return videoGenResult{}, ctx.Err()
		case <-time.After(agnesVideoPollEvery):
		}

		req, _ := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
		req.Header.Set("Authorization", "Bearer "+key)
		r2, err := client.Do(req)
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(r2.Body, 2<<20))
		r2.Body.Close()
		var st struct {
			Status  string `json:"status"`
			URL     string `json:"url"`
			Size    string `json:"size"`
			Seconds string `json:"seconds"`
			Error   string `json:"error"`
		}
		if err := json.Unmarshal(data, &st); err != nil {
			continue
		}
		status = st.Status
		sizeStr, secondsStr, videoURL = st.Size, st.Seconds, st.URL
		switch status {
		case "completed":
			if videoURL == "" {
				// 兜底：metadata.url（2.5 文档格式）
				var meta struct {
					Metadata struct {
						URL string `json:"url"`
					} `json:"metadata"`
				}
				_ = json.Unmarshal(data, &meta)
				videoURL = meta.Metadata.URL
			}
			if videoURL == "" {
				return videoGenResult{}, fmt.Errorf("任务完成但响应无 url")
			}
			return downloadVideo(ctx, videoURL, videoGenSpec{
				Model: model, OutDir: "", Name: "",
			}, videoGenResult{
				Model: model, Size: sizeStr, Seconds: secondsStr,
			})
		case "failed":
			msg := st.Error
			if msg == "" {
				msg = strings.TrimSpace(string(data))
				if len(msg) > 300 {
					msg = msg[:300]
				}
			}
			return videoGenResult{}, fmt.Errorf("视频生成失败: %s", msg)
		}
	}
}

// downloadVideo 从 Agnes 输出 URL 下载视频落盘。
func downloadVideo(ctx context.Context, videoURL string, spec videoGenSpec, res videoGenResult) (videoGenResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", videoURL, nil)
	if err != nil {
		return res, err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return res, fmt.Errorf("下载视频失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return res, fmt.Errorf("下载视频 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, agnesVideoMaxBytes))
	if err != nil {
		return res, fmt.Errorf("读取视频失败: %w", err)
	}
	if len(data) == 0 {
		return res, fmt.Errorf("下载到空视频")
	}
	// 完整性校验：mp4 必须含 moov（实测 curl 下载不完整时 moov 缺失）
	if !bytes.Contains(data[:minInt(len(data), 512*1024)], []byte("moov")) && !bytes.Contains(data, []byte("moov")) {
		return res, fmt.Errorf("视频文件不完整（缺 moov），请重试")
	}

	dir := strings.TrimSpace(spec.OutDir)
	if dir == "" {
		dir = videoOutputDir()
	}
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return res, fmt.Errorf("创建目录失败: %w", mkErr)
	}
	name := sanitizeImageName(spec.Name)
	if name == "" {
		name = fmt.Sprintf("video-%d", time.Now().UnixMilli())
	}
	// 统一存 .mp4：Agnes 返回的 Content-Type 常是 video/x-m4v / video/mp4，
	// 若按 Content-Type 推断会存成 .m4v——Wails WebView2 / 浏览器 <video> 对 .m4v
	// 播放支持差（2026-08-26 实测：对话内嵌播放器弹不出来）。m4v 与 mp4 同为
	// MPEG-4 容器，改扩展名即可正常播放；Content-Type 也归一为 video/mp4。
	ext := ".mp4"
	file := filepath.Join(dir, name+ext)
	if wErr := os.WriteFile(file, data, 0o644); wErr != nil {
		return res, fmt.Errorf("保存视频失败: %w", wErr)
	}
	res.File = file
	res.MimeType = "video/mp4"
	res.Bytes = data
	if rel, relErr := filepath.Rel(videoOutputDir(), file); relErr == nil && !strings.HasPrefix(rel, "..") {
		res.URL = "/api/video/file/" + filepath.ToSlash(rel)
	}
	return res, nil
}

// callNativeVideoGenerate 内置工具 video_generate 实现。
func callNativeVideoGenerate(ctx context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		Prompt    string `json:"prompt"`
		Model     string `json:"model"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		NumFrames int    `json:"num_frames"`
		FrameRate int    `json:"frame_rate"`
		Seed      int64  `json:"seed"`
		Negative  string `json:"negative"`
		ImageURL  string `json:"image_url"`
	}
	if strings.TrimSpace(argsJSON) != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}
	}
	res, err := generateVideo(ctx, videoGenSpec{
		Prompt:    args.Prompt,
		Model:     args.Model,
		Width:     args.Width,
		Height:    args.Height,
		NumFrames: args.NumFrames,
		FrameRate: args.FrameRate,
		Seed:      args.Seed,
		Negative:  args.Negative,
		ImageURL:  args.ImageURL,
	})
	if err != nil {
		return nativeToolResult{}, err
	}
	text := fmt.Sprintf("已生成视频（%s，%s，%s秒）\n本地路径: %s\n预览地址: %s",
		res.Model, res.Size, res.Seconds, res.File, res.URL)
	// 视频体积大，不走 base64 内联（聊天框只放路径），前端预览页可用 URL 播放。
	// Videos 工件让工作流把视频内嵌成可拖动进度条的播放块（同图片内嵌块模式）。
	return nativeToolResult{
		Text: text,
		Videos: []mcpVideoArtifact{{
			URL: res.URL, File: res.File,
			Mime: "video/mp4", Size: res.Size, Seconds: res.Seconds,
		}},
	}, nil
}

// urlQueryEscape 简化：video_id 是 base64url 安全字符集，直接透传即可。
func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "+", "%2B"), "=", "%3D")
}
