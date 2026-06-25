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

	"github.com/gin-gonic/gin"
)

// chat_engines_cloud.go 最终版：支持多轮工具调用
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

	// 转换为通用的消息格式，便于多轮追加
	var currentMessages []map[string]interface{}
	for _, m := range chatMessages {
		currentMessages = append(currentMessages, map[string]interface{}{
			"role": m["role"], "content": m["content"],
		})
	}

	// 多轮工具调用循环
	for {
		reqBody := map[string]interface{}{
			"model":    model,
			"messages": currentMessages,
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

		// 流式读取并实时推送
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
				writeSSE(c, "content", "content", map[string]string{"content": chunk.Message.Content})
				c.Writer.Flush()
			}
			if chunk.Done {
				break
			}
		}

		finalContent := strings.TrimSpace(fullContent.String())

		// 检测工具调用
		if tc, ok := parseToolCallFromText(finalContent); ok {
			fmt.Printf("🔧 [Cloud] 检测到工具调用: %s\n", tc.Tool)

			// 发送工具调用开始事件
			writeSSE(c, "tool_call", "tool_call_start", map[string]string{
				"name": tc.Tool,
				"args": fmt.Sprintf("%v", tc.Args),
			})
			c.Writer.Flush()

			// 执行工具
			resultContent, err := executeToolSilently(sessionID, *tc)
			if err != nil {
				writeSSE(c, "tool_call", "tool_call_error", map[string]string{
					"name":  tc.Tool,
					"error": fmt.Sprintf("%v", err),
				})
				c.Writer.Flush()
				// 即使失败也追加结果，让模型知道并继续
				resultContent = fmt.Sprintf("工具执行失败: %v", err)
			}

			// 发送工具调用结果事件
			writeSSE(c, "tool_call", "tool_call_result", map[string]string{
				"name":   tc.Tool,
				"result": resultContent,
			})
			c.Writer.Flush()

			// 将本次的 assistant 消息和 tool 结果追加到消息历史
			currentMessages = append(currentMessages, map[string]interface{}{
				"role": "assistant", "content": finalContent,
			})
			currentMessages = append(currentMessages, map[string]interface{}{
				"role": "tool", "content": resultContent,
			})

			// 继续循环，让模型基于结果决定是否继续调用工具
			continue
		}

		// 不是工具调用，返回空，内容已在流式推送完毕
		return "", "", 0, nil
	}
}
