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

const maxHistoryMessages = 10

// ---------- Token 工具 ----------
func estimateTokens(messages []map[string]interface{}) int {
	totalChars := 0
	for _, m := range messages {
		for _, v := range m {
			if s, ok := v.(string); ok {
				totalChars += len(s)
			} else if arr, ok := v.([]interface{}); ok {
				for _, item := range arr {
					if s, ok := item.(string); ok {
						totalChars += len(s)
					} else if m2, ok := item.(map[string]interface{}); ok {
						for _, v2 := range m2 {
							if s2, ok := v2.(string); ok {
								totalChars += len(s2)
							}
						}
					}
				}
			}
		}
	}
	return totalChars / 4
}

func truncateMessages(messages []map[string]interface{}, keepNonSystem int) []map[string]interface{} {
	var cleaned []map[string]interface{}
	for _, m := range messages {
		if m["role"] == "system" {
			cleaned = append(cleaned, m)
		}
	}
	startIdx := len(messages) - keepNonSystem
	if startIdx < 0 {
		startIdx = 0
	}
	// 截断窗口不能以 "tool" 消息开头——那意味着丢弃了它对应的、带 tool_calls
	// 的 assistant 消息，DS API 会因此拒绝整个请求（400: tool 消息必须紧跟在
	// 带 tool_calls 的 assistant 消息后面）。往前找到那条 assistant 消息，一并带上。
	for startIdx > 0 && messages[startIdx]["role"] == "tool" {
		startIdx--
	}
	for i := startIdx; i < len(messages); i++ {
		if messages[i]["role"] != "system" {
			cleaned = append(cleaned, messages[i])
		}
	}
	return cleaned
}

// ---------- 流式请求辅助 ----------
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

func (h *ChatHandler) dsStreamRequest(c *gin.Context, reqBody map[string]interface{}, apiKey string) error {
	_, _, _, err := h.sendDSStream(c, reqBody, apiKey)
	return err
}

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
	var currentLine strings.Builder
	lineIsJSON := false

	// 按字符扫描一个 delta：一旦当前行（去掉行首空白后）以 { 开头，就判定这一行是
	// 工具调用 JSON——从这个字符起不再转发 content 事件，避免 JSON 在拼完整之前就
	// 裸露闪现在聊天记录里；JSON 前面那句自然语言（如果模型有说）照常实时流式转发。
	// fullContent 始终完整累积（不受这个判断影响），后面解析工具调用要用完整文本。
	emitDelta := func(delta string) {
		fullContent.WriteString(delta)
		if lineIsJSON {
			return
		}
		var safe strings.Builder
		for _, ch := range delta {
			if ch == '\n' {
				currentLine.Reset()
				safe.WriteRune(ch)
				continue
			}
			currentLine.WriteRune(ch)
			trimmed := strings.TrimLeft(currentLine.String(), " \t")
			if len(trimmed) > 0 && trimmed[0] == '{' {
				lineIsJSON = true
				break
			}
			safe.WriteRune(ch)
		}
		if s := safe.String(); s != "" {
			writeSSE(c, "content", "content", map[string]string{"content": s})
			c.Writer.Flush()
		}
	}

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
			emitDelta(chunk.Message.Content)
		}
		if chunk.Done {
			break
		}
	}
	return fullContent.String(), "", 0, nil
}
