package handler

// aggregate_responses.go —— 聚合 API 对外 /v1/responses 端点（2026-08-26 新增）。
//
// 新版 OpenAI 生态客户端（Claude Code / Codex / 各 agent 框架）默认优先走
// /v1/responses 协议。本文件把 Responses 协议请求翻译成内部 chat/completions
// 处理，再翻译回 Responses 协议响应。路由内核（auto 链 / 熔断 / 额度 / failover）
// 完全复用现有链路。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// aggregateResponsesRequest OpenAI /v1/responses 请求体。
type aggregateResponsesRequest struct {
	Model           string           `json:"model"`
	Input           json.RawMessage  `json:"input"` // 字符串 或 items 数组
	Instructions    string           `json:"instructions"`
	Stream          bool             `json:"stream"`
	Tools           []map[string]any `json:"tools"`
	MaxOutputTokens int              `json:"max_output_tokens"`
	Temperature     float64          `json:"temperature"`
}

// HandleAggregateResponses POST /v1/responses —— 翻译 Responses 协议到 chat/completions。
func HandleAggregateResponses(c *gin.Context) {
	if !aggregateAuth(c) {
		return
	}
	rawBody, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewReader(rawBody))
	var req aggregateResponsesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	msgs, err := responsesInputToMessages(req.Input, req.Instructions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(msgs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "input 不能为空"})
		return
	}
	tools := responsesToChatTools(req.Tools)

	chain := modelToAggregateBackends(req.Model)
	if len(chain) == 0 {
		if req.Model != "" && req.Model != "auto" && req.Model != "rescene-auto" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("模型 %q 未找到（精确模型禁止自动回退，请检查模型名或改用 auto）", req.Model),
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "没有可用的免费模型（未配置任何 key）"})
		return
	}

	upstream := map[string]any{
		"model":    "", // 逐 backend 填充
		"messages": msgs,
		"stream":   req.Stream,
	}
	if req.Temperature == 0 {
		upstream["temperature"] = 0.2
	} else {
		upstream["temperature"] = req.Temperature
	}
	if req.MaxOutputTokens > 0 {
		upstream["max_tokens"] = req.MaxOutputTokens
	}
	if len(tools) > 0 {
		upstream["tools"] = tools
	}

	var lastErr error
	tried := []string{}
	for i := range chain {
		b := chain[i]
		tried = append(tried, b.Model)
		upstream["model"] = b.Model
		if req.Stream {
			resp, err := aggregateStreamOnce(c.Request.Context(), b, upstream)
			if err != nil {
				circuitFail(b)
				lastErr = err
				continue
			}
			aggStatsInc(b, estimateJSONTokens(rawBody), "")
			aggregateForwardResponsesSSE(c, b, resp)
			return
		}
		content, calls, err := openAIChatOnce(c.Request.Context(), b, msgs, tools)
		if err != nil {
			lastErr = err
			continue
		}
		aggStatsInc(b, estimateJSONTokens(rawBody), "")
		c.JSON(http.StatusOK, buildAggregateResponses(b, content, calls))
		return
	}
	if len(tried) <= 1 {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("模型 %s 请求失败: %s", tried[0], lastErr)})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{
		"error": fmt.Sprintf("所有免费模型均失败（已尝试: %s）: %s", strings.Join(tried, " → "), lastErr),
	})
}