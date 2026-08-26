package handler

// video_watermark_tool.go —— AI 视频去水印工具（2026-08-25）
//
// 免费视频生成网站（Google AI Studio / Gemini / Veo 等）出片自带右下角
// 半透明 "veo" 可见水印（Google 官方双层水印机制：可见水印 + SynthID
// 不可见水印；SynthID 肉眼不可见，无需处理）。本工具用 ffmpeg delogo
// 抹除右下角水印区域，并重编码清理 C2PA 等元数据。
//
// 水印位置策略：
//   - 默认预设 gemini：按视频分辨率比例定位右下角（veo 文字水印区域）
//   - 可手动指定 x/y/w/h 覆盖，适配其他网站水印

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"backend/internal/ai/core"
)

// watermarkToolDef video_watermark_remove 工具定义（随 nativeOnDemandToolDefs 按需加载）。
var watermarkToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        "video_watermark_remove",
		Description: "去除 AI 视频右下角可见水印（Gemini/Veo/免费视频生成网站出片自带）。用 ffmpeg delogo 抹除水印区域并重编码清理元数据。默认按 Gemini veo 右下角水印位置处理，也可手动指定水印矩形坐标。返回去水印后视频的绝对路径。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"video": {
					Type:        "string",
					Description: "输入视频绝对路径（必填）",
				},
				"out": {
					Type:        "string",
					Description: "可选，输出视频路径；缺省为 <视频名>_clean.mp4（同目录）",
				},
				"preset": {
					Type:        "string",
					Description: "可选，水印预设：gemini（默认，右下角 veo 半透明文字，按分辨率比例定位）；自定义坐标时忽略",
				},
				"x": {
					Type:        "integer",
					Description: "可选，水印矩形左上角 x 像素；与 y/w/h 同时给时覆盖预设",
				},
				"y": {
					Type:        "integer",
					Description: "可选，水印矩形左上角 y 像素",
				},
				"w": {
					Type:        "integer",
					Description: "可选，水印矩形宽（像素）",
				},
				"h": {
					Type:        "integer",
					Description: "可选，水印矩形高（像素）",
				},
			},
			Required: []string{"video"},
		},
	},
}

// callWatermarkRemove video_watermark_remove 工具实现。
// 策略：优先用 VeoWatermarkRemover（GeminiWatermarkTool-Video.exe，逆向 alpha
// 混合精确还原，自动检测 Gemini diamond / Veo 文字水印）；exe 不存在、SKIP
// 或失败时，用 ffmpeg delogo 兜底（预设定位右下角）。
func callWatermarkRemove(ctx context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		Video  string `json:"video"`
		Out    string `json:"out"`
		Preset string `json:"preset"`
		X      int    `json:"x"`
		Y      int    `json:"y"`
		W      int    `json:"w"`
		H      int    `json:"h"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
	}
	if strings.TrimSpace(args.Video) == "" {
		return nativeToolResult{}, fmt.Errorf("video 必填")
	}
	in, err := filepath.Abs(args.Video)
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("解析输入路径失败: %w", err)
	}
	if _, err := os.Stat(in); err != nil {
		return nativeToolResult{}, fmt.Errorf("输入视频不存在: %s", in)
	}

	// 输出路径
	out := strings.TrimSpace(args.Out)
	if out == "" {
		ext := filepath.Ext(in)
		base := strings.TrimSuffix(in, ext)
		out = base + "_clean" + ext
	}
	if abs, err := filepath.Abs(out); err == nil {
		out = abs
	}

	// 优先：GWT exe（自动检测 + 逆向 alpha 还原）
	if gwt, err := findGWT(); err == nil {
		if text, ok := runGWT(ctx, gwt, in, out, args.Preset); ok {
			return nativeToolResult{Text: text}, nil
		} else {
			// GWT SKIP/失败：如果手动指定了坐标，直接 delogo；
			// 否则 GWT 已确认无 diamond 水印时仍按预设 delogo 兜底
			_ = text
		}
	}

	// 兜底：ffmpeg delogo
	return callWatermarkDelogo(ctx, in, out, args)
}

// runGWT 调 GeminiWatermarkTool-Video.exe 去水印。
// 返回 (日志文本, 是否成功产出)。--legacy 处理旧版 Veo 文字水印。
func runGWT(ctx context.Context, gwt, in, out, preset string) (string, bool) {
	cmdArgs := []string{"-i", in, "-o", out}
	if strings.EqualFold(strings.TrimSpace(preset), "legacy") {
		cmdArgs = append([]string{"--legacy"}, cmdArgs...)
	}
	cmd := hiddenCommandContext(ctx, gwt, cmdArgs...)
	output, err := cmd.CombinedOutput()
	log := truncateTail(string(output), 800)
	if err != nil {
		return fmt.Sprintf("GWT 去水印失败: %s", log), false
	}
	if _, statErr := os.Stat(out); statErr != nil {
		return fmt.Sprintf("GWT 未产出文件（%s），改用 delogo 兜底。%s", out, log), false
	}
	return fmt.Sprintf("✅ 去水印完成（VeoWatermarkRemover 逆向 alpha 还原）：%s\n  输出：%s\n  日志：%s", in, out, log), true
}

// findGWT 定位 GeminiWatermarkTool-Video.exe（main-backend/tools/veo-watermark-remover/）。
func findGWT() (string, error) {
	// 优先从 backendRoot 推算
	if root, err := backendRoot(); err == nil {
		candidates := []string{
			filepath.Join(root, "tools", "veo-watermark-remover", "GeminiWatermarkTool-Video.exe"),
			filepath.Join(root, "tools", "veo-watermark-remover", "GeminiWatermarkTool-Video"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c, nil
			}
		}
	}
	// PATH 兜底
	if p, err := exec.LookPath("GeminiWatermarkTool-Video"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("找不到 GeminiWatermarkTool-Video.exe")
}

// callWatermarkDelogo ffmpeg delogo 兜底：按预设/手动坐标抹除水印并清元数据。
func callWatermarkDelogo(ctx context.Context, in, out string, args struct {
	Video  string `json:"video"`
	Out    string `json:"out"`
	Preset string `json:"preset"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	W      int    `json:"w"`
	H      int    `json:"h"`
}) (nativeToolResult, error) {
	x, y, w, h := args.X, args.Y, args.W, args.H
	manual := x > 0 && y > 0 && w > 0 && h > 0
	if !manual {
		// 按预设定位：先 ffprobe 读分辨率
		W, H, err := probeVideoSize(ctx, in)
		if err != nil {
			return nativeToolResult{}, err
		}
		preset := strings.ToLower(strings.TrimSpace(args.Preset))
		if preset == "" || preset == "legacy" {
			preset = "gemini"
		}
		switch preset {
		case "gemini":
			// veo 半透明文字水印在右下角（距边 ~15-20px）：区域取
			// 6% 屏宽 × 4.5% 屏高，距右 1% 屏宽、距下 2% 屏高。
			// 实测：delogo 区域必须完整盖住水印块，顶部漏 6px 就会留黑边；
			// 区域过大在纯色背景会渐变过渡，精确匹配是关键。
			w = int(float64(W) * 0.06)
			h = int(float64(H) * 0.045)
			x = W - w - int(float64(W)*0.01)
			y = H - h - int(float64(H)*0.02)
		default:
			return nativeToolResult{}, fmt.Errorf("未知水印预设: %s（支持 gemini 或手动 x/y/w/h）", preset)
		}
	}
	// delogo 区域至少 8px，且不越界
	if w < 8 {
		w = 8
	}
	if h < 8 {
		h = 8
	}

	ffmpeg, err := findFFmpeg()
	if err != nil {
		return nativeToolResult{}, err
	}

	// delogo 抹除 + 重编码（清 C2PA 元数据）
	delogo := fmt.Sprintf("delogo=x=%d:y=%d:w=%d:h=%d", x, y, w, h)
	cmdArgs := []string{
		"-i", in,
		"-vf", delogo,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "20",
		"-pix_fmt", "yuv420p",
		"-map_metadata", "-1", // 丢弃全部元数据（含 C2PA 痕迹）
		"-c:a", "copy",
		"-y", out,
	}
	cmd := hiddenCommandContext(ctx, ffmpeg, cmdArgs...)
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("去水印失败: %s", truncateTail(string(outBytes), 600))
	}
	if _, err := os.Stat(out); err != nil {
		return nativeToolResult{}, fmt.Errorf("去水印后未找到输出文件: %s", out)
	}

	return nativeToolResult{Text: fmt.Sprintf(
		"✅ 去水印完成（ffmpeg delogo 兜底）：%s\n  水印区域 (x=%d, y=%d, w=%d, h=%d) 已抹除，元数据已清理，输出：%s",
		in, x, y, w, h, out)}, nil
}

// probeVideoSize 用 ffprobe 读取视频分辨率（宽x高）。
func probeVideoSize(ctx context.Context, path string) (w, h int, err error) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		return 0, 0, fmt.Errorf("找不到 ffprobe（请确认 ffmpeg 已安装并在 PATH）")
	}
	cmd := hiddenCommandContext(ctx, ffprobe,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=s=x:p=0",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("ffprobe 读取分辨率失败: %w", err)
	}
	s := strings.TrimSpace(string(out))
	var dim [2]int
	if _, err := fmt.Sscanf(s, "%dx%d", &dim[0], &dim[1]); err != nil || dim[0] <= 0 || dim[1] <= 0 {
		return 0, 0, fmt.Errorf("无法解析分辨率: %q", s)
	}
	return dim[0], dim[1], nil
}

// findFFmpeg 定位 ffmpeg：PATH 优先，找不到时报错。
func findFFmpeg() (string, error) {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("ffmpeg.exe"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("找不到 ffmpeg（请安装并加入 PATH）")
}
