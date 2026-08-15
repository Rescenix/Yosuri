package handler

// translate.go —— 文章翻译 API（2026-08-06）
// POST /api/translate           单条翻译
// POST /api/translate/batch     批量翻译（前端一次拉全部插件描述，缓存本地）
// 请求: {texts|text, target_lang: "zh"/"en"/"ja"/"ko"}（源语言自动识别，双向）
// 响应: {ok: true, translated|results: ...}
//
// 模型策略（08-15 实锤）：免费模型 RPM 极低（sensenova 翻 2 条就 rpm exhausted），
// 单个模型必然 429 → 多模型降级链（sensenova → kilo → zen → modelscope → 免费池兜底），
// 遇限流自动换下一个；批量接口内部每条间隔 300ms 限速，避免打爆 RPM。

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type translateRequest struct {
	Text       string `json:"text"`
	TargetLang string `json:"target_lang"`
}

type translateBatchRequest struct {
	Texts      []string `json:"texts"`
	TargetLang string   `json:"target_lang"`
}

var translateLangNames = map[string]string{
	"en": "English",
	"ja": "Japanese",
	"ko": "Korean",
	"zh": "简体中文",
}

// translateModelChain 多模型降级链：免费模型 RPM 极低，遇限流换下一个。
var translateModelChain = []string{
	"free_sensenova_deepseek_v4_flash",
	"kilo_auto_free",
	"free_zen_deepseek_v4_flash",
	"free_modelscope_qwen2_5_vl",
}

// translateOne 用多模型降级链翻译单条文本，返回 (翻译结果, 实际模型, 错误)。
func translateOne(ctx context.Context, text, langName string) (string, string, error) {
	prompt := fmt.Sprintf(`请将以下内容翻译成%s（自动识别源语言）。要求：
1. 保持原文的段落结构和格式（包括标题、列表、引用）
2. 专业术语和技术名词保留原样或给出准确翻译
3. 不要添加原文没有的解释或评论
4. 只输出翻译结果，不要任何前缀说明

内容：
%s`, langName, text)

	msgs := []map[string]any{{"role": "user", "content": prompt}}
	var lastErr error
	for _, modelID := range translateModelChain {
		b := resolveExact("", modelID)
		if b == nil {
			continue
		}
		// 单模型硬超时 12s：免费模型挂起时立即降级，避免整条链等 90s
		modelCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
		content, _, err := openAIChatOnce(modelCtx, *b, msgs, nil)
		cancel()
		if err == nil && strings.TrimSpace(content) != "" {
			return strings.TrimSpace(content), modelID, nil
		}
		lastErr = err
	}
	// 全链失败：回落任意免费池兜底
	backends := resolveBackends("", "")
	if len(backends) > 0 {
		bctx, cancel := context.WithTimeout(ctx, 12*time.Second)
		content, _, err := openAIChatOnce(bctx, backends[0], msgs, nil)
		cancel()
		if err == nil && strings.TrimSpace(content) != "" {
			return strings.TrimSpace(content), "free-pool-fallback", nil
		} else {
			lastErr = err
		}
	}
	return "", "", lastErr
}

// HandleTranslate 单条翻译
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
	langName := translateLangNames[req.TargetLang]
	if langName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_lang 仅支持 zh/en/ja/ko"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	translated, model, err := translateOne(ctx, req.Text, langName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "翻译失败（多模型均已限流/不可用）: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":          true,
		"translated":  translated,
		"model":       model,
		"target_lang": req.TargetLang,
	})
}

// HandleTranslateBatch 批量翻译：前端一次拉全部插件描述，翻译完缓存本地。
// 串行翻译（每条间隔 300ms 限速），失败条目留空（前端回退原文）。
func HandleTranslateBatch(c *gin.Context) {
	var req translateBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "解析失败: " + err.Error()})
		return
	}
	if len(req.Texts) == 0 || len(req.Texts) > 60 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "texts 需为 1~60 条"})
		return
	}
	langName := translateLangNames[req.TargetLang]
	if langName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_lang 仅支持 zh/en/ja/ko"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 150*time.Second)
	defer cancel()

	results := make([]string, len(req.Texts))
	okCount := 0
	// 并发 3（semaphore）：翻译快 3 倍，且模型链各自降级不互相挤占
	sem := make(chan struct{}, 3)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, text := range req.Texts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		wg.Add(1)
		go func(idx int, t string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// 每条间隔限速：免费模型 RPM 极低，批量也别打爆
			select {
			case <-time.After(250 * time.Millisecond):
			case <-ctx.Done():
				return
			}
			translated, _, err := translateOne(ctx, t, langName)
			if err == nil && translated != "" {
				mu.Lock()
				results[idx] = translated
				okCount++
				mu.Unlock()
			}
		}(i, text)
	}
	wg.Wait()

	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"results":  results,
		"ok_count": okCount,
		"total":    len(results),
	})
}
