package handler

// aggregate_api.go —— Rescene 聚合 API（2026-08-02）。
//
// 把用户在前端填的所有 API key（免费模型池 + 自定义提供方）聚合成
// 一个 OpenAI 兼容端点：外部工具（Claude Code / Cursor / Codex / Gemini CLI
// 等支持 OpenAI 兼容配置的客户端）填 base_url=http://localhost:8080/v1 +
// 一个 Bearer key，就能用上全部免费模型，路由由 Auto 链接管
// （探活信号 + LRU 新鲜度 + 熔断 + 秒切 failover）。
//
//   - POST /v1/chat/completions —— OpenAI 兼容聊天（非流式 + 流式 SSE）
//   - GET  /v1/models —— 列出可用模型（免费池 + 自定义提供方）
//
// 鉴权：Bearer key == env RESCENE_AGG_API_KEY，未设置时用默认 sk-rescene-local
// （文档写清楚，用户可改）。只监听本机端口（后端默认绑定 localhost 场景）。
//
// 模型映射（model 字段）：
//   - 空 / "auto" / "rescene-auto" → Auto 全链（信号 + LRU 排序）
//   - 免费池 ID（free_xxx）/ 自定义配置 ID → 精确路由
//   - 其他字符串 → 按 Model 名 / 名称模糊匹配目录，找不到回退 Auto 全链
//
// 二期预留：Anthropic（/v1/messages）与 Gemini 原生协议适配层，共用同一路由内核。

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// aggregateDefaultKey 默认聚合 key；可通过 RESCENE_AGG_API_KEY 环境变量覆盖。
const aggregateDefaultKey = "sk-rescene-local"

func aggregateAPIKey() string {
	if k := os.Getenv("RESCENE_AGG_API_KEY"); k != "" {
		return k
	}
	return aggregateDefaultKey
}

// aggregateAuth 校验 Bearer key。返回 false 时已写好 401 响应。
func aggregateAuth(c *gin.Context) bool {
	auth := c.GetHeader("Authorization")
	if auth == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少 Authorization: Bearer <key>"})
		return false
	}
	tok := strings.TrimPrefix(auth, "Bearer ")
	if tok == "" || tok != aggregateAPIKey() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的聚合 API key（默认 sk-rescene-local，可用 RESCENE_AGG_API_KEY 覆盖）"})
		return false
	}
	return true
}

// aggregateChatRequest OpenAI /v1/chat/completions 请求体（按需解析字段，未知字段忽略）。
type aggregateChatRequest struct {
	Model       string          `json:"model"`
	Messages    []map[string]any `json:"messages"`
	Stream      bool            `json:"stream"`
	Tools       []map[string]any `json:"tools"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
}

// modelToAggregateBackends 把外部请求的 model 字段解析成路由链。
func modelToAggregateBackends(model string) []RouterBackend {
	if model == "" || model == "auto" || model == "rescene-auto" {
		return resolveBackends("", "auto")
	}
	if b := resolveExact("", model); b != nil {
		return []RouterBackend{*b}
	}
	// 按 Model 名 / 名称模糊匹配目录（外部工具可能直接填模型名）
	entries, _ := loadModelConfigs("")
	entryByID := map[string]ModelConfigEntry{}
	for _, e := range entries {
		entryByID[e.ID] = e
	}
	lower := strings.ToLower(model)
	for _, f := range freeModelCatalog {
		if f.Disabled {
			continue
		}
		if strings.EqualFold(f.Model, model) || strings.Contains(strings.ToLower(f.Name), lower) {
			key := ""
			if e, ok := entryByID[f.ID]; ok {
				key = e.APIKey
			}
			if key == "" && !f.Local && !f.Keyless {
				key = os.Getenv(f.KeyEnv)
			}
			if key == "" && !f.Local && !f.Keyless {
				continue
			}
			b := RouterBackend{
				ID: f.ID, Name: f.Name, BaseURL: f.Endpoint, Model: f.Model,
				APIKey:           key,
				ParamsB:          f.ParamsB, Timeout: 45 * time.Second, Source: "free",
				Vision: f.Vision, ContextWindow: f.ContextWindow, Reasoning: f.Reasoning,
				IsLocal: f.Local, Keyless: f.Keyless, WireResponses: f.Responses,
			}
			return []RouterBackend{b}
		}
	}
	// 按自动发现模型（auto_ 前缀）解析：/v1/models 暴露的是解码后的可读 ID（hex → 真实模型名），
	// 这里把可读 ID 反解回发现快照里的原始 auto_ 条目再精确路由。
	if b := resolveAutoReadable("", model); b != nil {
		return []RouterBackend{*b}
	}
	// 找不到：回退 Auto 全链（容忍未知模型名，让路由自己挑）
	return resolveBackends("", "auto")
}

// HandleAggregateChat POST /v1/chat/completions
func HandleAggregateChat(c *gin.Context) {
	if !aggregateAuth(c) {
		return
	}
	var req aggregateChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "messages 不能为空"})
		return
	}

	chain := modelToAggregateBackends(req.Model)
	if len(chain) == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "没有可用的免费模型（未配置任何 key）"})
		return
	}
	// 精确指定模型（非 auto）时，追加 Auto 全链做兜底：免费档 429 限流很常见，
	// 单一源失败不该让外部工具（Hermes 等）直接 502 掉线——先试精确源，
	// 挂了秒切 Auto 链里其他可用源（熔断/探活信号自动排序）。
	if req.Model != "" && req.Model != "auto" && req.Model != "rescene-auto" {
		chain = append(chain, resolveBackends("", "auto")...)
	}

	// 构造上游请求体（透传 messages/tools，统一 temperature/max_tokens）
	upstream := map[string]any{
		"model":       "", // 逐 backend 填充
		"messages":    req.Messages,
		"stream":      req.Stream,
		"temperature": req.Temperature,
	}
	if req.Temperature == 0 {
		upstream["temperature"] = 0.2
	}
	if req.MaxTokens > 0 {
		upstream["max_tokens"] = req.MaxTokens
	}
	if len(req.Tools) > 0 {
		upstream["tools"] = req.Tools
	}

	var lastErr error
	for i := range chain {
		b := chain[i]
		upstream["model"] = b.Model
		if req.Stream {
			resp, err := aggregateStreamOnce(c.Request.Context(), b, upstream)
			if err != nil {
				circuitFail(b)
				lastErr = err
				continue // 流还没开始，切下一个
			}
			// 流已建立：转发 SSE（开始后不再 failover，与主链语义一致）
			aggregateForwardSSE(c, b, resp)
			return
		}
		content, calls, err := openAIChatOnce(c.Request.Context(), b, req.Messages, req.Tools)
		if err != nil {
			lastErr = err
			continue
		}
		// 200 已由 openAIChatOnce 内部处理 circuitSuccess
		msg := map[string]any{"role": "assistant", "content": content}
		if len(calls) > 0 {
			tcs := make([]map[string]any, 0, len(calls))
			for _, tc := range calls {
				tcs = append(tcs, map[string]any{
					"id": tc.ID, "type": "function",
					"function": map[string]any{"name": tc.Function.Name, "arguments": tc.Function.Arguments},
				})
			}
			msg["tool_calls"] = tcs
		}
		c.JSON(http.StatusOK, gin.H{
			"id":      "chatcmpl-rescene",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   b.Model,
			"choices": []map[string]any{{
				"index": 0, "message": msg, "finish_reason": "stop",
			}},
			"usage": gin.H{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
		})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": "所有免费模型均失败: " + fmt.Sprint(lastErr)})
}

// aggregateStreamOnce 请求上游流式端点，返回裸响应（SSE 由调用方转发）。
func aggregateStreamOnce(ctx context.Context, b RouterBackend, reqBody map[string]any) (*http.Response, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL(b.BaseURL), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if b.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+b.APIKey)
	}
	client := &http.Client{Timeout: b.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		// 与主链同口径：401/403/404 永久禁用（auto_ 发现模型走 autoDisabled），429/5xx 计入熔断
		if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 {
			if strings.HasPrefix(b.ID, "auto_") {
				disableAutoModel(b.BaseURL, b.Model)
			} else {
				disableFreeModel(b.Model)
			}
		} else if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			circuitFail(b)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateChars(string(raw), 300))
	}
	return resp, nil
}

// aggregateForwardSSE 把上游 OpenAI SSE 流逐 chunk 重组为标准 OpenAI 格式转发。
// 流开始后（收到第一个合法 delta）不再 failover；中途错误直接结束流。
func aggregateForwardSSE(c *gin.Context, b RouterBackend, resp *http.Response) {
	defer resp.Body.Close()
	circuitSuccess(b)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	flusher, _ := c.Writer.(http.Flusher)

	writeChunk := func(delta map[string]any, finish string) {
		payload, _ := json.Marshal(map[string]any{
			"id":      "chatcmpl-rescene",
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   b.Model,
			"choices": []map[string]any{{
				"index": 0, "delta": delta, "finish_reason": finish,
			}},
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
		if flusher != nil {
			flusher.Flush()
		}
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	started := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			writeChunk(map[string]any{}, "stop")
			return
		}
		var chunk struct {
			Choices []struct {
				Delta map[string]any `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		if len(delta) == 0 {
			continue
		}
		started = true
		writeChunk(delta, "")
	}
	if !started {
		// 流没吐出任何内容（上游空响应/中途断）
		writeChunk(map[string]any{}, "stop")
	}
}

// HandleAggregateModels GET /v1/models —— 列出可用模型（OpenAI 格式）。
func HandleAggregateModels(c *gin.Context) {
	if !aggregateAuth(c) {
		return
	}
	data := make([]map[string]any, 0, len(freeModelCatalog)+1)
	// 伪装一个 auto 条目：Hermes 等工具自动探测模型列表后，选它即走 Auto 智能路由
	data = append(data, map[string]any{
		"id": "auto", "object": "model", "owned_by": "Rescene", "created": 0,
	})
	seen := map[string]bool{"auto": true}
	for _, f := range freeModelCatalog {
		if f.Disabled {
			continue
		}
		data = append(data, map[string]any{
			"id": f.ID, "object": "model", "owned_by": f.Vendor, "created": 0,
		})
		seen[f.ID] = true
	}
	// 自动发现的免费池模型（用户配 key 后自动 /v1/models 拉取的）：
	// 内部 ID 是 auto_<vendor>_<hex>（hex 编码真实模型名，防 / : 特殊字符进 URL），
	// 对外解码成 auto_<vendor>_<真实模型名>，外部工具（Hermes 等）看到可读名字，
	// 选它后原样填回 model 字段，路由侧 resolveAutoReadable 反解回原始条目精确路由。
	// 不可用的（探活信号 0 连续失败 / 确定性 401/403/404 淘汰）不输出，避免
	// 外部工具选到付费墙或已下架的模型（动态淘汰 + 探活拉起，见 free_probe.go）。
	// DeepSeek 保活只作用于目录精选条目（disableFreeModel 路径）；自动发现的
	// 付费墙 DeepSeek（Kilo/Ollama/Zen 401/403）照样淘汰，不占列表。
	for _, dm := range discoveredFreeModels("") {
		if seen[dm.ID] {
			continue
		}
		if isAutoModelDisabled(dm.Endpoint, dm.Model) {
			continue
		}
		if sig := probeSignal(RouterBackend{BaseURL: dm.Endpoint, Model: dm.Model}); sig == 0 {
			continue // 探活确认不可用：沉底不输出
		}
		id := autoReadableID(dm.ID)
		if seen[id] {
			continue
		}
		data = append(data, map[string]any{
			"id": id, "object": "model", "owned_by": dm.Vendor, "created": 0,
		})
		seen[dm.ID] = true
		seen[id] = true
	}
	// 自定义提供方模型
	if entries, err := loadModelConfigs(""); err == nil {
		for _, e := range entries {
			if isFreeCatalogID(e.ID) || (e.APIKey == "" && !e.Keyless) {
				continue
			}
			for _, m := range configuredProviderModels(e) {
				id := customModelSelectionID(e.ID, m.ID)
				if seen[id] {
					continue
				}
				data = append(data, map[string]any{
					"id": id, "object": "model", "owned_by": e.Name, "created": 0,
				})
				seen[id] = true
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}
