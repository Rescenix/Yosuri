package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// askDeepSeekWithMessages 调用 DeepSeek API，返回 (回复内容, Token消耗, 延迟毫秒)
func askDeepSeekWithMessages(messages []DSMessage, temperature float64, topP float64, maxTokens int, reasoningEffort string) (string, int, int64) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	model := os.Getenv("DEEPSEEK_MODEL")
	if apiKey == "" || model == "" {
		log.Println("缺少必要的环境变量")
		return "抱歉，顾问配置错误，请联系管理员。", 0, 0
	}

	reqBody := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}
	// 思考模式开关：如果前端传了 reasoning_effort，就开启思考
	if reasoningEffort != "" {
		reqBody["extra_body"] = map[string]interface{}{
			"thinking": map[string]string{
				"type": "enabled",
			},
		}
		reqBody["reasoning_effort"] = reasoningEffort
	}
	// 动态注入其他参数
	if temperature > 0 {
		reqBody["temperature"] = temperature
	}
	if topP > 0 {
		reqBody["top_p"] = topP
	}
	if maxTokens > 0 {
		reqBody["max_tokens"] = maxTokens
	}

	reqBytes, _ := json.Marshal(reqBody)

	client := &http.Client{}
	request, _ := http.NewRequest("POST", "https://api.deepseek.com/v1/chat/completions", bytes.NewBuffer(reqBytes))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+apiKey)

	startTime := time.Now()
	resp, err := client.Do(request)
	latency := time.Since(startTime).Milliseconds() // 请求延迟（毫秒）
	if err != nil {
		log.Println("请求DeepSeek失败:", err)
		return "抱歉，顾问暂时无法连接。", 0, latency
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var dsResp struct {
		Choices []struct {
			Message DSMessage `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(respBytes, &dsResp)

	tokenUsage := dsResp.Usage.TotalTokens

	if len(dsResp.Choices) == 0 {
		return "顾问暂时没有合适的回答。", tokenUsage, latency
	}

	// 在 return 语句之前添加
	fmt.Printf("📊 请求耗时: %dms, Token消耗: %d\n", latency, tokenUsage)
	return cleanInvalidChars(dsResp.Choices[0].Message.Content), tokenUsage, latency
}

// askDeepSeekSimple 调用 DeepSeek API 处理单轮简单请求（保持不变，不添加调试参数）
func askDeepSeekSimple(prompt string) string {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	model := os.Getenv("DEEPSEEK_MODEL")
	if apiKey == "" || model == "" {
		return ""
	}
	reqBody := DSReq{
		Model: model,
		Messages: []DSMessage{
			{Role: "user", Content: prompt},
		},
	}
	reqBytes, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.deepseek.com/v1/chat/completions", bytes.NewBuffer(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := new(http.Client).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var dsResp DSResp
	json.NewDecoder(resp.Body).Decode(&dsResp)
	if len(dsResp.Choices) == 0 {
		return ""
	}
	return dsResp.Choices[0].Message.Content
}
