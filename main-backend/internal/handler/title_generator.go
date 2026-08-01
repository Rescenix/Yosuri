package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// generateTitleWithModel 按前端传来的模型 ID 解析 backend 并生成简短中文标题。
// model 为空或解析失败时走路由全链兜底（用户配置 > 免费池），与主对话口径一致。
// 输入用户第一条消息的纯文本；返回清洗后的标题。失败返回空串，由调用方静默放弃。
func generateTitleWithModel(ctx context.Context, model, userText string) string {
	userText = strings.TrimSpace(userText)
	if userText == "" {
		return ""
	}
	b := resolveExact("", model)
	if b == nil {
		backends := resolveBackends("", "")
		if len(backends) == 0 {
			return ""
		}
		b = &backends[0]
	}
	return generateTitleWithBackend(ctx, b, userText)
}

// generateTitleWithBackend 用指定的 backend（主对话实际回复用的那个）为会话
// 生成简短中文标题。输入用户第一条消息的纯文本；返回清洗后的标题。
//
// 任何一步失败都返回空串（调用方静默放弃，保持"新对话"默认标题，绝不回退原文）：
// 标题生成是旁路任务，绝不能因为它的失败让会话出现一个假的标题。
func generateTitleWithBackend(ctx context.Context, b *RouterBackend, userText string) string {
	userText = strings.TrimSpace(userText)
	if userText == "" || b == nil {
		return ""
	}
	prompt := fmt.Sprintf(`你是会话标题生成器。根据用户的第一条消息，生成一个简洁的中文会话标题，概括对话主题。
要求：
- 2-8 个汉字，像「友好的问候」「Python 学习计划」这样
- 不要引号、不要书名号、不要句末标点
- 不要「标题：」之类的前缀
- 直接输出标题本身，不要任何解释

用户消息：%s`, userText)

	msgs := []map[string]any{{"role": "user", "content": prompt}}
	// 标题生成是旁路任务：超时 15s 就放弃，不拖住流式渲染
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	content, _, err := openAIChatOnce(ctx, *b, msgs, nil)
	if err != nil {
		return ""
	}
	return cleanTitle(content)
}

// HandleGenerateTitle POST /api/title/generate
// body: { text: 用户第一条消息纯文本, model: 前端下拉框当前选中的模型 ID }
// 返回: { title: AI 生成的标题，失败时为空串 }
func HandleGenerateTitle(c *gin.Context) {
	var body struct {
		Text  string `json:"text"`
		Model string `json:"model"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	title := generateTitleWithModel(c.Request.Context(), body.Model, body.Text)
	c.JSON(200, gin.H{"title": title})
}

// cleanTitle 清洗模型生成的标题：去引号/书名号/前缀/标点，截断到 16 字。
func cleanTitle(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "\"'“”‘’《》「」『』【】()（）")
	raw = strings.TrimSpace(raw)
	// 去掉「标题：」「标题是」之类前缀（注意全角冒号是多字节，按字节长度切）
	if idx := strings.Index(raw, "："); idx != -1 && idx <= 6 {
		raw = strings.TrimSpace(raw[idx+len("："):])
	} else if idx := strings.Index(raw, ":"); idx != -1 && idx <= 6 {
		raw = strings.TrimSpace(raw[idx+1:])
	}
	raw = strings.Trim(raw, "\"'“”‘’《》「」『』【】()（）")
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, "。.!！?？~～…")
	runes := []rune(raw)
	if len(runes) > 16 {
		runes = runes[:16]
	}
	return strings.TrimSpace(string(runes))
}
