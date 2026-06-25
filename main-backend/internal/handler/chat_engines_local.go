// chat_engines_local.go - 诊断版（定位无回复根因）
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
	fmt.Printf("📡 [本地] 开始调用 Ollama，depth=%d\n", depth)

	model := "qwen2.5-coder:7b"
	ollamaURL := "http://localhost:11434/api/chat"
	client := &http.Client{Timeout: 5 * time.Minute}

	history := h.sessionStore.Get(sessionID)
	history = truncateHistory(history, maxHistoryMessages)
	chatMessages := buildChatMessages(systemPrompt, history, userMessage)

	// ★ 诊断日志1：完整打印发送给模型的消息（截断显示）
	fmt.Printf("🔍 [诊断] 发送消息数量: %d\n", len(chatMessages))
	for i, msg := range chatMessages {
		role := msg["role"]
		content := msg["content"]
		// 只显示前80字符
		displayContent := content
		if len(displayContent) > 80 {
			displayContent = displayContent[:80] + "..."
		}
		fmt.Printf("  [%d] %s: %s\n", i, role, displayContent)
	}

	// 如果消息列表只有 system 且没有 user，直接告警
	if len(chatMessages) == 1 && chatMessages[0]["role"] == "system" {
		fmt.Println("⚠️ [警告] 仅包含 system 消息，模型可能不生成内容！")
	}

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
	fmt.Println("✅ [本地] 流式连接建立，开始读取...")

	lineCount := 0
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if len(line) > 0 {
					processLineWithLog(line, &fullText, c, &lineCount)
				}
				break
			}
			return "", "", 0, fmt.Errorf("读取本地流失败: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		processLineWithLog(line, &fullText, c, &lineCount)
	}

	fullContent := fullText.String()
	fmt.Printf("📦 [本地] 完整输出: %s\n", fullContent)
	fmt.Printf("📊 [统计] 共接收 %d 行, 累计内容长度: %d\n", lineCount, len(fullContent))

	// 检测工具调用
	if depth == 0 {
		if tc, ok := parseToolCallFromText(fullContent); ok {
			fmt.Printf("🔧 [本地] 检测到工具调用: %s, 参数: %v\n", tc.Tool, tc.Args)
			writeSSE(c, "tool_call", "tool_call_start", map[string]string{
				"name": tc.Tool,
				"args": fmt.Sprintf("%v", tc.Args),
			})
			c.Writer.Flush()

			resultContent, err := h.executeToolAndNotify(c, sessionID, fullContent, *tc)
			if err != nil {
				fmt.Printf("❌ [本地] 工具执行失败: %v\n", err)
			} else {
				fmt.Printf("✅ [本地] 工具执行成功，结果长度 %d\n", len(resultContent))
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
	}

	return "", "", 0, nil
}

// processLineWithLog 解析行并推送，同时输出诊断
func processLineWithLog(line string, fullText *strings.Builder, c *gin.Context, lineCount *int) {
	*lineCount++
	var chunk struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Done bool `json:"done"`
	}
	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		// 诊断日志2：打印解析失败的行（仅前120字符）
		if len(line) > 120 {
			fmt.Printf("⚠️ [解析失败] %s...\n", line[:120])
		} else {
			fmt.Printf("⚠️ [解析失败] %s\n", line)
		}
		return
	}
	if chunk.Message.Content != "" {
		fullText.WriteString(chunk.Message.Content)
		writeSSE(c, "content", "content", map[string]string{"content": chunk.Message.Content})
		c.Writer.Flush()
		// 诊断日志3：成功推送的内容片段
		fmt.Printf("📤 [推送] %s\n", chunk.Message.Content)
	}
	if chunk.Done {
		fmt.Println("✅ [本地] 流结束（done=true）")
	}
}
