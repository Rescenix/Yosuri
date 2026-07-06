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

	var initialMessages []map[string]interface{}
	for _, m := range chatMessages {
		if m["role"] == "tool" {
			continue
		}
		initialMessages = append(initialMessages, map[string]interface{}{
			"role": m["role"], "content": m["content"],
		})
	}

	// 工具定义
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

	currentMessages := initialMessages

	for {
		// ===== Token 限制 =====
		const maxAllowedTokens = 3000
		estimatedTokens := estimateTokens(currentMessages)
		if estimatedTokens > maxAllowedTokens {
			fmt.Printf("⚠️ [DS] 上下文过长 (估计 %d tokens)，自动截断\n", estimatedTokens)
			currentMessages = truncateMessages(currentMessages, 4)
			estimatedTokens = estimateTokens(currentMessages)
			fmt.Printf("🔧 [DS] 截断后约 %d tokens\n", estimatedTokens)
		}

		reqBody := map[string]interface{}{
			"model":       model,
			"messages":    currentMessages,
			"stream":      true,
			"temperature": temperature,
			"top_p":       topP,
		}
		if maxTokens > 0 && maxTokens <= 2000 {
			reqBody["max_tokens"] = maxTokens
		} else {
			reqBody["max_tokens"] = 2000
		}
		if len(dsTools) > 0 {
			reqBody["tools"] = dsTools
		}

		body, _ := json.Marshal(reqBody)
		httpReq, _ := http.NewRequestWithContext(c.Request.Context(), "POST", "https://api.deepseek.com/chat/completions", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Timeout: 5 * time.Minute}
		resp, err := client.Do(httpReq)
		if err != nil {
			if c.Request.Context().Err() != nil {
				return "", "", 0, c.Request.Context().Err()
			}
			return "", "", 0, fmt.Errorf("DS请求失败: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return "", "", 0, fmt.Errorf("DS API 返回错误 %d: %s", resp.StatusCode, string(bodyBytes))
		}

		reader := bufio.NewReader(resp.Body)
		var fullContent strings.Builder
		toolCallsMap := make(map[int]*core.ToolCall)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				resp.Body.Close()
				return "", "", 0, fmt.Errorf("读取DS流失败: %w", err)
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
		resp.Body.Close()

		var completeCalls []core.ToolCall
		for i := 0; i < len(toolCallsMap); i++ {
			tc, ok := toolCallsMap[i]
			if ok && tc.Function.Name != "" && tc.Function.Arguments != "" {
				completeCalls = append(completeCalls, *tc)
				fmt.Printf("🔧 [DS] 完整工具调用 index=%d, name=%s, args=%s\n", i, tc.Function.Name, tc.Function.Arguments)
				// 将 JSON 字符串参数转为 key="value" 格式
				var argsMap map[string]interface{}
				argsStr := tc.Function.Arguments
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &argsMap); err == nil {
					argsStr = formatToolArgs(argsMap)
				}
				writeSSE(c, "tool_call", "tool_call_start", map[string]string{
					"name": tc.Function.Name,
					"args": argsStr,
				})
				c.Writer.Flush()
			}
		}

		if len(completeCalls) == 0 {
			return fullContent.String(), "", 0, nil
		}

		var toolResults []string
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

		assistantMsg := map[string]interface{}{
			"role":    "assistant",
			"content": fullContent.String(),
		}
		if len(completeCalls) > 0 {
			var dsToolCalls []map[string]interface{}
			for _, tc := range completeCalls {
				dsToolCalls = append(dsToolCalls, map[string]interface{}{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]interface{}{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				})
			}
			assistantMsg["tool_calls"] = dsToolCalls
		}
		currentMessages = append(currentMessages, assistantMsg)

		for i, tc := range completeCalls {
			currentMessages = append(currentMessages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": tc.ID,
				"content":      toolResults[i],
			})
		}
	}
}
