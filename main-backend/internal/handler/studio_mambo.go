package handler

// studio_mambo.go —— 创作工作台「文案成片」API（2026-08-06）
//
// POST /api/studio/mambo    粘贴文案 → 一键生成曼波视频（联网搜素材拼接）
//   请求: {topic, text, voice?, rate?, out?}
//   响应: {ok, video, srt, manifest, duration, segments}
//
// 与 agent 工具 mambo_video 共用同一个 Python 引擎 scripts/mambo_video.py；
// 工作台走直接 REST（不走 LLM），适合「粘贴文案 → 生成 → 剪辑 → 重新生成 → 导出」
// 的必剪式工作流。剪辑 = 前端改文案段落（删/换序）后带同一 out 再次 POST，
// 引擎重新 TTS + 重新配素材，保证素材、字幕、语音三者始终对齐。
//
// 产物统一放 main-backend/test_output/studio/，由 /api/studio/files/* 静态服务
// 提供播放与下载（Wails 前端 video 标签可直接引用）。

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// studioOutputDir 工作台产物目录（相对 backendRoot）
const studioOutputDir = "test_output/studio"

// studioManifest 引擎返回的结构
type studioManifest struct {
	Topic    string `json:"topic"`
	Voice    string `json:"voice"`
	Rate     string `json:"rate"`
	Duration float64 `json:"duration"`
	Segments []struct {
		Index       int      `json:"index"`
		Sentence    string   `json:"sentence"`
		Keywords    []string `json:"keywords"`
		Topic       string   `json:"topic"`
		SearchTerms []string `json:"search_terms"`
		Duration    float64  `json:"duration"`
		Source      string   `json:"source"`
	} `json:"segments"`
}

// HandleStudioMambo POST /api/studio/mambo
func HandleStudioMambo(c *gin.Context) {
	var req struct {
		Topic     string `json:"topic"`
		Text      string `json:"text"`
		Voice     string `json:"voice"`
		Rate      string `json:"rate"`
		Out       string `json:"out"`
		PexelsKey string `json:"pexels_key"`
		VideoOnly bool  `json:"video_only"`
		Width     int   `json:"width"`
		Height    int   `json:"height"`
		Compose   bool  `json:"compose"`
		Orientation string `json:"orientation"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体解析失败: " + err.Error()})
		return
	}
	if strings.TrimSpace(req.Text) == "" && strings.TrimSpace(req.Topic) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text 或 topic 至少填一个"})
		return
	}

	root, err := backendRoot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	py, err := findPython()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	outDir := filepath.Join(root, studioOutputDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "产物目录创建失败: " + err.Error()})
		return
	}
	// 文件名：主题拼音/去特殊字符 + 时间戳；剪辑重生成时用请求里的 out（保持同名覆盖）
	base := req.Out
	if base == "" {
		safe := regexpReplaceNonWord(req.Topic)
		if safe == "" {
			safe = "mambo"
		}
		base = fmt.Sprintf("%s_%s", safe, time.Now().Format("20060102_150405"))
	}
	// 素材模式（video_only / compose）：out 是目录（seg_XXX.mp4 + script.txt + manifest.json）
	outTarget := filepath.Join(outDir, base+".mp4")
	if req.VideoOnly || req.Compose {
		outTarget = filepath.Join(outDir, base)
	}

	// LLM 语义分析（可选）：理解每句文案语义 → 主题标签 + 英文搜索词 → 素材库精准搜索。
	// 失败时返回 nil，引擎自动降级到关键词+映射表兜底，绝不影响成片。
	var semantic *studioSemanticResult
	if req.Text != "" {
		if segs := splitStudioSentences(req.Text); len(segs) > 0 {
			semantic = analyzeStudioSegments(c.Request.Context(), req.Topic, segs)
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Minute)
	defer cancel()

	cmdArgs := []string{filepath.Join(root, "scripts", "mambo_video.py"), "--topic", req.Topic, "--out", outTarget}
	if req.VideoOnly || req.Compose {
		cmdArgs = append(cmdArgs, "--video-only")
		if req.Width > 0 {
			cmdArgs = append(cmdArgs, "--width", strconv.Itoa(req.Width))
		}
		if req.Height > 0 {
			cmdArgs = append(cmdArgs, "--height", strconv.Itoa(req.Height))
		}
	}
	if req.Text != "" {
		cmdArgs = append(cmdArgs, "--text", req.Text)
	}
	if semantic != nil {
		if sj, err := json.Marshal(semantic); err == nil {
			cmdArgs = append(cmdArgs, "--semantic", string(sj))
		}
	}
	if req.Voice != "" {
		cmdArgs = append(cmdArgs, "--voice", req.Voice)
	}
	if req.Rate != "" {
		cmdArgs = append(cmdArgs, "--rate", req.Rate)
	}
	if req.PexelsKey != "" {
		cmdArgs = append(cmdArgs, "--pexels-key", req.PexelsKey)
	}

	cmd := hiddenCommandContext(ctx, py, cmdArgs...)
	cmd.Dir = root

	// SSE 流式进度
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	flusher := c.Writer.(http.Flusher)

	stderrPipe, _ := cmd.StderrPipe()
	stdoutPipe, _ := cmd.StdoutPipe()
	cmd.Start()

	// 读 stderr → 进度事件
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "[mambo]") {
				fmt.Fprintf(c.Writer, "event: progress\ndata: %s\n\n", strings.TrimSpace(line))
				flusher.Flush()
			}
		}
	}()

	// 读 stdout → 最终结果
	stdoutBytes, _ := io.ReadAll(stdoutPipe)
	cmd.Wait()

	if cmd.ProcessState.ExitCode() != 0 {
		fmt.Fprintf(c.Writer, "event: error\ndata: 引擎执行失败\n\n")
		flusher.Flush()
		return
	}

	var res struct {
		Ok       bool    `json:"ok"`
		Video    string  `json:"video"`
		Srt      string  `json:"srt"`
		Manifest string  `json:"manifest"`
		Duration float64 `json:"duration"`
		Segments int     `json:"segments"`
		OutDir   string  `json:"out_dir"`
	}
	if err := json.Unmarshal(stdoutBytes, &res); err != nil || !res.Ok {
		fmt.Fprintf(c.Writer, "event: error\ndata: 引擎返回异常: %s\n\n", truncateTail(string(stdoutBytes), 500))
		flusher.Flush()
		return
	}

	fmt.Fprintf(c.Writer, "event: progress\ndata: ✅ 素材搜索完成\n\n")
	flusher.Flush()

	// 素材模式：返回素材包目录
		if req.VideoOnly {
			man := studioManifest{}
			if mb, err := os.ReadFile(res.Manifest); err == nil {
				_ = json.Unmarshal(mb, &man)
			}
			resultJSON, _ := json.Marshal(gin.H{
				"ok":        true,
				"out_dir":   res.OutDir,
				"videoOnly": true,
				"manifest":  res.Manifest,
				"duration":  res.Duration,
				"segments":  man.Segments,
			})
			fmt.Fprintf(c.Writer, "event: result\ndata: %s\n\n", string(resultJSON))
			flusher.Flush()
			return
		}

		// 默认成片模式：返回最终视频
		man := studioManifest{}
		if mb, err := os.ReadFile(res.Manifest); err == nil {
			_ = json.Unmarshal(mb, &man)
		}
		relVideo := filepath.ToSlash(filepath.Base(res.Video))
		resultJSON, _ := json.Marshal(gin.H{
			"ok":        true,
			"video":     "/api/studio/files/" + relVideo,
			"videoPath": res.Video,
			"srtPath":   res.Srt,
			"manifest":  res.Manifest,
			"duration":  res.Duration,
			"segments":  man.Segments,
		})
		fmt.Fprintf(c.Writer, "event: result\ndata: %s\n\n", string(resultJSON))
		flusher.Flush()
	}

// HandleStudioFiles 静态服务：/api/studio/files/* → test_output/studio/
func HandleStudioFiles(c *gin.Context) {
	root, err := backendRoot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	name := filepath.Base(c.Param("file"))
	// 只放行视频/字幕/清单/图片四种产物（png 供短剧工作台模板卡缩略图）
	if !strings.HasSuffix(name, ".mp4") && !strings.HasSuffix(name, ".srt") &&
		!strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".png") {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅允许 mp4/srt/json/png"})
		return
	}
	// 优先 studio 产物目录，找不到再从素材目录（~/rescene_data/videos）服务
	p := filepath.Join(root, studioOutputDir, name)
	if _, err := os.Stat(p); err != nil {
		p = filepath.Join(videoOutputDir(), name)
	}
	http.ServeFile(c.Writer, c.Request, p)
}

// HandleStudioLibrary GET /api/studio/library —— 扫描本地素材目录返回素材列表
// （素材库是基础设施：角色图/视频/抽帧图动态入库，不硬编码）
func HandleStudioLibrary(c *gin.Context) {
	type asset struct {
		Name string `json:"name"`
		Kind string `json:"kind"` // image / video
		Src  string `json:"src"`
		Dur  string `json:"dur,omitempty"`
	}
	var out []asset
	seen := map[string]bool{}
	add := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			lower := strings.ToLower(name)
			kind := ""
			if strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") || strings.HasSuffix(lower, ".mov") {
				kind = "video"
			} else if strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".webp") {
				kind = "image"
			}
			if kind == "" || seen[name] {
				continue
			}
			seen[name] = true
			dur := ""
			if kind == "video" {
				dur = "00:05"
			}
			out = append(out, asset{Name: name, Kind: kind, Src: "/api/studio/files/" + name, Dur: dur})
		}
	}
	// 素材目录：生成视频 + 角色图 + 基准图
	add(filepath.Join(videoOutputDir()))            // ~/rescene_data/videos
	add(filepath.Join(resceneUserDataDir(), "videos")) // 兜底同目录
	root, _ := backendRoot()
	add(filepath.Join(root, "drama", "assets", "characters"))
	add(filepath.Join(root, "drama", "assets", "refs"))
	c.JSON(http.StatusOK, gin.H{"assets": out})
}

// HandleStudioLibraryDelete DELETE /api/studio/library/:file —— 删除素材文件
// （仅删素材目录/产物目录内文件，杜绝路径穿越）
func HandleStudioLibraryDelete(c *gin.Context) {
	name := filepath.Base(c.Param("file"))
	if name == "." || name == ".." {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件名"})
		return
	}
	root, _ := backendRoot()
	candidates := []string{
		filepath.Join(videoOutputDir(), name),
		filepath.Join(root, studioOutputDir, name),
		filepath.Join(root, "drama", "assets", "characters", name),
		filepath.Join(root, "drama", "assets", "refs", name),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			if err := os.Remove(p); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"ok": true, "deleted": name})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "素材不存在"})
}

// HandleStudioUpload POST /api/studio/upload —— 上传参考素材（图片/视频）到素材库
func HandleStudioUpload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件"})
		return
	}
	name := filepath.Base(file.Filename)
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".png") && !strings.HasSuffix(lower, ".jpg") &&
		!strings.HasSuffix(lower, ".jpeg") && !strings.HasSuffix(lower, ".webp") &&
		!strings.HasSuffix(lower, ".mp4") && !strings.HasSuffix(lower, ".webm") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持图片(png/jpg/webp)或视频(mp4/webm)"})
		return
	}
	dir := videoOutputDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败"})
		return
	}
	dst := filepath.Join(dir, name)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	kind := "image"
	if strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") {
		kind = "video"
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "name": name, "kind": kind, "src": "/api/studio/files/" + name})
}

// HandleStudioExtractFrame POST /api/studio/frames —— 视频抽帧转图片参考
// body: {video: "xxx.mp4", time: 2.5} → ffmpeg 抽帧 → 存素材库 → 返回图片
func HandleStudioExtractFrame(c *gin.Context) {
	var req struct {
		Video string  `json:"video"`
		Time  float64 `json:"time"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	name := filepath.Base(req.Video)
	if !strings.HasSuffix(strings.ToLower(name), ".mp4") && !strings.HasSuffix(strings.ToLower(name), ".webm") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 mp4/webm 视频抽帧"})
		return
	}
	// 找视频文件
	root, _ := backendRoot()
	candidates := []string{
		filepath.Join(videoOutputDir(), name),
		filepath.Join(root, studioOutputDir, name),
	}
	srcPath := ""
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			srcPath = p
			break
		}
	}
	if srcPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "视频不存在: " + name})
		return
	}
	if req.Time < 0 {
		req.Time = 0
	}
	// ffmpeg 抽帧（毫秒级）
	outName := strings.TrimSuffix(name, filepath.Ext(name)) + "_frame.png"
	outPath := filepath.Join(videoOutputDir(), outName)
	args := []string{"-y", "-ss", fmt.Sprintf("%.2f", req.Time), "-i", srcPath, "-frames:v", "1", "-q:v", "2", outPath}
	cmd := exec.Command("ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "抽帧失败: " + err.Error() + " " + truncateChars(string(out), 200)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "name": outName, "kind": "image", "src": "/api/studio/files/" + outName})
}

func regexpReplaceNonWord(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
