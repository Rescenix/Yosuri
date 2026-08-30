package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== 视频理解（2026-08-30 新增） ==========
//
// 原生视频理解：用户上传视频 → ffmpeg 均匀抽 N 帧关键画面 → 多帧作为多张
// image_url 一次性送给视觉模型链（与识图同一套负载均衡 + failover，见
// vision.go 的 visionBackends / routeChatOnce），模型综合各帧内容输出对
// 整段视频的理解。前端把返回文本作为上下文塞进 Agent 流水线即可。
//
// 设计取舍：
//   - 不挑"原生支持视频输入"的模型——免费池绝大多数视觉模型只认图片，
//     抽帧后走多图协议是唯一对所有模型通用的形式，失败还能 failover。
//   - 用 multipart 上传而不是 base64 JSON：视频动辄几十 MB，base64 会把
//     请求体撑大 33% 且整块驻留内存；multipart 流式落盘更稳。
//   - 帧数默认 4、上限 8；每帧缩放到宽 640 控 token（4 帧约几百 KB base64）。

// HandleVideoAnalyze POST /api/video/analyze —— 视频理解入口。
// multipart 字段：
//
//	file        必填，视频文件
//	question    可选，分析指令；默认中文综合描述提示
//	model       可选，设置面板「模型」页选的识图模型 ID；失败回退默认视觉链
//	max_frames  可选，抽帧数（1-8，默认 4）
//
// 返回 {"text": "…", "frames": N}。
func HandleVideoAnalyze(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少视频文件: " + err.Error()})
		return
	}
	question := strings.TrimSpace(c.PostForm("question"))
	if question == "" {
		question = "这是一段视频的若干关键帧，按时间顺序排列。请综合这些画面，用简洁的中文描述这段视频的内容：画面里发生了什么、人物/场景/动作变化、出现的关键文字或界面元素等，供后续 Agent 流水线使用。"
	}
	model := strings.TrimSpace(c.PostForm("model"))
	maxFrames := 4
	if s := strings.TrimSpace(c.PostForm("max_frames")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= 8 {
			maxFrames = n
		}
	}

	tmpDir, err := os.MkdirTemp("", "ameko-video-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时目录失败: " + err.Error()})
		return
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "input"+filepath.Ext(file.Filename))
	if err := c.SaveUploadedFile(file, srcPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存视频失败: " + err.Error()})
		return
	}

	frames, err := extractVideoFrames(srcPath, tmpDir, maxFrames)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(frames) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "视频抽帧失败：没有抽到任何画面"})
		return
	}

	// 1. 显式指定的识图模型（设置面板「模型」页选的）优先
	if model != "" {
		text, err := analyzeVideoFramesWithModelID(c.Request.Context(), model, frames, question)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"text": text, "frames": len(frames)})
			return
		}
		fmt.Printf("⚠️ [视频理解] 指定模型 %s 失败，回退默认视觉链: %v\n", model, err)
	}

	// 2. 默认视觉模型链（visionBackends 负载均衡 failover）
	backends := visionBackends()
	if len(backends) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "视频理解失败: 没有可用的视觉模型（请检查设置面板「模型」页的识图模型配置）"})
		return
	}
	text, err := analyzeVideoFramesWithBackends(c.Request.Context(), backends, frames, question)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "视频理解失败: " + err.Error()})
		return
	}
	fmt.Printf("🎬 [视频理解] %d 帧，由 %s (%s) 完成分析\n", len(frames), backends[0].Name, backends[0].Model)
	c.JSON(http.StatusOK, gin.H{"text": text, "frames": len(frames)})
}

// extractVideoFrames 用 ffmpeg 把视频均匀抽 N 帧（区间中点采样），返回 jpeg base64 列表。
func extractVideoFrames(srcPath, tmpDir string, n int) ([]string, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("未找到 ffmpeg，无法抽帧（请安装 ffmpeg 并加入 PATH）")
	}

	// 先拿时长，决定均匀分布的时间点；拿不到就退化为每秒一帧的近似
	dur := videoDuration(srcPath)
	times := make([]float64, n)
	if dur > 0 {
		// 区间中点采样：把视频等分 N 段，每帧取该段中点，保证首帧 != 0、
		// 末帧 != dur（否则 -ss 落在视频末尾抽不到帧，报 Invalid argument）。
		for i := range times {
			times[i] = dur * (0.5 + float64(i)) / float64(n)
		}
	} else {
		for i := range times {
			times[i] = float64(i)
		}
	}

	var frames []string
	for i, t := range times {
		outPath := filepath.Join(tmpDir, fmt.Sprintf("frame_%02d.jpg", i))
		args := []string{
			"-y",
			"-ss", fmt.Sprintf("%.2f", t),
			"-i", srcPath,
			"-frames:v", "1",
			// 宽 640 高按比例（-2 保持偶数），控 token。不用 min()——min 里的
			// 逗号在 exec.Command 不经 shell 的 Windows 命令行重组下会被错误转义，
			// 导致 ffmpeg 解析失败；去逗号的 scale=640:-2 没有这个问题。
			"-vf", "scale=640:-2",
			"-q:v", "3",
			outPath,
		}
		cmd := hiddenCommand("ffmpeg", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			// ffmpeg 的 banner（版本 + 编译配置）有一千多字符，真正的报错总在
			// 输出的末尾——只取尾部，否则会被 banner 淹没看不到真因。
			return nil, fmt.Errorf("视频抽帧失败: %s", truncateTail(string(out), 800))
		}
		data, err := os.ReadFile(outPath)
		if err != nil {
			return nil, fmt.Errorf("读取抽帧结果失败: %w", err)
		}
		frames = append(frames, base64.StdEncoding.EncodeToString(data))
	}
	return frames, nil
}

// videoDuration 用 ffprobe 读视频时长（秒）；失败返回 0。
func videoDuration(srcPath string) float64 {
	cmd := hiddenCommand("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", srcPath)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || d <= 0 {
		return 0
	}
	return d
}

// analyzeVideoFramesWithModelID 走设置面板选中的具体识图模型（同
// chat_engines_gemini_vision.go 的 analyzeImageWithModelID，不强制 Vision 门禁）。
func analyzeVideoFramesWithModelID(ctx context.Context, modelID string, frames []string, question string) (string, error) {
	b := resolveExact("", modelID)
	if b == nil {
		return "", fmt.Errorf("模型 %s 未找到或未配置 Key", modelID)
	}
	return analyzeVideoFramesWithBackends(ctx, []RouterBackend{*b}, frames, question)
}

// analyzeVideoFramesWithBackends 把 N 帧作为 N 个 image_url 放进一条 user 消息，
// 走 routeChatOnce 的视觉链（失败自动 failover 下一个模型）。
func analyzeVideoFramesWithBackends(ctx context.Context, backends []RouterBackend, frames []string, question string) (string, error) {
	content := []map[string]any{{"type": "text", "text": question}}
	for _, f := range frames {
		content = append(content, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{"url": "data:image/jpeg;base64," + f},
		})
	}
	msgs := []map[string]any{{"role": "user", "content": content}}

	// 多帧 + 云端视觉模型比单图慢，给足 2 分钟
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	text, _, err := routeChatOnce(ctx, backends, msgs, nil)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("视觉模型返回空内容")
	}
	return text, nil
}
