package handler

// translate.go —— 文章翻译 API（2026-08-06）
// POST /api/translate
// 请求: {text: "中文文章", target_lang: "en"/"ja"/"ko"}
// 响应: {ok: true, translated: "英文文章"}

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type translateRequest struct {
	Text       string `json:"text"`
	TargetLang string `json:"target_lang"`
}

// HandleTranslate 翻译一篇文章到目标语言
func HandleTranslate(c *gin.Context) {
	var req translateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "解析失败: " + err.Error()})
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text 不能为空"})
		return
	}

	langName := map[string]string{
		"en": "English",
		"ja": "Japanese",
		"ko": "Korean",
	}[req.TargetLang]
	if langName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_lang 仅支持 en/ja/ko"})
		return
	}

	// 复用 LLM 路由：优先 step credit，回落免费池
	b := resolveExact("", "plan_step_gateway")
	if b == nil {
		b = resolveExact("", "free_step_3_7_flash")
	}
	if b == nil {
		backends := resolveBackends("", "")
		if len(backends) == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "没有可用模型"})
			return
		}
		bb := backends[0]
		b = &bb
	}

	prompt := fmt.Sprintf(`请将以下中文文章翻译成%s。要求：
1. 保持原文的段落结构和格式（包括标题、列表、引用）
2. 专业术语和技术名词保留原样或给出准确翻译
3. 不要添加原文没有的解释或评论
4. 只输出翻译结果，不要任何前缀说明

文章内容：
%s`, langName, req.Text)

	msgs := []map[string]any{{"role": "user", "content": prompt}}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	content, _, err := openAIChatOnce(ctx, *b, msgs, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "翻译失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":          true,
		"translated":  strings.TrimSpace(content),
		"target_lang": req.TargetLang,
	})
}