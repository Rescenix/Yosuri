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
// {topic} 主题 → 生成小说/文章（标题+简介+章节）；{chapter} 续写章节号
func HandleAIWrite(c *gin.Context) {
	var req struct {
		Topic   string `json:"topic" binding:"required"`
		Type    string `json:"type"` // novel=小说（默认）| article=文章
		Chapter int    `json:"chapter"` // 续写：0=第一章，N=第N+1章
		Title   string `json:"title"`   // 续写时的小说标题
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

	kind := req.Type
	if kind != "article" {
		kind = "novel"
	}

	var prompt string
	if kind == "novel" {
		if req.Chapter > 0 {
			cn := req.Chapter + 1
			prompt = fmt.Sprintf(`你是一家 AI 小说公司里的作者。继续写《%s》的第%d章。

已知设定：%s

要求：承接前文，800-1500 字，有场景、人物、悬念推进，结尾留钩子。直接输出第%d章正文。`, req.Title, cn, topic, cn)
		} else {
			prompt = fmt.Sprintf(`你是一家 AI 小说公司里的主编。围绕「%s」创作一部小说的第一章。

输出格式（严格）：
标题：<小说标题>
简介：<80-150 字简介，吸引人>
第一章：<800-1500 字的章节正文，有场景、人物、悬念>

要求：真实有吸引力的网文风格，不是 AI 腔，有画面感。`, topic)
		}
	} else {
		prompt = fmt.Sprintf(`你是一位住在 AI 公司里的全能作者。围绕「%s」写一篇 800-1500 字的完整文章。

要求：
- 有吸引人的标题（第一行）
- 有真实的内容与观点，不是空话
- 用中文，有温度，不像 AI 腔
- 直接输出正文，标题用 # 开头

写吧。`, topic)
	}

	content, err := callLocalAggregate(prompt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "模型暂不可用（免费额度限流），请稍后重试", "topic": topic})
		return
	}

	if kind == "novel" {
		if req.Chapter > 0 {
			cn := req.Chapter + 1
			title := req.Title
			if title == "" {
				title = topic
			}
			c.JSON(http.StatusOK, gin.H{
				"type": "novel", "title": title, "chapterNo": cn, "chapter": content, "content": content,
			})
			return
		}
		title, summary, chapter := parseNovel(content)
		c.JSON(http.StatusOK, gin.H{
			"type": "novel", "title": title, "summary": summary, "chapter": chapter, "chapterNo": 1, "content": content,
		})
		return
	}
	title := topic
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			title = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
			break
		}
	}
	c.JSON(http.StatusOK, gin.H{"type": "article", "title": title, "content": content})
}

// parseNovel 解析小说输出（标题/简介/第一章）
func parseNovel(content string) (title, summary, chapter string) {
	lines := strings.Split(content, "\n")
	title = "未命名"
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "标题") || strings.HasPrefix(t, "书名") {
			title = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(t, "标题"), "书名"))
			title = strings.Trim(title, "：:")
			break
		}
		if strings.HasPrefix(t, "#") && title == "未命名" {
			title = strings.TrimSpace(strings.TrimPrefix(t, "#"))
		}
		_ = i
	}
	var sb strings.Builder
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "简介") {
			summary = strings.TrimSpace(strings.TrimPrefix(t, "简介"))
			summary = strings.Trim(summary, "：:")
			continue
		}
		if strings.HasPrefix(t, "第一章") || strings.HasPrefix(t, "第 1 章") {
			chapter += t + "\n"
			continue
		}
		if summary != "" && chapter != "" {
			chapter += l + "\n"
		}
	}
	if chapter == "" {
		chapter = content
	}
	_ = sb
	return title, summary, strings.TrimSpace(chapter)
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
	req, _ := http.NewRequest("POST", "http://127.0.0.1:8080/v1/chat/completions", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+aggregateAPIKey()) // 聚合 API 鉴权（之前漏了 → 一直 401）
	resp, err := client.Do(req)
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