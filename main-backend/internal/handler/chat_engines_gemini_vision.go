package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== Aether 视觉预处理：谷歌 Interactions REST 接口（2026-06 GA） ==========
//
// 官方 Go SDK（google.golang.org/genai）目前还没有 Interactions API 的绑定，
// 只有 Models/Chats/Files 等旧接口，所以这里不依赖任何第三方 SDK，直接手写
// net/http 调 REST 端点——跟 chat_engines_ds.go / chat_engines_cloud.go /
// chat_engines_local.go 这几个引擎的实现方式完全一致。

const (
	geminiInteractionsEndpoint = "https://generativelanguage.googleapis.com/v1beta/interactions"
	geminiVisionModel          = "gemini-2.5-flash"
	geminiDefaultInstruction   = "请分析这张图片，给出简洁的中文结构化描述（画面内容、关键 UI 元素、可见文字），供后续 Agent 流水线使用。"
)

// geminiInteractionRequest 对应 REST 请求体
type geminiInteractionRequest struct {
	Model                 string                   `json:"model"`
	Input                 []geminiInteractionInput `json:"input"`
	PreviousInteractionID string                   `json:"previous_interaction_id,omitempty"`
	Store                 bool                     `json:"store"`
}

// geminiInteractionInput 是 input 数组里的一项：文本或图片
type geminiInteractionInput struct {
	Type     string `json:"type"` // "text" | "image"
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"` // 图片的 base64（不带 data:xxx;base64, 前缀）
	MimeType string `json:"mime_type,omitempty"`
}

// geminiInteractionResponse 对应 REST 原始响应体
// 注意：官方 SDK 里的 output_text 是 SDK 自己拼出来的便利属性，
// 原始 REST JSON 里没有这个字段，真正的文本在 steps[].content[].text 里，
// 需要自己遍历提取。
type geminiInteractionResponse struct {
	ID     string                  `json:"id"`
	Status string                  `json:"status"`
	Steps  []geminiInteractionStep `json:"steps"`
	Error  *geminiInteractionError `json:"error,omitempty"`
}

type geminiInteractionStep struct {
	Type    string                     `json:"type"` // "model_output" | "function_call" | "user_input" | ...
	Content []geminiInteractionContent `json:"content,omitempty"`
}

type geminiInteractionContent struct {
	Type string `json:"type"` // "text" | ...
	Text string `json:"text,omitempty"`
}

type geminiInteractionError struct {
	Message string `json:"message"`
}

// analyzeImageWithGemini 调用 Gemini Interactions REST 接口分析一张图片。
//
// imageBase64: 图片的 base64 编码（纯数据部分，不带 data URI 前缀）
// mimeType:    图片的 MIME 类型，如 "image/png"、"image/jpeg"
// instruction: 给模型的具体指令；传空字符串则用默认的通用视觉预处理指令
// previousInteractionID: 上一次调用返回的 interaction id；传入即可让服务端
//                         沿用之前的会话状态，命中隐式缓存，降低后续 token 消耗；
//                         传空字符串代表开启一条全新的会话链
//
// 返回：模型输出的中文分析文本、这次调用的 interaction id、错误
func analyzeImageWithGemini(
	ctx context.Context,
	imageBase64 string,
	mimeType string,
	instruction string,
	previousInteractionID string,
) (string, string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", "", fmt.Errorf("缺少 GEMINI_API_KEY，请检查 main-backend/.env 是否已正确加载")
	}
	if imageBase64 == "" {
		return "", "", fmt.Errorf("图片 base64 数据为空")
	}
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	if instruction == "" {
		instruction = geminiDefaultInstruction
	}

	reqPayload := geminiInteractionRequest{
		Model: geminiVisionModel,
		Input: []geminiInteractionInput{
			{Type: "text", Text: instruction},
			{Type: "image", Data: imageBase64, MimeType: mimeType},
		},
		// 默认开启持久化：免费层自动保留 1 天数据，足够日常调试，
		// 同时也是 previous_interaction_id 能生效的前提
		Store: true,
	}
	if previousInteractionID != "" {
		reqPayload.PreviousInteractionID = previousInteractionID
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", "", fmt.Errorf("构造请求体失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, geminiInteractionsEndpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", "", fmt.Errorf("构造 HTTP 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", apiKey) // 官方文档确认：走这个 header，不是 Bearer/query 参数

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return "", "", ctx.Err()
		}
		return "", "", fmt.Errorf("调用 Gemini Interactions API 失败: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("Gemini Interactions API 返回错误 %d: %s", resp.StatusCode, string(respBytes))
	}

	var parsed geminiInteractionResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", "", fmt.Errorf("解析响应 JSON 失败: %w", err)
	}
	if parsed.Error != nil {
		return "", "", fmt.Errorf("Gemini 返回错误: %s", parsed.Error.Message)
	}

	var textBuilder strings.Builder
	for _, step := range parsed.Steps {
		if step.Type != "model_output" {
			continue
		}
		for _, item := range step.Content {
			if item.Type == "text" {
				textBuilder.WriteString(item.Text)
			}
		}
	}
	outputText := textBuilder.String()
	if outputText == "" {
		return "", parsed.ID, fmt.Errorf("Gemini 未返回任何文本内容（status=%s）", parsed.Status)
	}

	return outputText, parsed.ID, nil
}

// aetherVisionRequest 是前端上传图片时的请求体
type aetherVisionRequest struct {
	ImageBase64           string `json:"image_base64" binding:"required"`
	MimeType              string `json:"mime_type"`
	Instruction           string `json:"instruction"`
	PreviousInteractionID string `json:"previous_interaction_id"`
}

// HandleAetherVisionPreprocess POST /api/aether/vision-preprocess
// 接收前端上传的图片，调用 Gemini 做视觉预处理，把分析出的中文文本
// 连同这次的 interaction id 一起回传给前端；前端后续把这段文本
// 作为上下文塞进 Agent 流水线（/api/workflow/run 的 task 里）即可。
func HandleAetherVisionPreprocess(c *gin.Context) {
	var req aetherVisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	outputText, interactionID, err := analyzeImageWithGemini(
		c.Request.Context(),
		req.ImageBase64,
		req.MimeType,
		req.Instruction,
		req.PreviousInteractionID,
	)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "视觉预处理失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"text":           outputText,
		"interaction_id": interactionID,
	})
}
