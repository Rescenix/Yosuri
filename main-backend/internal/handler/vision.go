package handler

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// VisionQA 是一轮历史问答，供多轮追问时把上文带回去（见 AnalyzeImage）。
type VisionQA struct {
	Q string `json:"q"`
	A string `json:"a"`
}

// visionBackends 构建视觉模型 backend 链（负载均衡：按成功率排序，逐个 failover）。
//
// 不再只认一个写死的默认 ID——那样一旦默认模型下线，Rescene 就「有识图模型却不会识图」。
// 改为收集免费池里所有 Vision=true 且当前可用（keyless 或有 key）的条目，
// 按「信号格高 + 最近真实成功过」优先排序成链，routeChatOnce 会依次尝试、失败自动切下一个。
// 用户配置的 VISION_MODEL_ID 若仍指向某个具体模型，则只精确用那一个（保留覆盖能力）。
//
// 2026-08-29 修复：原实现只解析默认 ID free_sf_pro_moonshotai_kimi_k2_6（硅基流动已移除），
// 导致「免费池一堆识图模型，粘贴图片却总失败」。负载均衡后原生识图，任何环境都有可用视觉链。
func visionBackends() []RouterBackend {
	id := os.Getenv("VISION_MODEL_ID")
	if id != "" {
		// 显式覆盖：只精确用这一个（沿用旧行为，供高级用户钉死某个模型）
		bs := resolveBackends("", id)
		for _, b := range bs {
			if b.Vision {
				return []RouterBackend{b}
			}
		}
		return nil
	}

	envKeys := userKeysByEnv("")
	var chain []RouterBackend
	for _, f := range freeModelCatalog {
		if f.Disabled || !f.Vision {
			continue
		}
		key := ""
		entry, hasEntry := capabilityEntry(f.ID)
		if hasEntry {
			key = entry.APIKey
		}
		if key == "" && !f.Local && !f.Keyless {
			key = envKeys[f.KeyEnv]
		}
		if key == "" && !f.Local && !f.Keyless {
			key = os.Getenv(f.KeyEnv)
		}
		if key == "" && !f.Local && !f.Keyless {
			continue // 要 key 但没配：不进视觉链
		}
		chain = append(chain, RouterBackend{
			Name: f.Name, BaseURL: f.Endpoint, Model: f.Model, APIKey: key,
			Timeout: 5 * time.Minute, Source: "free",
			Vision: true, ContextWindow: f.ContextWindow, Reasoning: f.Reasoning,
			Keyless: f.Keyless,
		})
	}

	// 按成功率排序：信号格高（0-4）优先，信号相同时最近真实成功过的优先。
	// 这样「成功率最高的模型」总是排在最前，失败自动切下一个（负载均衡 failover）。
	sort.SliceStable(chain, func(i, j int) bool {
		si, sj := probeSignal(chain[i]), probeSignal(chain[j])
		if si != sj {
			if si == -1 {
				return false // 未探测的沉底
			}
			if sj == -1 {
				return true
			}
			return si > sj
		}
		ui, uj := freeLastUsed(chain[i]), freeLastUsed(chain[j])
		if !ui.IsZero() && !uj.IsZero() {
			if !ui.Equal(uj) {
				return ui.After(uj)
			}
		} else if !ui.IsZero() {
			return true
		} else if !uj.IsZero() {
			return false
		}
		return false
	})
	return chain
}

// analyzeImageViaRouter 用 OpenAI 兼容的视觉格式走通用模型路由（model_router.go）。
// openAIChatOnce 把 msgs 原样塞进 "messages" 再 Marshal，对 content 不做任何强制转换，
// 所以这里可以直接给数组型 content —— 视觉调用因此白拿了整套失败切换逻辑。
func analyzeImageViaRouter(backends []RouterBackend, cleanBase64, question string, history []VisionQA) (string, error) {
	var msgs []map[string]any
	for _, h := range history {
		if h.Q == "" && h.A == "" {
			continue
		}
		// 历史轮只带文字：图已经在下面那条最终 user 消息里，重复携带纯属烧 token
		msgs = append(msgs,
			map[string]any{"role": "user", "content": h.Q},
			map[string]any{"role": "assistant", "content": h.A},
		)
	}
	msgs = append(msgs, map[string]any{
		"role": "user",
		"content": []map[string]any{
			{"type": "text", "text": question},
			// 截图链路给的是 PNG。老的 DashScope 分支写死 jpeg 也能过，是因为它宽容；
			// OpenAI 兼容端点会按 data URI 里声明的类型去解，写错就是解码失败。
			{"type": "image_url", "image_url": map[string]any{
				"url": "data:image/png;base64," + cleanBase64,
			}},
		},
	})

	// 60s 而不是更长：这一步外面还套着 MCP 的整体超时（见 mcpCallTimeout），
	// 主模型卡太久会把回退 qwen 的时间也一起吃掉，最后两边都没结果。
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	content, _, err := routeChatOnce(ctx, backends, msgs, nil)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("视觉模型返回空内容")
	}
	return content, nil
}

// AnalyzeImage 分析图片。走模型路由（默认硅基流动 Kimi K2.6，见 visionBackends）。
// history 非空时把之前的问答对铺在图片这一轮前面，支持"先问整体、再问细节"的连续追问——
// image 只挂在最后一条 user 消息上即可，之前的图不需要重复携带。
func AnalyzeImage(imageBase64 string, question string, history []VisionQA) (string, error) {
	backends := visionBackends()
	if len(backends) == 0 {
		return "", fmt.Errorf("未配置视觉模型（VISION_MODEL_ID 指向的 backend 不存在或未标记 Vision）")
	}
	clean := imageBase64
	if idx := strings.Index(clean, "base64,"); idx != -1 {
		clean = clean[idx+7:]
	}
	text, err := analyzeImageViaRouter(backends, clean, question, history)
	if err != nil {
		return "", err
	}
	fmt.Printf("👁️ [视觉] 由 %s (%s) 完成分析\n", backends[0].Name, backends[0].Model)
	return text, nil
}

// VisionAnalyzeRequest 是 HandleVisionAnalyze 的请求体。
// image_url / image_base64 二选一；history 用于多轮追问（见 AnalyzeImage 注释）。
type VisionAnalyzeRequest struct {
	ImageURL    string     `json:"image_url"`
	ImageBase64 string     `json:"image_base64"`
	Question    string     `json:"question"`
	History     []VisionQA `json:"history"`
}

// HandleVisionAnalyze POST /api/vision/analyze —— 前端看图分析入口。
// Key 和视觉模型路由统一由 Go 管理；内置 view_image 工具直接调用 AnalyzeImage，
// 不再通过 Python stdio MCP 转发。
func HandleVisionAnalyze(c *gin.Context) {
	var req VisionAnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}

	imgB64 := req.ImageBase64
	if imgB64 == "" && req.ImageURL != "" {
		// 不少图床（含 Wikimedia）拿 Go 默认 UA 直接 403；没有 UA 装成浏览器，
		// 拿到的错误页正文会被当成"图片"一路 base64 送进视觉模型，对方只会报
		// "image format is illegal"，看不出真实原因是下载环节被拦了。
		imgReq, err := http.NewRequest("GET", req.ImageURL, nil)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "图片 URL 无效: " + err.Error()})
			return
		}
		imgReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		resp, err := http.DefaultClient.Do(imgReq)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "下载图片失败: " + err.Error()})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("下载图片返回非200: %d, body: %s", resp.StatusCode, string(body))})
			return
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "读取图片失败: " + err.Error()})
			return
		}
		imgB64 = base64.StdEncoding.EncodeToString(data)
	}
	if imgB64 == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_url 和 image_base64 至少提供一个"})
		return
	}

	question := req.Question
	if question == "" {
		question = "请详细描述这张图片的内容"
	}

	text, err := AnalyzeImage(imgB64, question, req.History)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "视觉分析失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"text": text})
}
