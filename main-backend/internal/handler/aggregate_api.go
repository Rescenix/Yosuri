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
//   - 其他字符串 → 按 Model 名 / 名称模糊匹配目录，找不到明确报错
//     （2026-08-13 铁律：非 auto 精确模型禁止偷偷 failover 回退 Auto 链）
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
	"path/filepath"
	"sort"
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

// aggAutoChain 聚合端口 auto 专用路由链（2026-08-14 重构：Zen 已死；
// 2026-08-14 晚：智谱 GLM-4.7-Flash 踢出 auto 链——它在工具调用循环返回
// HTTP200 + content='' + tool_calls（usage=0），Hermes 判空回复 → No reply 实锤）
// 按速度+稳定性优先级：魔搭 Step-3.7 → 商汤 DS V4 Flash → 魔搭 Qwen3-235B → Zen 兜底
// 每个源检查 key 有无（免 key 的直接进，要 key 的检查 user_configs/env），无 key 跳过。
func aggAutoChain() []RouterBackend {
	entries, _ := loadModelConfigs("")
	entryByID := map[string]ModelConfigEntry{}
	for _, e := range entries {
		entryByID[e.ID] = e
	}
	envKeys := userKeysByEnv("")

	// 按优先级逐一构造
	backends := []struct {
		id    string
		vendor string
		model string
		base  string
		keyEnv string
		keyless bool
		reasoning bool
		timeout time.Duration
		vision bool
		window int
	}{}
	// 1. 魔搭 Step-3.7-flash（200 OK 实测可用）
	if hasKey("free_modelscope_deepseek_v4_flash", "MODELSCOPE_API_KEY", entryByID, envKeys) {
		backends = append(backends, struct {
			id    string; vendor string; model string; base string; keyEnv string; keyless bool; reasoning bool; timeout time.Duration; vision bool; window int
		}{
			id: "auto_modelscope_stepfun-ai/Step-3.7-Flash", vendor: "ModelScope 魔搭", model: "stepfun-ai/Step-3.7-Flash",
			base: "https://api-inference.modelscope.cn/v1", keyEnv: "MODELSCOPE_API_KEY", reasoning: true, timeout: 45,
		})
	}
	// 3. 商汤 DS V4 Flash（200 OK 实测可用）
	if hasKey("free_sensenova_deepseek_v4_flash", "SENSENOVA_API_KEY", entryByID, envKeys) {
		backends = append(backends, struct {
			id    string; vendor string; model string; base string; keyEnv string; keyless bool; reasoning bool; timeout time.Duration; vision bool; window int
		}{
			id: "free_sensenova_deepseek_v4_flash", vendor: "SenseNova", model: "deepseek-v4-flash",
			base: "https://token.sensenova.cn/v1", keyEnv: "SENSENOVA_API_KEY", reasoning: true, timeout: 45, window: 1048576,
		})
	}
	// 4. 魔搭 Qwen3-235B（200 OK 实测可用）
	if hasKey("free_modelscope_qwen3_235b", "MODELSCOPE_API_KEY", entryByID, envKeys) {
		backends = append(backends, struct {
			id    string; vendor string; model string; base string; keyEnv string; keyless bool; reasoning bool; timeout time.Duration; vision bool; window int
		}{
			id: "free_modelscope_qwen3_235b", vendor: "ModelScope 魔搭", model: "Qwen/Qwen3-235B-A22B",
			base: "https://api-inference.modelscope.cn/v1", keyEnv: "MODELSCOPE_API_KEY", reasoning: true, timeout: 45, window: 131072,
		})
	}
	// 5. Zen 免 key DS（经常 502 限流，放最后兜底）
	backends = append(backends, struct {
		id    string; vendor string; model string; base string; keyEnv string; keyless bool; reasoning bool; timeout time.Duration; vision bool; window int
	}{
		id: "free_zen_deepseek_v4_flash", vendor: "OpenCode Zen", model: "deepseek-v4-flash-free",
		base: "https://opencode.ai/zen/v1", keyless: true, reasoning: true, timeout: 15,
	})

	out := make([]RouterBackend, 0, len(backends))
	for _, b := range backends {
		key := ""
		if e, ok := entryByID[b.id]; ok {
			key = e.APIKey
		}
		if key == "" && !b.keyless && b.keyEnv != "" {
			if envKeys[b.keyEnv] != "" {
				key = envKeys[b.keyEnv]
			} else {
				key = os.Getenv(b.keyEnv)
			}
		}
		if key == "" && !b.keyless {
			continue // 无 key 跳过
		}
		out = append(out, RouterBackend{
			ID: b.id, Name: b.model, BaseURL: b.base, Model: b.model,
			APIKey: key, Timeout: b.timeout * time.Second, Source: "free",
			Keyless: b.keyless, Reasoning: b.reasoning, Vision: b.vision,
			ContextWindow: b.window,
		})
	}
	return out
}

// hasKey 检查某个条目是否有 key（user_configs / env / keyless）
func hasKey(id, keyEnv string, entryByID map[string]ModelConfigEntry, envKeys map[string]string) bool {
	if e, ok := entryByID[id]; ok && e.APIKey != "" {
		return true
	}
	if keyEnv == "" {
		return true // 免 key
	}
	if envKeys[keyEnv] != "" {
		return true
	}
	return os.Getenv(keyEnv) != ""
}

// modelToAggregateBackends 把外部请求的 model 字段解析成路由链。
func modelToAggregateBackends(model string) []RouterBackend {
	if model == "" || model == "auto" || model == "rescene-auto" {
		return aggAutoChain()
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
	// 找不到：明确返回空链，绝不回退 Auto 全链（2026-08-13 用户铁律：
	// 非 auto 精确模型禁止偷偷 failover，挂了就明确报错，让调用方如实返回）。
	// 此前这里 `return resolveBackends("", "auto")` 会把未匹配的精确模型名
	// 静默替换成 Auto 多源轮换链，产生「选了 A 却在跑 B/C/D」的隐形 failover。
	return nil
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
		// 空链两种成因，报错要分清（2026-08-13 铁律配套）：
		// 1. 精确模型名未匹配 → 明确告诉调用方，绝不静默换成别的模型
		// 2. auto 但一个可用源都没有 → 提示配置 key
		if req.Model != "" && req.Model != "auto" && req.Model != "rescene-auto" {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("模型 %q 未找到（精确模型禁止自动回退，请检查模型名或改用 auto）", req.Model)})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "没有可用的免费模型（未配置任何 key）"})
		return
	}
	// 精确指定模型（非 auto）绝不追加 Auto 链：用户选定哪个模型就路由到哪个，
	// 挂了就明确报错，禁止偷偷 failover 到别的模型（2026-08-13 用户铁律）。

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
	tried := []string{} // 实际尝试过的上游模型（精确模型=1个；auto=多个），报错时如实列出
	for i := range chain {
		b := chain[i]
		tried = append(tried, b.Model)
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
	// 报错如实区分：精确模型（只试了 1 个）直接点名；auto（多源轮换）才说「所有」
	if len(tried) <= 1 {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("模型 %s 请求失败: %s", tried[0], lastErr)})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("所有免费模型均失败（已尝试: %s）: %s", strings.Join(tried, " → "), lastErr)})
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
	// 带浏览器 UA：Cerebras/Zen 等走 Cloudflare 风控，无 UA 返回 403/1009（2026-08-13 实测）
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
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
// 暴露范围由配置决定（~/rescene_data/aggregate_config.json）：
//   - official（默认）：官方遴选 = DS V4 系 + auto + 「快又聪明」精选，干净聚焦
//   - custom：用户自定义 = 只暴露用户勾选的模型 ID（目录 / auto_ 发现 / 自定义提供方）
// auto 智能路由入口永远在列表里。
func HandleAggregateModels(c *gin.Context) {
	if !aggregateAuth(c) {
		return
	}
	data := make([]map[string]any, 0, 16)
	// 伪装一个 auto 条目：Hermes 等工具自动探测模型列表后，选它即走 Auto 智能路由
	data = append(data, map[string]any{
		"id": "auto", "object": "model", "owned_by": "Rescene", "created": 0,
	})
	seen := map[string]bool{"auto": true}
	for _, m := range aggregateExposedModels() {
		if seen[m.ID] {
			continue
		}
		data = append(data, map[string]any{
			"id": m.ID, "object": "model", "owned_by": m.Vendor, "created": 0,
		})
		seen[m.ID] = true
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

// isDeepSeekModel 判断模型名是否 DeepSeek 系（聚合端口只暴露它）。
func isDeepSeekModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "deepseek")
}

// isUsableAggModel 聚合端口暴露规则（2026-08-13 用户定稿扩展）：
//   - 付费墙 vendor 排除（kilo gateway / ollama cloud）
//   - NVIDIA NIM 移除（用户「纯纯垃圾，不要了」——实测慢/超时）
//   - 保留 DeepSeek V4 系 + agent 基准与 DS 相持的顶级免费模型
//     （GLM-5.2 / Qwen3.5-397B / Qwen3-235B / MiMo / GPT-OSS / Laguna / step-3.7，
//      2026-08-13 实测收录；GLM-5.2 商汤 429/魔搭空回复由探活动态沉底）
//   - Kilo Gateway 免 key 模型仅用于应用面板，不加入聚合端口（2026-08-15）
func isUsableAggModel(model, vendor string) bool {
	if isPaidWallVendor(vendor) {
		return false
	}
	v := strings.ToLower(vendor)
	if strings.Contains(v, "nvidia") || strings.Contains(v, "订阅") {
		return false // NVIDIA NIM 移除（用户 2026-08-13）+ 订阅档（plan_*）不暴露
	}
	if isUsableDeepSeek(model, vendor) {
		return true
	}
	ml := strings.ToLower(model)
	for _, top := range aggTopModels {
		if strings.Contains(ml, top) {
			return true
		}
	}
	return false
}

// aggTopModels 聚合端口「快又聪明」名单（2026-08-13 用户二轮收窄定稿）：
// 用户「聚合端口跑 Hermes 太多垃圾太卡」——只留实测秒回 + agent 可用的旗舰：
//   step-3.7-flash 1.6s / Qwen3-235B 2.1s（魔搭实测）
// 已剔除（实测慢或废）：
//   MiMo 12.8s、Qwen3.5-397B 4.9-10.8s（慢）、GLM-5.2（商汤 429/魔搭空回复）、
//   Laguna 5.9s+免key限流、GPT-OSS-120B（Cerebras/Groq 地域风控 403 + Ollama/Kilo 付费墙，无活源）。
// 注意收窄到旗舰线：qwen3 只留 235b（27/35/122B 非顶级）、gpt-oss 只留 120b（20b 用户否决）。
var aggTopModels = []string{
	"qwen3-235b",   // 通义 Qwen3-235B（魔搭 2.1s 实测秒回）
	"step-3.7",     // 阶跃 step-3.7-flash（1.6s 实测秒回）
}

// isUsableDeepSeek 判断是否聚合端口真正可用的 DeepSeek：
// 1. 只认 V4 系列（V4-Flash / V4-Pro 实测可用）；v3.x / r1 / chat 等低版本实测用不了
// 2. 排除付费墙提供方（Kilo 401、Ollama Cloud 403，挂着占列表、选了必挂）
func isUsableDeepSeek(model, vendor string) bool {
	if !isDeepSeekModel(model) {
		return false
	}
	if isPaidWallVendor(vendor) {
		return false
	}
	lower := strings.ToLower(model)
	// 只保留 V4 系；排除低版本（v3.x/r1/chat/coder 等）
	if !strings.Contains(lower, "v4") {
		return false
	}
	return true
}

// paidWallVendors 聚合端口直接排除的提供方：实测 DeepSeek 全是付费墙
// （Kilo 401 PAID_MODEL_AUTH_REQUIRED、Ollama Cloud 403 requires subscription）。
var paidWallVendors = []string{"kilo gateway", "ollama cloud"}

// isPaidWallVendor 判断 vendor 是否付费墙提供方（其 DeepSeek 不可用）。
func isPaidWallVendor(vendor string) bool {
	v := strings.ToLower(vendor)
	for _, p := range paidWallVendors {
		if strings.Contains(v, p) {
			return true
		}
	}
	return false
}

// ========== 聚合 API 健康度可视化（2026-08-14）==========
// GET /api/aggregate/health —— 设置面板「聚合 API」tab 的健康度卡片数据源。
// 返回聚合端口实际暴露的每个模型（与 /v1/models 同过滤规则）的：
//   - 探活信号格 signal（0-4，-1=未探测/无key未测）
//   - 探活实测延迟 probe_ms（最近一轮 probeChatOnce 的真实毫秒数）
//   - 真实请求成功延迟 real_ms（探活只是敲门砖，真实请求延迟更能反映实际体验；
//     没有真实成功记录时回退为探活延迟，再没有就是 0）
//   - 最近真实成功时刻 last_used（LRU 新鲜度，零值=从未成功）
//   - 可用状态：disabled = 探活确认 0 格 / 确定性 401-403-404 淘汰 / 熔断中
//
// 纯读内存状态，零额外探活、零 key 泄露（signal/latency 不涉及密钥）。

// aggHealthModel 健康度单条视图。
type aggHealthModel struct {
	ID         string    `json:"id"`          // 与 /v1/models 一致的对外 ID（auto_ 可读 ID / custom::…）
	Vendor     string    `json:"vendor"`      // 厂商分组
	Name       string    `json:"name"`        // 展示名（目录 Name，无则用模型名）
	Model      string    `json:"model"`       // 真实模型名（探活/真实请求用）
	Signal     int       `json:"signal"`      // 0-4；-1 未探测
	ProbeMs    int64     `json:"probe_ms"`    // 探活实测延迟 ms（0=未测）
	RealMs     int64     `json:"real_ms"`     // 真实请求成功延迟 ms（0=暂无记录）
	LastUsed   time.Time `json:"last_used"`   // 最近真实成功时刻（零值=从未）
	Disabled   bool      `json:"disabled"`    // 不可用（淘汰/熔断/确认0格）
	Keyless    bool      `json:"keyless"`     // 免 key 网关
	InAuto     bool      `json:"in_auto"`     // 是不是 auto 链候选（聚合端口 model=auto 的梯队）
	AutoOrder  int       `json:"auto_order"`  // auto 链中的优先级（1 最前）
}

// aggModelHealth 读一个 backend 的健康度状态（零锁外开销）。
func aggModelHealth(b RouterBackend) aggHealthModel {
	m := aggHealthModel{
		ID: b.ID, Vendor: "", Name: b.Name, Model: b.Model,
		Signal: -1, Keyless: b.Keyless,
		LastUsed: freeLastUsed(b),
	}
	probeMu.Lock()
	if st, ok := probeStates[probeKey(b)]; ok {
		m.Signal = st.signal
		m.ProbeMs = st.latency.Milliseconds()
	}
	probeMu.Unlock()
	// ⚠️ 不用 lastLatency 回退：它存的是 b.Timeout（配置超时值，如 45s），
	// 不是实测延迟（free_probe.go markFreeUsed 既有实现），画进去会误导。
	if m.ProbeMs == 0 {
		m.RealMs = 0
	} else {
		m.RealMs = m.ProbeMs
	}
	// 不可用判定：确定性淘汰 + 熔断 + 探活确认 0 格
	m.Disabled = isAutoModelDisabled(b.BaseURL, b.Model) ||
		circuitOpen(b) ||
		(m.Signal == 0)
	return m
}

// aggBackendVendor 从目录按 backend ID 反查 vendor（auto 链 log 里 RouterBackend
// 不带 Vendor 字段，显示用厂商标识，查不到就用模型名兜底）。
func aggBackendVendor(b RouterBackend) string {
	for _, f := range freeModelCatalog {
		if f.ID == b.ID {
			return f.Vendor
		}
	}
	return b.Name
}

// HandleAggregateHealth GET /api/aggregate/health
func HandleAggregateHealth(c *gin.Context) {
	// 1. auto 链候选（聚合端口 model=auto 的实际路由梯队）
	autoChain := make([]aggHealthModel, 0, 8)
	for i, b := range aggAutoChain() {
		m := aggModelHealth(b)
		m.InAuto = true
		m.AutoOrder = i + 1
		m.Vendor = aggBackendVendor(b)
		autoChain = append(autoChain, m)
	}

	// 2. 全部暴露模型（与 /v1/models 同规则：official 官方遴选 / custom 用户自定义）
	models := make([]aggHealthModel, 0, 32)
	for _, m := range aggregateExposedModels() {
		hm := aggModelHealth(RouterBackend{ID: m.ID, Name: m.Name, BaseURL: m.Endpoint, Model: m.Model, Keyless: m.Keyless})
		hm.Vendor = m.Vendor
		models = append(models, hm)
	}

	// 排序：信号降序 → auto 优先 → 名称
	sort.SliceStable(models, func(i, j int) bool {
		if models[i].Signal != models[j].Signal {
			return models[i].Signal > models[j].Signal
		}
		return models[i].Name < models[j].Name
	})

	c.JSON(http.StatusOK, gin.H{
		"auto_chain": autoChain,
		"models":     models,
	})
}

// ========== 聚合 API 暴露模型配置（2026-08-17，issue #5）==========
// 用户诉求：聚合 API 可以自行设置添加哪些模型。
// 两个模式：
//   - official（默认）：官方遴选 = isUsableAggModel 精选（DS V4 系 + 快又聪明），
//     保证聚合端口跑 Agent 又快又稳，这是给普通用户的默认体验。
//   - custom：用户自定义 = 只暴露用户在设置面板勾选的模型 ID，想加什么加什么
//     （如 Kilo 免 key 的 kilo-auto/free），不受官方精选限制。
//
// 配置存 ~/rescene_data/aggregate_config.json（与 free_model_order.json 同目录，
// 全局共享不分用户）。

// aggExposedModel 聚合端口实际暴露的单个模型（列表与健康度共用）。
type aggExposedModel struct {
	ID       string // 对外 ID（目录 ID / auto_ 可读 ID / custom:: 选择 ID）
	Vendor   string
	Name     string // 展示名
	Model    string // 真实模型名（上游请求用）
	Endpoint string
	Keyless  bool
}

// aggregateExposeConfig 聚合 API 暴露模型配置。
type aggregateExposeConfig struct {
	Mode     string   `json:"mode"`      // official=官方遴选（默认）| custom=用户自定义
	ModelIDs []string `json:"model_ids"` // custom 模式暴露的模型 ID（目录 / auto_ 可读 / custom::）
}

func aggregateConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "rescene_data", "aggregate_config.json"), nil
}

func loadAggregateExposeConfig() aggregateExposeConfig {
	cfg := aggregateExposeConfig{Mode: "official"}
	path, err := aggregateConfigPath()
	if err != nil {
		return cfg
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg
	}
	if cfg.Mode != "custom" {
		cfg.Mode = "official"
	}
	return cfg
}

func saveAggregateExposeConfig(cfg aggregateExposeConfig) error {
	path, err := aggregateConfigPath()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// aggregateExposedModels 按配置返回聚合端口实际暴露的模型（列表 + 健康度共用）。
func aggregateExposedModels() []aggExposedModel {
	cfg := loadAggregateExposeConfig()
	if cfg.Mode == "custom" {
		return exposeByCustomIDs(cfg)
	}
	return exposeOfficial()
}

// exposeOfficial 官方遴选：DS V4 系 + 快又聪明精选（2026-08-13 用户定稿规则）。
func exposeOfficial() []aggExposedModel {
	out := []aggExposedModel{}
	seen := map[string]bool{}
	for _, f := range freeModelCatalog {
		if f.Disabled || !isUsableAggModel(f.Model, f.Vendor) {
			continue
		}
		out = append(out, aggExposedModel{ID: f.ID, Vendor: f.Vendor, Name: f.Name, Model: f.Model, Endpoint: f.Endpoint, Keyless: f.Keyless})
		seen[f.ID] = true
	}
	for _, dm := range discoveredFreeModels("") {
		if seen[dm.ID] || !isUsableAggModel(dm.Model, dm.Vendor) {
			continue
		}
		if catalogHasModel(dm.Model, dm.Endpoint) {
			continue // 目录已有同 endpoint 同模型，不重复暴露
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
		out = append(out, aggExposedModel{ID: id, Vendor: dm.Vendor, Name: dm.Model, Model: dm.Model, Endpoint: dm.Endpoint, Keyless: dm.Keyless})
		seen[dm.ID] = true
		seen[id] = true
	}
	if entries, err := loadModelConfigs(""); err == nil {
		for _, e := range entries {
			if isFreeCatalogID(e.ID) || (e.APIKey == "" && !e.Keyless) {
				continue
			}
			for _, m := range configuredProviderModels(e) {
				if !isUsableAggModel(m.ID, e.Name) {
					continue
				}
				id := customModelSelectionID(e.ID, m.ID)
				if seen[id] {
					continue
				}
				cname := strings.TrimSpace(m.Name)
				if cname == "" {
					cname = m.ID
				}
				out = append(out, aggExposedModel{ID: id, Vendor: e.Name, Name: cname, Model: m.ID, Endpoint: e.Endpoint, Keyless: e.Keyless})
				seen[id] = true
			}
		}
	}
	return out
}

// exposeByCustomIDs 用户自定义：只暴露用户勾选的模型 ID，用户自己负责挑选。
// 三类 ID 都支持：免费池目录 ID / auto_ 可读 ID（或内部 ID）/ 自定义提供方选择 ID。
// 无 key 且非免 key 的目录条目跳过（列表里出现的都保证能路由）。
func exposeByCustomIDs(cfg aggregateExposeConfig) []aggExposedModel {
	out := []aggExposedModel{}
	seen := map[string]bool{}
	entries, _ := loadModelConfigs("")
	entryByID := map[string]ModelConfigEntry{}
	for _, e := range entries {
		entryByID[e.ID] = e
	}
	envKeys := userKeysByEnv("")
next:
	for _, id := range cfg.ModelIDs {
		if seen[id] {
			continue
		}
		// 1. 免费池目录 ID
		for _, f := range freeModelCatalog {
			if f.Disabled || f.ID != id {
				continue
			}
			if !f.Keyless && !f.Local && !hasKey(f.ID, f.KeyEnv, entryByID, envKeys) {
				continue next // 没配 key 的模型不进列表（选了必挂）
			}
			out = append(out, aggExposedModel{ID: f.ID, Vendor: f.Vendor, Name: f.Name, Model: f.Model, Endpoint: f.Endpoint, Keyless: f.Keyless})
			seen[id] = true
			continue next
		}
		// 2. auto_ 自动发现模型（可读 ID 或内部 ID 都能认）
		for _, dm := range discoveredFreeModels("") {
			if autoReadableID(dm.ID) != id && dm.ID != id {
				continue
			}
			if isAutoModelDisabled(dm.Endpoint, dm.Model) {
				continue next // 确定性淘汰的模型不暴露
			}
			out = append(out, aggExposedModel{ID: autoReadableID(dm.ID), Vendor: dm.Vendor, Name: dm.Model, Model: dm.Model, Endpoint: dm.Endpoint, Keyless: dm.Keyless})
			seen[id] = true
			continue next
		}
		// 3. 自定义提供方选择 ID（custom::…）
		if providerID, modelID, ok := parseCustomModelSelectionID(id); ok {
			for _, e := range entries {
				if e.ID != providerID || isFreeCatalogID(e.ID) {
					continue
				}
				if e.APIKey == "" && !e.Keyless {
					continue next
				}
				for _, mm := range configuredProviderModels(e) {
					if mm.ID != modelID {
						continue
					}
					cname := strings.TrimSpace(mm.Name)
					if cname == "" {
						cname = mm.ID
					}
					out = append(out, aggExposedModel{ID: id, Vendor: e.Name, Name: cname, Model: mm.ID, Endpoint: e.Endpoint, Keyless: e.Keyless})
					seen[id] = true
					continue next
				}
			}
		}
	}
	return out
}

// aggCandidate 聚合端口可选模型（设置面板「用户自定义」勾选列表）。
type aggCandidate struct {
	ID     string `json:"id"`     // 对外暴露 ID
	Name   string `json:"name"`   // 展示名
	Vendor string `json:"vendor"` // 厂商分组
	Model  string `json:"model"`  // 真实模型名
	KeySet bool   `json:"key_set"` // 已配 key 或免 key（false = 勾了也路由不了，前端禁用）
	Chat   bool   `json:"chat"`   // 是否聊天模型（false = TTS/ASR/生图/实时等非对话，勾了 chat/completions 必挂）
}

// isChatModel 判断模型是不是聊天模型。排除明显非对话的：
// 语音合成 TTS / 语音识别 ASR / 生图编辑 image-edit / 实时音频 realtime /
// 音频生成 overture / 路由 router / 专用搜索 API（dr-search / search-image）。
// ⚠️ vision 不算非聊天（聊天+识图），audio 语音对话系列（step-audio-2 等）保留。
func isChatModel(model string) bool {
	m := strings.ToLower(model)
	for _, kw := range []string{"tts", "asr", "image-edit", "image_edit", "router", "realtime", "overture", "dr-search", "search-image"} {
		if strings.Contains(m, kw) {
			return false
		}
	}
	return true
}

// aggregateCandidates 全部可选模型：免费池目录（全量，不受官方精选限制）+
// auto_ 自动发现（可读 ID）+ 自定义提供方模型。
func aggregateCandidates() []aggCandidate {
	out := []aggCandidate{}
	seen := map[string]bool{}
	entries, _ := loadModelConfigs("")
	entryByID := map[string]ModelConfigEntry{}
	for _, e := range entries {
		entryByID[e.ID] = e
	}
	envKeys := userKeysByEnv("")
	for _, f := range freeModelCatalog {
		if f.Disabled {
			continue
		}
		keySet := f.Keyless || f.Local || hasKey(f.ID, f.KeyEnv, entryByID, envKeys)
		out = append(out, aggCandidate{ID: f.ID, Name: f.Name, Vendor: f.Vendor, Model: f.Model, KeySet: keySet, Chat: isChatModel(f.Model)})
		seen[f.ID] = true
	}
	for _, dm := range discoveredFreeModels("") {
		if catalogHasModel(dm.Model, dm.Endpoint) {
			continue // 目录已覆盖，不重复
		}
		if isAutoModelDisabled(dm.Endpoint, dm.Model) {
			continue
		}
		id := autoReadableID(dm.ID)
		if seen[id] {
			continue
		}
		// 自动发现快照只对有 key 的提供方拉取（免 key 网关也在列）——
		// 能进快照 = key 已配好或本身免 key，一律可勾选（2026-08-17 bug 修复：
		// 之前写成 dm.Keyless，把「需 key 的提供方」误标成未配 key 禁用）。
		out = append(out, aggCandidate{ID: id, Name: dm.Model, Vendor: dm.Vendor, Model: dm.Model, KeySet: true, Chat: isChatModel(dm.Model)})
		seen[id] = true
	}
	for _, e := range entries {
		if isFreeCatalogID(e.ID) || (e.APIKey == "" && !e.Keyless) {
			continue
		}
		for _, mm := range configuredProviderModels(e) {
			id := customModelSelectionID(e.ID, mm.ID)
			if seen[id] {
				continue
			}
			cname := strings.TrimSpace(mm.Name)
			if cname == "" {
				cname = mm.ID
			}
			out = append(out, aggCandidate{ID: id, Name: cname, Vendor: e.Name, Model: mm.ID, KeySet: e.APIKey != "" || e.Keyless, Chat: isChatModel(mm.ID)})
			seen[id] = true
		}
	}
	// 按 vendor 分组 + free 过滤（2026-08-17 issue #5 二轮）：
	// 提供方模型列表里有 free 后缀（:free / /free / -free）→ 只显示 free 的；
	// 没有 free 后缀 → 全显示。避免 Kilo 这类网关 361 个付费模型全列出来。
	groups := map[string][]aggCandidate{}
	var order []string
	for _, c := range out {
		if _, ok := groups[c.Vendor]; !ok {
			order = append(order, c.Vendor)
		}
		groups[c.Vendor] = append(groups[c.Vendor], c)
	}
	filtered := []aggCandidate{}
	for _, v := range order {
		list := groups[v]
		hasFree := false
		for _, c := range list {
			if hasFreeSuffix(c.Model) {
				hasFree = true
				break
			}
		}
		if hasFree {
			for _, c := range list {
				if hasFreeSuffix(c.Model) {
					filtered = append(filtered, c)
				}
			}
		} else {
			filtered = append(filtered, list...)
		}
	}
	return filtered
}

// hasFreeSuffix 模型名是否带免费后缀标记（:free / /free / -free）。
// 如 stepfun/step-3.7-flash:free、kilo-auto/free、deepseek-v4-flash-free。
func hasFreeSuffix(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, ":free") || strings.Contains(m, "/free") || strings.Contains(m, "-free")
}

// HandleGetAggregateConfig GET /api/aggregate/config —— 读取暴露模式配置 + 可选模型清单。
func HandleGetAggregateConfig(c *gin.Context) {
	cfg := loadAggregateExposeConfig()
	c.JSON(http.StatusOK, gin.H{
		"mode":       cfg.Mode,
		"model_ids":  cfg.ModelIDs,
		"candidates": aggregateCandidates(),
	})
}

// HandlePutAggregateConfig PUT /api/aggregate/config —— 保存暴露模式配置。
// mode ∈ {official, custom}；model_ids 只保留候选清单里真实存在的 ID（过滤脏 ID）。
func HandlePutAggregateConfig(c *gin.Context) {
	var req struct {
		Mode     string   `json:"mode"`
		ModelIDs []string `json:"model_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Mode != "official" && req.Mode != "custom" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode 必须是 official 或 custom"})
		return
	}
	valid := map[string]bool{}
	for _, cand := range aggregateCandidates() {
		valid[cand.ID] = true
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, id := range req.ModelIDs {
		if seen[id] || !valid[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	cfg := aggregateExposeConfig{Mode: req.Mode, ModelIDs: ids}
	if err := saveAggregateExposeConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "mode": cfg.Mode, "model_ids": cfg.ModelIDs})
}
