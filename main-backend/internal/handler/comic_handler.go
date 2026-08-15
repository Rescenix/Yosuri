package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// comicPanel 单格漫画面板
type comicPanel struct {
	Index     int    `json:"index"`
	Scene     string `json:"scene"`
	Dialogue  string `json:"dialogue"`
	Character string `json:"character"`
	Action    string `json:"action"`
	Camera    string `json:"camera"`
	PromptEN  string `json:"promptEn"`
	ImageURL  string `json:"imageUrl,omitempty"`
}

// comicPageRequest 漫画生成请求
type comicPageRequest struct {
	Title     string `json:"title" binding:"required"`
	Chapter   string `json:"chapter" binding:"required"`
	Style     string `json:"style"`
	Character string `json:"character"`
	Panels    int    `json:"panels"` // 分几格，默认4
}

// comicPageResponse 漫画页响应
type comicPageResponse struct {
	Title  string       `json:"title"`
	Panels []comicPanel `json:"panels"`
	PageID string       `json:"pageId"`
}

var comicHTTPClient = &http.Client{Timeout: 3 * time.Minute}
var comicSDState struct {
	sync.Mutex
	starting bool
	lastErr  string
	baseURL  string
}

func comicSDBaseURL() string {
	if value := strings.TrimRight(strings.TrimSpace(os.Getenv("RESCENE_SD_URL")), "/"); value != "" {
		return value
	}
	comicSDState.Lock()
	discovered := comicSDState.baseURL
	comicSDState.Unlock()
	if discovered != "" {
		return discovered
	}
	return "http://127.0.0.1:7860"
}

func comicSDOnline() (bool, string) {
	client := &http.Client{Timeout: 3 * time.Second}
	baseURLs := []string{comicSDBaseURL()}
	if strings.TrimSpace(os.Getenv("RESCENE_SD_URL")) == "" {
		baseURLs = append(baseURLs, "http://127.0.0.1:7860", "http://127.0.0.1:7861")
	}
	seen := map[string]bool{}
	for _, baseURL := range baseURLs {
		if seen[baseURL] {
			continue
		}
		seen[baseURL] = true
		resp, err := client.Get(baseURL + "/sdapi/v1/options")
		if err != nil {
			continue
		}
		var options map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&options)
		_ = resp.Body.Close()
		if resp.StatusCode < 400 {
			model, _ := options["sd_model_checkpoint"].(string)
			comicSDState.Lock()
			comicSDState.baseURL = baseURL
			comicSDState.Unlock()
			return true, model
		}
	}
	return false, ""
}

func findComicSDLauncher() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("RESCENE_SD_LAUNCHER")); configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("配置的绘图引擎启动文件不存在: %s", configured)
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		`C:\Pro2026\SDWebUI\webui-user.bat`,
		filepath.Join(home, "stable-diffusion-webui", "webui-user.bat"),
		filepath.Join(home, "SDWebUI", "webui-user.bat"),
	}
	if runtime.GOOS != "windows" {
		candidates = []string{
			filepath.Join(home, "stable-diffusion-webui", "webui-user.sh"),
			filepath.Join(home, "SDWebUI", "webui-user.sh"),
		}
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未找到本地绘图引擎，请设置 RESCENE_SD_LAUNCHER")
}

func runnableComicPython(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return hiddenCommandContext(ctx, path, "--version").Run() == nil
}

func findComicPython(sdDir string) (string, error) {
	if configured := strings.Trim(strings.TrimSpace(os.Getenv("RESCENE_SD_PYTHON")), `"`); configured != "" {
		if runnableComicPython(configured) {
			return configured, nil
		}
		return "", fmt.Errorf("配置的 Python 无法运行: %s", configured)
	}
	candidates := []string{filepath.Join(sdDir, "venv", "Scripts", "python.exe")}
	if runtime.GOOS != "windows" {
		candidates = []string{filepath.Join(sdDir, "venv", "bin", "python3")}
	}
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		candidates = append(candidates,
			filepath.Join(localAppData, "Programs", "Python", "Python310", "python.exe"),
			filepath.Join(localAppData, "Programs", "Python", "Python311", "python.exe"),
		)
	}
	if appData := os.Getenv("APPDATA"); appData != "" {
		matches, _ := filepath.Glob(filepath.Join(appData, "uv", "python", "cpython-3.1[01]-windows-*", "python.exe"))
		candidates = append(candidates, matches...)
	}
	if fromPath, err := exec.LookPath("python"); err == nil {
		candidates = append(candidates, fromPath)
	}
	for _, candidate := range candidates {
		if runnableComicPython(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("本地 SD 缺少可用的 Python 3.10/3.11 环境，无法自动启动")
}

// HandleComicStatus GET /api/comic/status —— 探测本地 SD WebUI API。
func HandleComicStatus(c *gin.Context) {
	online, model := comicSDOnline()
	comicSDState.Lock()
	starting, lastErr := comicSDState.starting, comicSDState.lastErr
	comicSDState.Unlock()
	c.JSON(http.StatusOK, gin.H{"online": online, "model": model, "starting": starting, "error": lastErr})
}

// HandleComicStartSD POST /api/comic/start-sd —— 后台启动本机 SD WebUI。
func HandleComicStartSD(c *gin.Context) {
	if online, model := comicSDOnline(); online {
		c.JSON(http.StatusOK, gin.H{"online": true, "model": model, "starting": false})
		return
	}
	comicSDState.Lock()
	if comicSDState.starting {
		comicSDState.Unlock()
		c.JSON(http.StatusAccepted, gin.H{"online": false, "starting": true})
		return
	}
	launcher, err := findComicSDLauncher()
	if err != nil {
		comicSDState.lastErr = err.Error()
		comicSDState.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"online": false, "starting": false, "error": err.Error()})
		return
	}
	logPath := filepath.Join(comicOutputDir(), "sd-webui.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		// 日志不可写不能阻断绘图；桌面沙箱或只读数据目录下静默丢弃子进程输出。
		logFile, err = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			comicSDState.Unlock()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "无法启动绘图引擎输出通道"})
			return
		}
	}
	var cmdArgs []string
	command := launcher
	pythonPath, err := findComicPython(filepath.Dir(launcher))
	if err != nil {
		_ = logFile.Close()
		comicSDState.lastErr = err.Error()
		comicSDState.Unlock()
		c.JSON(http.StatusServiceUnavailable, gin.H{"online": false, "starting": false, "error": err.Error()})
		return
	}
	if runtime.GOOS == "windows" {
		// 部分 webui-user.bat 不转发参数；直接调用同目录 webui.bat 才能确保 --api 生效。
		if strings.EqualFold(filepath.Base(launcher), "webui-user.bat") {
			webuiBat := filepath.Join(filepath.Dir(launcher), "webui.bat")
			if info, statErr := os.Stat(webuiBat); statErr == nil && !info.IsDir() {
				launcher = webuiBat
			}
		}
		command = "cmd.exe"
		cmdArgs = []string{"/d", "/c", launcher, "--api", "--nowebui"}
	} else {
		command = "bash"
		cmdArgs = []string{launcher, "--api", "--nowebui"}
	}
	cmd := hiddenCommand(command, cmdArgs...)
	cmd.Dir = filepath.Dir(launcher)
	cmd.Env = append(os.Environ(), `PYTHON="`+pythonPath+`"`)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		comicSDState.lastErr = "绘图引擎启动失败: " + err.Error()
		comicSDState.Unlock()
		c.JSON(http.StatusInternalServerError, gin.H{"online": false, "starting": false, "error": comicSDState.lastErr})
		return
	}
	comicSDState.starting = true
	comicSDState.lastErr = ""
	comicSDState.Unlock()
	go func() {
		err := cmd.Wait()
		_ = logFile.Close()
		comicSDState.Lock()
		comicSDState.starting = false
		if err != nil {
			comicSDState.lastErr = "绘图引擎已退出: " + err.Error()
		}
		comicSDState.Unlock()
	}()
	c.JSON(http.StatusAccepted, gin.H{"online": false, "starting": true, "message": "绘图引擎正在后台启动"})
}

// HandleComicBreakdown POST /api/comic/breakdown — LLM 把小说章节拆成漫画分镜
func HandleComicBreakdown(c *gin.Context) {
	var req comicPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if req.Panels < 1 || req.Panels > 9 {
		req.Panels = 4
	}

	prompt := fmt.Sprintf(`你是漫画分镜师。把以下小说章节拆成 %d 格漫画分镜。
对每一格，输出 JSON 格式：
{
  "index": 格子序号,
  "scene": "场景描述（中文，30字内）",
  "dialogue": "这一格的角色对话（无对话则空）",
  "character": "出现的角色名",
  "action": "角色动作",
  "camera": "镜头角度（远景/中景/特写/俯视）",
  "promptEn": "英文画面描述，用于 AI 生图，包含角色外貌、动作、场景、光线、风格"
}

注意事项：
- 第一格必须有场景建立镜头
- 最后一格必须是悬念/钩子
- 对话要精简，符合角色性格
- promptEn 用英文，描述画面不要有文字

小说标题：%s
%s
文风与要求：%s
角色设定：%s

输出 JSON 数组，不要其他文字。`, req.Panels, req.Title, req.Chapter, req.Style, req.Character)

	result, err := callLocalAggregate(prompt)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "分镜生成失败: " + err.Error()})
		return
	}

	// 解析 JSON
	result = cleanJSON(result)
	var panels []comicPanel
	if err := json.Unmarshal([]byte(result), &panels); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分镜解析失败: " + err.Error(), "raw": result})
		return
	}

	pageID := fmt.Sprintf("comic-%d", time.Now().UnixMilli())
	resp := comicPageResponse{
		Title:  req.Title,
		Panels: panels,
		PageID: pageID,
	}

	c.JSON(http.StatusOK, resp)
}

// HandleComicGenerate POST /api/comic/generate — 生成漫画页（分镜+出图+拼页）
func HandleComicGenerate(c *gin.Context) {
	var req comicPageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if req.Panels < 1 || req.Panels > 9 {
		req.Panels = 4
	}

	// Step 1: LLM 分镜
	prompt := fmt.Sprintf(`你是漫画分镜师。把以下小说章节拆成 %d 格漫画分镜。
对每一格，输出 JSON 格式：
{
  "index": 格子序号,
  "scene": "场景描述（中文，30字内）",
  "dialogue": "这一格的角色对话（无对话则空）",
  "character": "出现的角色名",
  "action": "角色动作",
  "camera": "镜头角度（远景/中景/特写/俯视）",
  "promptEn": "英文画面描述，用于 AI 生图，包含角色外貌、动作、场景、光线、风格，纯英文无中文"
}

注意事项：
- 第一格场景建立，最后一格悬念
- promptEn 纯英文，画面描述，不要文字/标题/对话框
- 对话精简，符合角色性格

小说标题：%s
%s
文风：%s
角色：%s

输出 JSON 数组，不要其他文字。`, req.Panels, req.Title, req.Chapter, req.Style, req.Character)

	result, err := callLocalAggregate(prompt)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "分镜生成失败: " + err.Error()})
		return
	}

	result = cleanJSON(result)
	var panels []comicPanel
	if err := json.Unmarshal([]byte(result), &panels); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分镜解析失败: " + err.Error()})
		return
	}

	pageID := fmt.Sprintf("comic-%d", time.Now().UnixMilli())
	resp := comicPageResponse{
		Title:  req.Title,
		Panels: panels,
		PageID: pageID,
	}
	comicDir := filepath.Join(comicOutputDir(), pageID)
	if err := os.MkdirAll(comicDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建漫画项目失败: " + err.Error()})
		return
	}
	pageData, _ := json.MarshalIndent(resp, "", "  ")
	_ = os.WriteFile(filepath.Join(comicDir, "page.json"), pageData, 0o600)

	// Step 2: 返回分镜结果，前端确认后生成图片
	c.JSON(http.StatusOK, resp)
}

// HandleComicRenderPanel POST /api/comic/render-panel — 调用 SD WebUI 生成单格图片
func HandleComicRenderPanel(c *gin.Context) {
	var req struct {
		PageID  string     `json:"pageId" binding:"required"`
		Panel   comicPanel `json:"panel"`
		GenType string     `json:"genType"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	pageID := filepath.Base(strings.TrimSpace(req.PageID))
	if pageID == "" || pageID == "." {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pageId 无效"})
		return
	}
	panelDir := filepath.Join(comicOutputDir(), pageID)
	os.MkdirAll(panelDir, 0o755)

	// 构建 SD 提示词
	sdPrompt := req.Panel.PromptEN
	if sdPrompt == "" {
		sdPrompt = fmt.Sprintf("1girl, long black hair, anime style, %s, %s, masterpiece, best quality", req.Panel.Scene, req.Panel.Action)
	}
	sdPrompt += ", anime style, masterpiece, best quality, detailed background"

	// 调用 SD WebUI
	payload := map[string]interface{}{
		"prompt":          sdPrompt,
		"negative_prompt": "lowres, bad anatomy, bad hands, text, watermark, blurry, nsfw, extra fingers",
		"steps":           20,
		"width":           512,
		"height":          768,
		"batch_size":      1,
		"cfg_scale":       7,
		"sampler_name":    "Euler a",
	}
	payloadBytes, _ := json.Marshal(payload)
	resp, err := comicHTTPClient.Post(comicSDBaseURL()+"/sdapi/v1/txt2img", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "SD WebUI 连接失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("SD WebUI 返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))})
		return
	}

	var sdResp struct {
		Images []string `json:"images"`
	}
	if err := json.Unmarshal(body, &sdResp); err != nil || len(sdResp.Images) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "SD 出图失败"})
		return
	}

	// 保存图片
	imgData, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(sdResp.Images[0], "data:image/png;base64,"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "SD 图片数据无效: " + err.Error()})
		return
	}
	panelFile := filepath.Join(panelDir, fmt.Sprintf("panel_%02d.png", req.Panel.Index))
	if err := os.WriteFile(panelFile, imgData, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存单格失败: " + err.Error()})
		return
	}
	if pageBytes, err := os.ReadFile(filepath.Join(panelDir, "page.json")); err == nil {
		var page comicPageResponse
		if json.Unmarshal(pageBytes, &page) == nil {
			for i := range page.Panels {
				if page.Panels[i].Index == req.Panel.Index {
					page.Panels[i].ImageURL = fmt.Sprintf("/api/comic/image/%s/panel_%02d.png", pageID, req.Panel.Index)
				}
			}
			updated, _ := json.MarshalIndent(page, "", "  ")
			_ = os.WriteFile(filepath.Join(panelDir, "page.json"), updated, 0o600)
		}
	}

	// 返回图片路径
	imageURL := fmt.Sprintf("/api/comic/image/%s/panel_%02d.png", pageID, req.Panel.Index)
	c.JSON(http.StatusOK, gin.H{
		"pageId":     pageID,
		"panelIndex": req.Panel.Index,
		"imageUrl":   imageURL,
		"status":     "done",
	})
}

// HandleComicAssemble POST /api/comic/assemble —— 后端原生拼页，不依赖 Python/Pillow。
func HandleComicAssemble(c *gin.Context) {
	var req struct {
		PageID   string   `json:"pageId" binding:"required"`
		Title    string   `json:"title"`
		Dialogue []string `json:"dialogue"` // 每格对话
		Panels   []string `json:"panels"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	panelDir := filepath.Join(comicOutputDir(), filepath.Base(req.PageID))
	entries, err := os.ReadDir(panelDir)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "漫画项目不存在"})
		return
	}
	var images []image.Image
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "panel_") || !strings.HasSuffix(strings.ToLower(entry.Name()), ".png") {
			continue
		}
		file, err := os.Open(filepath.Join(panelDir, entry.Name()))
		if err != nil {
			continue
		}
		img, decodeErr := png.Decode(file)
		_ = file.Close()
		if decodeErr == nil {
			images = append(images, img)
		}
	}
	if len(images) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "还没有可拼接的单格图片"})
		return
	}
	cols := 2
	if len(images) == 1 {
		cols = 1
	}
	rows := int(math.Ceil(float64(len(images)) / float64(cols)))
	cellW, cellH, gutter := 512, 768, 8
	canvas := image.NewRGBA(image.Rect(0, 0, cols*cellW+(cols+1)*gutter, rows*cellH+(rows+1)*gutter))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(image.Black), image.Point{}, draw.Src)
	for index, src := range images {
		x, y := gutter+(index%cols)*(cellW+gutter), gutter+(index/cols)*(cellH+gutter)
		sb := src.Bounds()
		scale := math.Max(float64(cellW)/float64(sb.Dx()), float64(cellH)/float64(sb.Dy()))
		cropW, cropH := int(float64(cellW)/scale), int(float64(cellH)/scale)
		crop := image.Rect(sb.Min.X+(sb.Dx()-cropW)/2, sb.Min.Y+(sb.Dy()-cropH)/2, sb.Min.X+(sb.Dx()+cropW)/2, sb.Min.Y+(sb.Dy()+cropH)/2)
		scaleNearest(canvas, image.Rect(x, y, x+cellW, y+cellH), src, crop)
	}
	outPath := filepath.Join(panelDir, "comic_page.png")
	out, err := os.Create(outPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	err = png.Encode(out, canvas)
	_ = out.Close()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pageId": req.PageID, "image": "/api/comic/image/" + req.PageID + "/comic_page.png", "file": outPath})
}

func scaleNearest(dst draw.Image, dstRect image.Rectangle, src image.Image, srcRect image.Rectangle) {
	for y := dstRect.Min.Y; y < dstRect.Max.Y; y++ {
		for x := dstRect.Min.X; x < dstRect.Max.X; x++ {
			sx := srcRect.Min.X + (x-dstRect.Min.X)*srcRect.Dx()/dstRect.Dx()
			sy := srcRect.Min.Y + (y-dstRect.Min.Y)*srcRect.Dy()/dstRect.Dy()
			dst.Set(x, y, src.At(sx, sy))
		}
	}
}

func toJSONString(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// HandleComicCharacters GET /api/comic/characters — 角色库列表
func HandleComicCharacters(c *gin.Context) {
	chars, _ := loadComicCharacters()
	c.JSON(http.StatusOK, gin.H{"characters": chars})
}

// HandleComicCreateCharacter POST /api/comic/characters — 创建角色设定
func HandleComicCreateCharacter(c *gin.Context) {
	var req struct {
		Name   string `json:"name" binding:"required"`
		Gender string `json:"gender"`
		Look   string `json:"look"`
		Style  string `json:"style"`
		RefURL string `json:"refUrl"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	chars, _ := loadComicCharacters()
	chars = append(chars, req)
	chars = saveComicCharacters(chars)

	c.JSON(http.StatusOK, gin.H{"character": req, "characters": chars})
}

// HandleComicList GET /api/comic/pages — 已生成的漫画页列表
func HandleComicList(c *gin.Context) {
	dir := comicOutputDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"pages": []string{}})
		return
	}
	var pages []string
	for _, e := range entries {
		if e.IsDir() {
			pages = append(pages, e.Name())
		}
	}
	c.JSON(http.StatusOK, gin.H{"pages": pages})
}

// HandleComicPage GET /api/comic/page/:id — 获取某页详情
func HandleComicPage(c *gin.Context) {
	pageID := c.Param("id")
	panelDir := filepath.Join(comicOutputDir(), pageID)
	entries, _ := os.ReadDir(panelDir)
	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	if data, err := os.ReadFile(filepath.Join(panelDir, "page.json")); err == nil {
		var page comicPageResponse
		if json.Unmarshal(data, &page) == nil {
			c.JSON(http.StatusOK, gin.H{"pageId": pageID, "files": files, "page": page})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"pageId": pageID, "files": files})
}

// --- 辅助函数 ---

func comicOutputDir() string {
	if root := strings.TrimSpace(os.Getenv("RESCENE_COMICS_DIR")); root != "" {
		return root
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "rescene_data", "comics")
}

func loadComicCharacters() ([]interface{}, error) {
	path := filepath.Join(comicOutputDir(), "characters.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}
	var chars []interface{}
	json.Unmarshal(data, &chars)
	return chars, nil
}

func saveComicCharacters(chars []interface{}) []interface{} {
	path := filepath.Join(comicOutputDir(), "characters.json")
	os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.MarshalIndent(chars, "", "  ")
	os.WriteFile(path, data, 0o600)
	return chars
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	// 去掉 ```json 和 ``` 包裹
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	return s
}
