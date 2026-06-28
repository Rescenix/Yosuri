package handler

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var dsNodeProcess *exec.Cmd

// startDSNodeServer 启动 Node.js 代理服务
func (h *ChatHandler) startDSNodeServer() error {
	if dsNodeProcess != nil && dsNodeProcess.Process != nil && dsNodeProcess.ProcessState == nil {
		return nil // 已在运行
	}

	dsNodeProcess = exec.Command("node", "C:\\Pro2026\\re0\\crack\\server.js")
	dsNodeProcess.Stdout = os.Stdout
	dsNodeProcess.Stderr = os.Stderr
	if err := dsNodeProcess.Start(); err != nil {
		return fmt.Errorf("启动 DS 代理失败: %w", err)
	}

	// 等待代理就绪（最多等 30 秒）
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		resp, err := http.Get("http://localhost:3000/ready")
		if err == nil {
			resp.Body.Close()
			return nil
		}
	}
	return fmt.Errorf("DS 代理启动超时")
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

	if err := sendToDSBrowser(userMessage); err != nil {
		return "", "", 0, fmt.Errorf("发送消息到 DS 失败: %w", err)
	}

	// 等待 DS 开始回复（最多等 15 秒）
	ready := false
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		if r, err := checkDSBrowserReady(); err == nil && r {
			ready = true
			break
		}
	}
	if !ready {
		reply, err := readFromDSBrowser()
		if err != nil {
			return "", "", 0, fmt.Errorf("读取 DS 回复失败: %w", err)
		}
		writeSSE(c, "content", "content", map[string]string{"content": reply})
		c.Writer.Flush()
		return reply, "", 0, nil
	}

	// 轮询 /read，流式推送给前端
	var fullContent string
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
	// 保存到 PrismD（Atri 域）
	engramBody := fmt.Sprintf("ENGRAM Atri域 %s", fullContent)
	go func() {
		resp, err := http.Post("http://localhost:5666", "text/plain", strings.NewReader(engramBody))
		if err != nil {
			fmt.Printf("[Atri] 写入 PrismD 失败: %v\n", err)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("[Atri] 已写入 PrismD: %s\n", string(body))
	}()
	return fullContent, "", 0, nil
}
