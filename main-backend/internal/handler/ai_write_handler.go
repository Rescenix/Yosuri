package handler

// ai_write_handler.go — AI 写作工坊（震撼小红书：输入主题 → AI 生成完整文章）
//   POST /api/ai/write {topic} → 聚合免费模型生成 800-1500 字文章 → {title, content}

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// HandleAIWrite POST /api/ai/write
func HandleAIWrite(c *gin.Context) {
	var req struct {
		Topic string `json:"topic" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入主题"})
		return
	}
	topic := strings.TrimSpace(req.Topic)
	if len([]rune(topic)) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "主题太短"})
		return
	}

	prompt := fmt.Sprintf(`你是一位住在 AI 公司里的全能作者。围绕「%s」写一篇 800-1500 字的完整文章。

要求：
- 有吸引人的标题（第一行）
- 有真实的内容与观点，不是空话
- 用中文，有温度，不像 AI 腔
- 直接输出正文，标题用 # 开头

写吧。`, topic)

	// 调本地聚合 API 生成（复用免费模型池）
	content, err := callLocalAggregate(prompt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "模型暂不可用（免费额度限流），请稍后重试", "topic": topic})
		return
	}

	// 解析标题（第一个 # 行）
	title := topic
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			title = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
			break
		}
	}
	c.JSON(http.StatusOK, gin.H{"title": title, "content": content})
}

// callLocalAggregate 调本地 /v1/chat/completions（聚合免费模型池）
func callLocalAggregate(prompt string) (string, error) {
	body := map[string]any{
		"model": "auto",
		"messages": []map[string]any{
			{"role": "system", "content": "你是 Rescene AI 公司的作者，写真实有温度的中文内容。"},
			{"role": "user", "content": prompt},
		},
		"max_tokens":   2048,
		"temperature":  0.8,
		"stream":       false,
	}
	reqBytes, _ := json.Marshal(body)

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Post("http://127.0.0.1:8080/v1/chat/completions", "application/json", bytes.NewReader(reqBytes))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("聚合 API HTTP %d", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("空响应")
	}
	return out.Choices[0].Message.Content, nil
}