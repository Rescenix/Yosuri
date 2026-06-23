// chat_engines.go — 三引擎版，修复 DS 工具调用上下文混乱

package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

const maxHistoryMessages = 30 // 适当增加，减少上下文断裂

// ---------- 本地 Ollama (增强版：双协议解析 + 重试机制) ----------
func (h *ChatHandler) resolveLocalConversation(
	c *gin.Context,
	systemPrompt string,
	userMessage string,
	sessionID string,
	temperature, topP float64,
	maxTokens int,
	reasoningEffort string,
	depth int,
) (string, string, int, error) {
	fmt.Printf("📡 [本地] 开始调用 Ollama，depth=%d\n", depth)

	model := "qwen2.5-coder:7b"
	ollamaURL := "http://localhost:11434/api/chat"
	client := &http.Client{Timeout: 5 * time.Minute}

	history := h.sessionStore.Get(sessionID)
	history = truncateHistory(history, maxHistoryMessages)
	chatMessages := buildChatMessages(systemPrompt, history, userMessage)

	reqBody := map[string]interface{}{
		"model":    model,
		"messages": chatMessages,
		"stream":   false,
		"options": map[string]interface{}{
			"temperature": temperature,
			"top_p":       topP,
		},
	}
	body, _ := json.Marshal(reqBody)
	fmt.Printf("📡 [本地] 请求体长度: %d 字节\n", len(body))

	httpReq, _ := http.NewRequest("POST", ollamaURL, bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		fmt.Printf("❌ [本地] 请求失败: %v\n", err)
		return "", "", 0, fmt.Errorf("本地模型请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("❌ [本地] 返回错误 %d: %s\n", resp.StatusCode, string(bodyBytes))
		return "", "", 0, fmt.Errorf("本地模型返回错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var ollamaResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		fmt.Printf("❌ [本地] 解析响应失败: %v\n", err)
		return "", "", 0, fmt.Errorf("解析本地模型响应失败: %w", err)
	}

	finalContent := ollamaResp.Message.Content
	fmt.Printf("✅ [本地] 收到回复，长度 %d 字符\n", len(finalContent))

	if depth == 0 {
		// 使用增强版解析器（支持 JSON + XML）
		if tc, ok := parseToolCallFromText(finalContent); ok {
			fmt.Printf("🔧 [本地] 检测到工具调用: %s, 参数: %v\n", tc.Tool, tc.Args)
			writeSSE(c, "tool_call", "tool_call_start", map[string]string{
				"name": tc.Tool,
				"args": fmt.Sprintf("%v", tc.Args),
			})
			c.Writer.Flush()

			resultContent, err := h.executeToolAndNotify(c, sessionID, finalContent, *tc)
			if err != nil {
				fmt.Printf("❌ [本地] 工具执行失败: %v\n", err)
			} else {
				fmt.Printf("✅ [本地] 工具执行成功，结果长度 %d\n", len(resultContent))
				followUpMsg := fmt.Sprintf("你的上一个操作（%s）已经完成，结果是：%s。请用自然的语气告诉朋友发生了什么。", tc.Tool, resultContent)
				return h.resolveConversation(c, systemPrompt, followUpMsg, sessionID, temperature, topP, maxTokens, reasoningEffort, depth+1, "local", "", "")
			}
		} else if strings.Contains(finalContent, "不能") || strings.Contains(finalContent, "无法") || strings.Contains(finalContent, "直接读取") || strings.Contains(finalContent, "无法访问") {
			// 模型拒绝执行工具调用，进行重试
			fmt.Printf("⚠️ [本地] 模型拒绝工具调用，重试中...\n")
			retryMsg := fmt.Sprintf("你刚才回复了：%s\n这是错误的。你必须只输出JSON对象，不要输出任何其他文字。请重新回答：%s", finalContent, userMessage)
			return h.resolveConversation(c, systemPrompt, retryMsg, sessionID, temperature, topP, maxTokens, reasoningEffort, depth+1, "local", "", "")
		}
	}

	return finalContent, "", 0, nil
}

// ---------- DeepSeek API (修复上下文混乱版) ----------
func (h *ChatHandler) resolveDSConversation(
	c *gin.Context,
	systemPrompt string,
	userMessage string,
	sessionID string,
	temperature, topP float64,
	maxTokens int,
	reasoningEffort string,
	apiKey string,
	dsModel string,
) (string, string, int, error) {
	fmt.Println("🚀 [DS] 开始调用 DeepSeek API")

	if apiKey == "" {
		apiKey = os.Getenv("DEEPSEEK_API_KEY")
	}
	if apiKey == "" {
		fmt.Println("❌ [DS] 缺少 API Key")
		return "", "", 0, fmt.Errorf("缺少 DeepSeek API Key，请在设置面板中填写")
	}

	model := dsModel
	if model == "" {
		model = os.Getenv("DEEPSEEK_MODEL")
	}
	if model == "" {
		model = "deepseek-v4-flash"
	}
	fmt.Printf("🚀 [DS] 使用模型: %s\n", model)

	history := h.sessionStore.Get(sessionID)
	history = truncateHistory(history, maxHistoryMessages)
	chatMessages := buildChatMessages(systemPrompt, history, userMessage)

	var dsMessages []map[string]interface{}
	for _, m := range chatMessages {
		if m["role"] == "tool" {
			continue
		}
		msg := map[string]interface{}{
			"role":    m["role"],
			"content": m["content"],
		}
		if tc, ok := m["tool_calls"]; ok {
			msg["tool_calls"] = tc
		}
		dsMessages = append(dsMessages, msg)
	}

	reqBody := map[string]interface{}{
		"model":       model,
		"messages":    dsMessages,
		"stream":      true,
		"temperature": temperature,
		"top_p":       topP,
	}

	if maxTokens > 0 {
		reqBody["max_tokens"] = maxTokens
		fmt.Printf("🚀 [DS] max_tokens: %d\n", maxTokens)
	}

	toolCount := len(core.ChatTools)
	if toolCount > 0 {
		var dsTools []map[string]interface{}
		for _, tool := range core.ChatTools {
			dsTools = append(dsTools, map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        tool.Function.Name,
					"description": tool.Function.Description,
					"parameters":  tool.Function.Parameters,
				},
			})
		}
		reqBody["tools"] = dsTools
		fmt.Printf("🚀 [DS] 携带 %d 个工具定义\n", toolCount)
	} else {
		fmt.Println("⚠️ [DS] 没有工具定义，只进行纯对话")
	}

	body, _ := json.Marshal(reqBody)
	fmt.Printf("🚀 [DS] 请求体大小: %d 字节\n", len(body))

	httpReq, _ := http.NewRequest("POST", "https://api.deepseek.com/chat/completions", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		fmt.Printf("❌ [DS] 请求失败: %v\n", err)
		return "", "", 0, fmt.Errorf("DS请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("❌ [DS] API 返回错误 %d: %s\n", resp.StatusCode, string(bodyBytes))
		return "", "", 0, fmt.Errorf("DS API 返回错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	fmt.Println("✅ [DS] 流式连接建立，开始读取...")

	reader := bufio.NewReader(resp.Body)
	var fullContent strings.Builder
	toolCallsMap := make(map[int]*core.ToolCall)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("❌ [DS] 读取流失败: %v\n", err)
			return "", "", 0, fmt.Errorf("读取DS流失败: %w", err)
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			fmt.Println("✅ [DS] 流结束")
			break
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		choices, ok := event["choices"].([]interface{})
		if !ok || len(choices) == 0 {
			continue
		}
		choice, ok := choices[0].(map[string]interface{})
		if !ok {
			continue
		}
		delta, ok := choice["delta"].(map[string]interface{})
		if !ok {
			continue
		}

		if content, ok := delta["content"].(string); ok && content != "" {
			fullContent.WriteString(content)
			writeSSE(c, "content", "content", map[string]string{"content": content})
			c.Writer.Flush()
		}

		if rawToolCalls, ok := delta["tool_calls"].([]interface{}); ok {
			for _, rawCall := range rawToolCalls {
				callMap, _ := rawCall.(map[string]interface{})

				idxFloat, hasIdx := callMap["index"].(float64)
				if !hasIdx {
					continue
				}
				idx := int(idxFloat)

				if _, exists := toolCallsMap[idx]; !exists {
					toolCallsMap[idx] = &core.ToolCall{Type: "function"}
				}
				tc := toolCallsMap[idx]

				if id, ok := callMap["id"].(string); ok && id != "" {
					tc.ID = id
				}
				if fnMap, ok := callMap["function"].(map[string]interface{}); ok {
					if name, ok := fnMap["name"].(string); ok && name != "" {
						tc.Function.Name = name
					}
					if argsStr, ok := fnMap["arguments"].(string); ok {
						tc.Function.Arguments += argsStr
					}
				}
			}
		}
	}

	var completeCalls []core.ToolCall
	for i := 0; i < len(toolCallsMap); i++ {
		tc, ok := toolCallsMap[i]
		if !ok {
			continue
		}
		if tc.Function.Name == "" || tc.Function.Arguments == "" {
			fmt.Printf("⚠️ [DS] 丢弃不完整的工具调用 index=%d, name=%q, args=%q\n", i, tc.Function.Name, tc.Function.Arguments)
			continue
		}
		completeCalls = append(completeCalls, *tc)
		fmt.Printf("🔧 [DS] 完整工具调用 index=%d, name=%s, args=%s\n", i, tc.Function.Name, tc.Function.Arguments)

		writeSSE(c, "tool_call", "tool_call_start", map[string]string{
			"name": tc.Function.Name,
			"args": tc.Function.Arguments,
		})
		c.Writer.Flush()
	}

	fmt.Printf("🔧 [DS] 完整工具调用数量: %d\n", len(completeCalls))

	var toolResults []string
	if len(completeCalls) > 0 {
		for _, tc := range completeCalls {
			fmt.Printf("⚙️ [DS] 执行工具: %s\n", tc.Function.Name)
			result, err := core.ExecuteToolCall(tc)
			if err != nil {
				fmt.Printf("❌ [DS] 工具执行失败: %v\n", err)
				writeSSE(c, "tool_call", "tool_call_error", map[string]string{
					"name":  tc.Function.Name,
					"error": fmt.Sprintf("%v", err),
				})
				c.Writer.Flush()
				toolResults = append(toolResults, fmt.Sprintf("tool execution error: %v", err))
				continue
			}

			eventType := "tool_call_result"
			if result.Failed {
				eventType = "tool_call_error"
			}

			fmt.Printf("✅ [DS] 工具执行完成，failed=%v, 内容长度=%d\n", result.Failed, len(result.Content))
			writeSSE(c, "tool_call", eventType, map[string]string{
				"name":   tc.Function.Name,
				"result": result.Content,
			})
			c.Writer.Flush()
			toolResults = append(toolResults, result.Content)
		}

		// 保存本轮 assistant tool_calls
		h.sessionStore.Append(sessionID, DSMessage{
			Role:      "assistant",
			Content:   "",
			ToolCalls: completeCalls,
		})

		// 保存 tool 结果
		for i, tr := range toolResults {
			h.sessionStore.Append(sessionID, DSMessage{
				Role:       "tool",
				Content:    tr,
				ToolCallID: completeCalls[i].ID,
			})
		}

		// 关键修复：第二次请求不要重建整套历史，只在当前轮基础上补齐 tool 链
		secondMessages := buildDSSecondRoundMessages(systemPrompt, history, userMessage, completeCalls, toolResults)

		secondReqBody := map[string]interface{}{
			"model":       model,
			"messages":    secondMessages,
			"stream":      true,
			"temperature": temperature,
			"top_p":       topP,
		}
		if maxTokens > 0 {
			secondReqBody["max_tokens"] = maxTokens
		}

		fmt.Println("🔄 [DS] 发送第二次请求（无工具）...")
		return h.sendDSStream(c, secondReqBody, apiKey)
	}

	fmt.Printf("✅ [DS] 纯对话回复长度: %d 字符\n", len(fullContent.String()))
	return fullContent.String(), "", 0, nil
}

// 只构造二次请求所需的最小闭环：system + 旧的普通历史 + 本轮 assistant(tool_calls) + tool results + 总结指令
func buildDSSecondRoundMessages(
	systemPrompt string,
	history []DSMessage,
	userMessage string,
	completeCalls []core.ToolCall,
	toolResults []string,
) []map[string]interface{} {
	msgs := make([]map[string]interface{}, 0, len(history)+6)

	msgs = append(msgs, map[string]interface{}{
		"role":    "system",
		"content": systemPrompt,
	})

	// 只保留普通历史，不回放任何带 tool_calls 的 assistant
	for _, msg := range history {
		switch msg.Role {
		case "user":
			if msg.Content != "" {
				msgs = append(msgs, map[string]interface{}{
					"role":    "user",
					"content": msg.Content,
				})
			}
		case "assistant":
			if msg.Content != "" {
				msgs = append(msgs, map[string]interface{}{
					"role":    "assistant",
					"content": msg.Content,
				})
			}
		}
	}

	// 如果你希望保留当前问题，可以作为普通 user 放在本轮 tool 链之前
	if userMessage != "" {
		msgs = append(msgs, map[string]interface{}{
			"role":    "user",
			"content": userMessage,
		})
	}

	// 本轮 assistant tool_calls
	if len(completeCalls) > 0 {
		assistantToolCalls := make([]map[string]interface{}, 0, len(completeCalls))
		for _, tc := range completeCalls {
			assistantToolCalls = append(assistantToolCalls, map[string]interface{}{
				"id":   tc.ID,
				"type": "function",
				"function": map[string]interface{}{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			})
		}

		msgs = append(msgs, map[string]interface{}{
			"role":       "assistant",
			"content":    "",
			"tool_calls": assistantToolCalls,
		})

		// tool 消息必须逐个紧跟 assistant(tool_calls)，数量必须一致
		for i, tc := range completeCalls {
			result := ""
			if i < len(toolResults) {
				result = toolResults[i]
			}
			if result == "" {
				result = "tool execution error: empty result"
			}
			msgs = append(msgs, map[string]interface{}{
				"role":         "tool",
				"content":      result,
				"tool_call_id": tc.ID,
			})
		}
	}

	// 最后再给总结指令
	msgs = append(msgs, map[string]interface{}{
		"role":    "user",
		"content": "请根据上面的工具执行结果，用自然的语气直接告诉我发生了什么，不要重复工具调用过程，不要重新执行工具。",
	})

	return msgs
}

// sendDSStream 发送流式请求，返回完整文本（同时实时推送 SSE）
func (h *ChatHandler) sendDSStream(c *gin.Context, reqBody map[string]interface{}, apiKey string) (string, string, int, error) {
	body, _ := json.Marshal(reqBody)
	httpReq, _ := http.NewRequest("POST", "https://api.deepseek.com/chat/completions", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", 0, fmt.Errorf("DS请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", 0, fmt.Errorf("DS API 返回 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	reader := bufio.NewReader(resp.Body)
	var fullContent strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", "", 0, err
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		choices, _ := event["choices"].([]interface{})
		if len(choices) == 0 {
			continue
		}
		choice, _ := choices[0].(map[string]interface{})
		delta, _ := choice["delta"].(map[string]interface{})
		if content, ok := delta["content"].(string); ok {
			fullContent.WriteString(content)
			writeSSE(c, "content", "content", map[string]string{"content": content})
			c.Writer.Flush()
		}
	}
	return fullContent.String(), "", 0, nil
}

// dsStreamRequest 仅推送流式内容，不返回文本（用于不需要返回值的场景）
func (h *ChatHandler) dsStreamRequest(c *gin.Context, reqBody map[string]interface{}, apiKey string) error {
	_, _, _, err := h.sendDSStream(c, reqBody, apiKey)
	return err
}

// ---------- Ollama Cloud ----------
// ---------- Ollama Cloud (静默工具调用 · 最终版) ----------
// ---------- Ollama Cloud (静默工具调用 · 最终版) ----------
func (h *ChatHandler) resolveCloudConversation(
	c *gin.Context,
	systemPrompt string,
	userMessage string,
	sessionID string,
	temperature, topP float64,
	maxTokens int,
	reasoningEffort string,
) (string, string, int, error) {
	apiKey := os.Getenv("CLOUD_API_KEY")
	if apiKey == "" {
		return "", "", 0, fmt.Errorf("缺少 Cloud API Key (CLOUD_API_KEY)")
	}
	model := os.Getenv("CLOUD_MODEL")
	if model == "" {
		model = "qwen3-coder:480b-cloud"
	}

	history := h.sessionStore.Get(sessionID)
	history = truncateHistory(history, maxHistoryMessages)
	chatMessages := buildChatMessages(systemPrompt, history, userMessage)

	reqBody := map[string]interface{}{
		"model":    model,
		"messages": chatMessages,
		"stream":   true,
		"options": map[string]interface{}{
			"temperature": temperature,
			"top_p":       topP,
		},
	}
	if maxTokens > 0 {
		reqBody["max_tokens"] = maxTokens
	}

	body, _ := json.Marshal(reqBody)
	httpReq, _ := http.NewRequest("POST", "https://ollama.com/api/chat", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", 0, fmt.Errorf("Cloud请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", 0, fmt.Errorf("Cloud API 返回错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// ★ 第一轮：只缓存，不推前端
	reader := bufio.NewReader(resp.Body)
	var fullContent strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", "", 0, fmt.Errorf("读取Cloud流失败: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			fullContent.WriteString(chunk.Message.Content)
			// 不发任何 SSE
		}
		if chunk.Done {
			break
		}
	}

	finalContent := strings.TrimSpace(fullContent.String())

	// ★ 检测工具调用：静默执行
	if tc, ok := parseToolCallFromText(finalContent); ok {
		resultContent, err := executeToolSilently(sessionID, *tc)
		if err != nil {
			return "", "", 0, err
		}

		// 构造第二次请求消息（arguments 用 map 而不是字符串）
		followUpMessages := []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
			{
				"role":    "assistant",
				"content": "",
				"tool_calls": []map[string]interface{}{
					{
						"id":   "cloud_tool_1",
						"type": "function",
						"function": map[string]interface{}{
							"name":      tc.Tool,
							"arguments": tc.Args, // 直接用 map，不序列化
						},
					},
				},
			},
			{
				"role":         "tool",
				"content":      resultContent,
				"tool_call_id": "cloud_tool_1",
			},
			{
				"role":    "user",
				"content": "请根据上面的工具执行结果，用自然的语气直接告诉我发生了什么。",
			},
		}

		reqBody2 := map[string]interface{}{
			"model":    model,
			"messages": followUpMessages,
			"stream":   true,
			"options": map[string]interface{}{
				"temperature": temperature,
				"top_p":       topP,
			},
		}
		if maxTokens > 0 {
			reqBody2["max_tokens"] = maxTokens
		}

		return h.sendCloudStream(c, reqBody2, apiKey)
	}

	// ★ 非工具调用：逐字推送
	for _, ch := range finalContent {
		writeSSE(c, "content", "content", map[string]string{"content": string(ch)})
		c.Writer.Flush()
		time.Sleep(20 * time.Millisecond)
	}
	return finalContent, "", 0, nil
}

// sendCloudStream 发送第二次请求，实时推送自然语言到前端
func (h *ChatHandler) sendCloudStream(c *gin.Context, reqBody map[string]interface{}, apiKey string) (string, string, int, error) {
	body, _ := json.Marshal(reqBody)
	httpReq, _ := http.NewRequest("POST", "https://ollama.com/api/chat", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", 0, fmt.Errorf("Cloud二次请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", 0, fmt.Errorf("Cloud二次请求返回错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	reader := bufio.NewReader(resp.Body)
	var fullContent strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", "", 0, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var chunk struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Done bool `json:"done"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			fullContent.WriteString(chunk.Message.Content)
			// 这次是最终的自然语言，实时推送
			writeSSE(c, "content", "content", map[string]string{"content": chunk.Message.Content})
			c.Writer.Flush()
		}
		if chunk.Done {
			break
		}
	}

	return fullContent.String(), "", 0, nil
}
