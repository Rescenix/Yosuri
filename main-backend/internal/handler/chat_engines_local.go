// chat_engines_local.go — 简洁版（无调试日志）
package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxOllamaLineSize = 64 * 1024

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
	model := "qwen2.5-coder:7b"
	ollamaURL := "http://localhost:11434/api/chat"
	client := &http.Client{Timeout: 5 * time.Minute}

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
	body, _ := json.Marshal(reqBody)

	httpReq, _ := http.NewRequest("POST", ollamaURL, bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", 0, fmt.Errorf("本地模型请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", 0, fmt.Errorf("本地模型返回错误 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	br := bufio.NewReaderSize(resp.Body, maxOllamaLineSize)
	var fullText strings.Builder

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if len(line) > 0 {
					processLine(line, &fullText, c)
				}
				break
			}
			return "", "", 0, fmt.Errorf("读取本地流失败: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		processLine(line, &fullText, c)
	}

	fullContent := fullText.String()

	// 工具调用检测（仅 depth == 0 时）
	if depth == 0 {
		if tc, ok := parseToolCallFromText(fullContent); ok {
			writeSSE(c, "tool_call", "tool_call_start", map[string]string{
				"name": tc.Tool,
				"args": fmt.Sprintf("%v", tc.Args),
			})
			c.Writer.Flush()

			resultContent, err := h.executeToolAndNotify(c, sessionID, fullContent, *tc)
			if err != nil {
				return "", "", 0, err
			}
			followUpMsg := fmt.Sprintf(
				"你的上一个操作（%s）已经完成，结果是：%s。请用自然的语气告诉朋友发生了什么。",
				tc.Tool, resultContent,
			)
			return h.resolveConversation(
				c, systemPrompt, followUpMsg, sessionID,
				temperature, topP, maxTokens, reasoningEffort,
				depth+1, "local", "", "",
			)
		}
	}

	return "", "", 0, nil
}

func processLine(line string, fullText *strings.Builder, c *gin.Context) {
	var chunk struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Done bool `json:"done"`
	}
	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		return
	}
	if chunk.Message.Content != "" {
		fullText.WriteString(chunk.Message.Content)
		writeSSE(c, "content", "content", map[string]string{"content": chunk.Message.Content})
		c.Writer.Flush()
	}
}
