package handler

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"backend/internal/swiftnet"

	"github.com/gin-gonic/gin"
)

var dsNodeProcess *exec.Cmd

// startDSNodeServer 启动 Node.js 代理服务
func (h *ChatHandler) startDSNodeServer() error {
	return EnsureDSNodeServer()
}

// sendToDSBrowser 发送消息到 DS 浏览器代理
func sendToDSBrowser(message string) error {
	body := fmt.Sprintf(`{"message":"%s"}`, message)
	resp, err := http.Post("http://localhost:3000/send", "application/json", strings.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// readFromDSBrowser 从 DS 浏览器代理读取最新回复（Markdown 格式）
func readFromDSBrowser() (string, error) {
	resp, err := http.Get("http://localhost:3000/read")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return string(data), nil
}

// checkDSBrowserReady 检查 DS 浏览器代理是否有新回复
func checkDSBrowserReady() (bool, error) {
	resp, err := http.Get("http://localhost:3000/ready")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return string(data) == "yes", nil
}

// resolveDSBrowserConversation 通过浏览器获取 DS 官网的免费回复，并以 SSE 流式推送给前端
func (h *ChatHandler) resolveDSBrowserConversation(
	c *gin.Context,
	systemPrompt string,
	userMessage string,
	sessionID string,
) (string, string, int, error) {

	if err := h.startDSNodeServer(); err != nil {
		return "", "", 0, fmt.Errorf("DS 代理启动失败: %w", err)
	}

	// ----- 新增：从 sessionStore 获取历史消息 -----
	history := h.sessionStore.Get(sessionID)
	// 截断过长的历史，保持上下文精简
	history = truncateHistory(history, maxHistoryMessages)

	// ----- 新增：构建包含历史的完整消息发送给 DS -----
	var contextBuilder strings.Builder
	for _, msg := range history {
		contextBuilder.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content))
	}
	contextBuilder.WriteString(fmt.Sprintf("[user]: %s", userMessage))
	fullMessage := contextBuilder.String()

	if err := sendToDSBrowser(fullMessage); err != nil {
		return "", "", 0, fmt.Errorf("发送消息到 DS 失败: %w", err)
	}
	ready := false
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		if r, err := checkDSBrowserReady(); err == nil && r {
			ready = true
			break
		}
	}

	var fullContent string
	if !ready {
		reply, err := readFromDSBrowser()
		if err != nil {
			return "", "", 0, fmt.Errorf("读取 DS 回复失败: %w", err)
		}
		fullContent = reply
		writeSSE(c, "content", "content", map[string]string{"content": reply})
		c.Writer.Flush()
	} else {
		// 轮询 /read，流式推送给前端
		lastLength := 0
		stableCount := 0

		for i := 0; i < 150; i++ {
			time.Sleep(200 * time.Millisecond)
			reply, err := readFromDSBrowser()
			if err != nil {
				continue
			}
			if len(reply) > lastLength {
				newPart := reply[lastLength:]
				writeSSE(c, "content", "content", map[string]string{"content": newPart})
				c.Writer.Flush()
				lastLength = len(reply)
				fullContent = reply
				stableCount = 0
			} else if len(reply) == lastLength && len(reply) > 10 {
				stableCount++
				if stableCount >= 5 {
					break
				}
			}
		}

		if fullContent == "" {
			reply, err := readFromDSBrowser()
			if err == nil {
				fullContent = reply
			}
		}
	}

	// ----- 新增：将对话保存到 sessionStore -----
	now := time.Now()
	h.sessionStore.Append(sessionID, DSMessage{
		Role:      "user",
		Content:   userMessage,
		Timestamp: now,
	})
	h.sessionStore.Append(sessionID, DSMessage{
		Role:      "assistant",
		Content:   fullContent,
		Timestamp: now,
		Model:     "ds_browser",
	})

	// 保存到 SwiftNet 事实库（写侧防重会自动合并同义内容）
	go func() {
		res := swiftnet.Default().MemAppend(fullContent, "Atri", "")
		if res.Err != "" {
			fmt.Printf("[Atri] 写入 SwiftNet 失败: %s\n", res.Err)
			return
		}
		fmt.Printf("[Atri] 已写入 SwiftNet: %s%s\n", res.ID, res.MergedID)
	}()
	return fullContent, "", 0, nil
}
