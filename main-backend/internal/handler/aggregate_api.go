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
	Model       string           `json:"model"`
	Messages    []map[string]any `json:"messages"`
	Stream      bool             `json:"stream"`
	Tools       []map[string]any `json:"tools"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature float64          `json:"temperature"`
}

// aggAutoChain 聚合端口 auto 专用路由链（2026-08-21 重构：不再手写小名单——
// DS 系经常被上游下架/改名，手写名单跟不上变化，导致 auto 链干瘪甚至打空。
// 改成直接复用应用内统一的 resolveBackends("", "auto")：全部免费模型池
// （探活信号降序 → LRU 新鲜度 → 手动排序）+ 用户自定义提供方，模型下架/
// 不可用会被探活自然沉底或熔断跳过，不用再手工维护「实测可用」名单。
//
// 2026-08-21 修复实锤：auto 必须跟随当前聚合页选的模式——custom 模式下用户
// 明确勾选了一份自己的模型清单，auto 却之前直接用全量 resolveBackends（等于
// custom 模式设置形同虚设，选了 3 个模型，auto 照样在全部免费池里乱跳）。
// 现在 custom 模式下 auto 只在用户勾选的 ID 范围内路由；official 模式行为不变。
func aggAutoChain() []RouterBackend {
	cfg := loadAggregateExposeConfig()
	var out []RouterBackend
	if cfg.Mode == "custom" {
		for _, id := range cfg.ModelIDs {
			if b := resolveExact("", id); b != nil {
				out = append(out, *b)
			}
		}
	} else {
		out = resolveBackends("", "auto")
		// 跟随前端分组设定：official 模式默认收全部免费模型（freeModelCatalog +
		// 自动发现），用户自定义提供方仅在免费厂商白名单内才放行（与 isOfficialAllowed /
		// isKnownFreeVendor / exposeOfficial 保持一致，避免个人付费 key 被悄悄算进默认池）。
		filtered := out[:0]
		for _, b := range out {
			if !isOfficialAllowed(b.Model, b.Name) {
				continue
			}
			if b.Source == "user" && !isKnownFreeVendor(b.Name) {
				continue
			}
			filtered = append(filtered, b)
		}
		out = filtered
	}
	// 1. 额度优先：未耗尽排前，耗尽沉底（避免把请求发给已没额度的 Key）
	sort.SliceStable(out, func(i, j int) bool {
		return quotaExhausted(out[i]) != quotaExhausted(out[j]) && !quotaExhausted(out[i])
	})
	// 2. 健康优先：按「探活信号降序 → 实测延迟升序 → 最近成功降序」排，
	//    让 auto 每次都直接走当前最健康的那条，撞到慢/挂源才 failover。
	//    （08-29 实锤：之前只按额度+随机打散，auto 常在随机慢源上拖 4-6s）
	//    用 Timsort（稳定）：auto 链 n≤10 走二分插入分支，最健康已在首时 O(n)。
	if len(out) > 1 {
		// 预取健康快照（probeStates 加锁一次，比较零锁）
		probeMu.Lock()
		items := make([]autoItem, len(out))
		for i := range out {
			k := probeKey(out[i])
			st, ok := probeStates[k]
			sig, lat := -1, time.Duration(0)
			if ok {
				sig, lat = st.signal, st.latency
			}
			items[i] = autoItem{b: out[i], h: autoHealth{
				exhausted: quotaExhausted(out[i]),
				signal:    sig, latency: lat,
				lastOK:      freeLastUsed(out[i]),
				circuitOpen: circuitOpen(out[i]), // 熔断中的源沉底（保护模型不删，但真实失败排最后）
			}}
		}
		probeMu.Unlock()
		less := func(a, b autoItem) bool { // a 是否应排在 b 前
			ha, hb := a.h, b.h
			if ha.exhausted != hb.exhausted {
				return !ha.exhausted
			}
			// 熔断中的源沉底：未熔断排前（真实失败已确认不可用，让健康源先试）
			if ha.circuitOpen != hb.circuitOpen {
				return !ha.circuitOpen
			}
			da, db := ha.signal == 0, hb.signal == 0 // 已确认死的沉底
			if da != db {
				return !da
			}
			// ⚠️ 真实成功优先于探活信号（08-29 实锤）：probe 24h 一轮且只探
			// keyless 网关，用户 key 源（商汤/B.AI）signal 恒 -1，而 LLM7 探活
			// signal=2 却是假健康（真实 400 model_unavailable）。排序若只看
			// signal，假健康的 LLM7 会压在「真实 200 但未探活」的 SenseNova
			// 前面，auto 首跳先撞死源 → 502。真实请求成功时间 lastOK 才是
			// 地面真相：近期真实成功过的源优先于只靠陈旧探活撑着的源。
			//   取「最近一次真实成功」时刻，零值 = 从未真实成功过（只靠探活）。
			if !ha.lastOK.IsZero() != !hb.lastOK.IsZero() {
				return !ha.lastOK.IsZero() // 有真实成功记录的优先
			}
			if ha.signal != hb.signal {
				return ha.signal > hb.signal
			}
			if ha.latency != hb.latency {
				return ha.latency < hb.latency
			}
			return ha.lastOK.After(hb.lastOK)
		}
		items = timSortStable(items, less)
		for i := range items {
			out[i] = items[i].b
		}
		// ⚠️ 不搞「轻微打散」——健康排序已把最健康源稳定在 #1，打散只会
		// 把假健康/未探测源换到最前，让 auto 首跳撞慢/挂源（08-29 实锤：
		// LLM7 探活 signal=2 但真实 400 model_unavailable，被随机换到 #1
		// 后 auto 每次先撞它，成功率暴跌）。最健康源就该一直在第一位。
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
	// ⚠️ 精准选模型绝不 failover（08-13 用户铁律）：外部填了具体模型名（如
	// deepseek-v4-flash），只路由到匹配的第一个可用源，挂了就如实报错，
	// 禁止悄悄切到别的源。failover 只属于 auto 模式。
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
			return []RouterBackend{{
				ID: f.ID, Name: f.Name, BaseURL: f.Endpoint, Model: f.Model,
				APIKey:  key,
				ParamsB: f.ParamsB, Timeout: 45 * time.Second, Source: "free",
				Vision: f.Vision, ContextWindow: f.ContextWindow, Reasoning: f.Reasoning,
				IsLocal: f.Local, Keyless: f.Keyless, WireResponses: f.Responses,
			}}
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
	// 先读原始 body（用于聚合统计 token 估算），再解析请求
	rawBody, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewReader(rawBody))
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
			// 预读第一块，检测空流假成功：200 但无合法 delta → 判失败，触发 failover
			// （08-29 实锤：B.AI 偶发返回 200 但流里无 content，之前被静默透传）
			if resp, err = aggregateStreamFirstDelta(c.Request.Context(), b, resp); err != nil {
				circuitFail(b)
				lastErr = err
				continue
			}
			// 流已建立并验证首块合法：转发 SSE（开始后不再 failover）
			aggStatsInc(b, estimateJSONTokens(rawBody))
			aggregateForwardSSE(c, b, resp)
			return
		}
		content, calls, err := openAIChatOnce(c.Request.Context(), b, req.Messages, req.Tools)
		if err != nil {
			lastErr = err
			continue
		}
		// 200 已由 openAIChatOnce 内部处理 circuitSuccess；这里计一次聚合调用
		aggStatsInc(b, estimateJSONTokens(rawBody))
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
// 暂时性故障（连接错误/429/5xx）在同一源上自动重试 maxTransientRetries 次，
// 撞满才把错误交回 failover；401/403/404/400 确定性失败不重试。
func aggregateStreamOnce(ctx context.Context, b RouterBackend, reqBody map[string]any) (*http.Response, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt <= maxTransientRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
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
		client := backendHTTPClient(b, b.Timeout, false)
		resp, err := client.Do(httpReq)
		if err != nil {
			// 连接失败/超时/EOF：暂时性故障，计入熔断，退避后重试
			circuitFail(b)
			lastErr = err
			if attempt < maxTransientRetries && waitRetry(ctx.Done(), retryWait(0, "", attempt)) {
				continue
			}
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateChars(string(raw), 300))
			// 与主链同口径：401/403/404 永久禁用（auto_ 发现模型走 autoDisabled），不重试
			if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 {
				if strings.HasPrefix(b.ID, "auto_") {
					disableAutoModel(b.BaseURL, b.Model)
				} else {
					disableFreeModel(b.ID)
				}
				return nil, lastErr
			}
			// 429/5xx 暂时性：计入熔断，尊重 Retry-After 退避后重试
			if transientStatus(resp.StatusCode) {
				circuitFail(b)
				if attempt < maxTransientRetries && waitRetry(ctx.Done(), retryWait(resp.StatusCode, resp.Header.Get("Retry-After"), attempt)) {
					continue
				}
			}
			return nil, lastErr
		}
		return resp, nil
	}
	return nil, lastErr
}

// aggregateStreamOnce 的调用方在拿到 resp 后直接转发 SSE。为避免「200 但空流」
// 的假成功被静默透传（2026-08-29 实锤：B.AI 偶发返回 200 但流里无 content，
// auto 链不 failover，客户端等到超时显示空回复），转发前必须预读第一块：
// 空流/无合法 delta 视为该源失败，交回 failover 换下一个源。
func aggregateStreamFirstDelta(ctx context.Context, b RouterBackend, resp *http.Response) (*http.Response, error) {
	br := bufio.NewReaderSize(resp.Body, 64*1024)
	for {
		if ctx.Err() != nil {
			resp.Body.Close()
			return nil, ctx.Err()
		}
		line, err := br.ReadString('\n')
		if err != nil {
			// EOF/错误：流已结束且没拿到任何合法 delta = 空流，判失败
			resp.Body.Close()
			return nil, fmt.Errorf("空流响应（上游 200 但无内容）: %s", b.Name)
		}
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue // 心跳/空行/非 data 行，跳过继续读
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			continue // 首块就 DONE = 空生成，继续等后续（仍未内容则 EOF 时判失败）
		}
		var chunk struct {
			Choices []struct {
				Delta map[string]any `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // 坏 JSON 跳过
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		// ⚠️ 必须真的有内容才算「合法首块」。判断三样：content（正文）、
		// tool_calls（工具调用）、reasoning_content（推理过程）——DeepSeek 系
		// 流式会先发长串 reasoning_content 再发 content，只认 content 会把
		// 推理模型误判成空流，整链 failover 全 502（2026-08-29 三次实锤）。
		hasContent := false
		if c, ok := delta["content"].(string); ok && c != "" {
			hasContent = true
		}
		if rc, ok := delta["reasoning_content"].(string); ok && rc != "" {
			hasContent = true
		}
		if tc, ok := delta["tool_calls"]; ok && tc != nil {
			hasContent = true
		}
		if !hasContent {
			continue // 纯 role/无内容的块跳过
		}
		// 拿到合法首块：把预读的首行补回 Body，让下游无感知继续读完整流
		resp.Body = &prefixedReadCloser{prefix: []byte(line + "\n"), rest: br}
		return resp, nil
	}
}

// prefixedReadCloser 把预读的首行 + 剩余 reader 拼回一个 ReadCloser，
// 让下游 aggregateForwardSSE 无感知地继续读完整流。
type prefixedReadCloser struct {
	prefix []byte
	rest   *bufio.Reader
}

func (p *prefixedReadCloser) Read(b []byte) (int, error) {
	if len(p.prefix) > 0 {
		n := copy(b, p.prefix)
		p.prefix = p.prefix[n:]
		return n, nil
	}
	return p.rest.Read(b)
}

func (p *prefixedReadCloser) Close() error { return nil }

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
//   - official（默认）：官方遴选 = 全部免费模型 + auto，小白用户开箱即用，不用挑模型
//   - custom：用户自定义 = 只暴露用户勾选的模型 ID（目录 / auto_ 发现 / 自定义提供方）
//
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

// isOfficialAllowed 官方默认免费池准入（2026-08-21 改策略）：
// 此前"只留 DeepSeek V4 系"（2026-08-19 收窄）——DS 系频繁被上游下架/改名，
// 官方池经常见底，小白用户误以为聚合端口不能用了。现在反过来：默认收全部免费
// 模型（不分厂商），只留一份「实测有问题」黑名单，其余一律放行：
//   - 腾讯混元 Hy3 系（tencent/hy3 / Tencent-Hunyuan/Hy3 / hy3-free）实测质量不达标 → 踢出
//   - DeepSeek 老代模型（v3 / v3.1 / v3.2 / terminus / r1 / distill / deepseek-chat）→ 踢出
//     （V4 系及以后正常放行；黑名单只在模型名含 deepseek 时生效，不影响其他厂商）
//
// 被踢出的模型仍可在「用户自定义」模式手工勾选，只是不进默认暴露池。
// 厂商侧的免费准入见 isKnownFreeVendor（限制「用户自定义提供方」分支，
// freeModelCatalog / 自动发现的模型本身就是免费源，不需要这层校验）。
func isOfficialAllowed(model, vendor string) bool {
	lower := strings.ToLower(model)
	if strings.Contains(lower, "hy3") || strings.Contains(lower, "hunyuan") {
		return false
	}
	if strings.Contains(lower, "deepseek") {
		for _, legacy := range []string{"r1", "distill", "terminus", "v3", "v3.1", "v3.2", "-chat"} {
			if strings.Contains(lower, legacy) {
				return false
			}
		}
	}
	return true
}

// officialFreeVendors 已知稳定给免费额度的厂商——「用户自定义提供方」（可能是
// 用户自己填的付费 key，如 OpenAI/Anthropic 官方）要进官方默认池，厂商必须在
// 这份白名单里（覆盖免费池目录已用到的全部厂商，含 ModelScope 魔搭全量模型，
// 即使某个模型没在目录 Note 里手写"免费"字样——魔搭是访问令牌额度制，同账号下
// 全部模型共享同一份免费额度，不按模型单独区分免费/付费）。
var officialFreeVendors = []string{
	"modelscope", "魔搭", "魔塔",
	"智谱", "bigmodel", "zhipu",
	"硅基流动", "siliconflow",
	"sensenova", "商汤",
	"stepfun", "阶跃星辰",
	"kilo", "opencode zen",
}

// wholeAccountFreeVendors 「整账号免费额度制」厂商——同一个 key 下全部模型共享同一份
// 免费额度，不区分模型单独付费，所以这几家厂商在 discoveredFreeModels（拉全量
// /v1/models）里新出现的模型也能放心当免费模型收进官方默认池。
//
// 不含 Kilo Gateway / OpenCode Zen：这两个是通用网关，/v1/models 全量列表里绝大多数
// 模型要付费 key（目录里的 :free 后缀条目已经是人工扫过全量列表、逐个筛出来的真免费
// 子集），如果不做区分，discoveredFreeModels 的全量列表会把几百个付费模型也算进来
// （2026-08-21 实测：Kilo 366 个模型只有 13 个真免费，混进去过一次官方池，已修复）。
var wholeAccountFreeVendors = []string{
	"modelscope", "魔搭", "魔塔",
	"sensenova", "商汤",
	"stepfun", "阶跃星辰",
}

// isWholeAccountFreeVendor 判断 vendor 是否整账号免费额度制厂商。
func isWholeAccountFreeVendor(vendor string) bool {
	v := strings.ToLower(vendor)
	for _, f := range wholeAccountFreeVendors {
		if strings.Contains(v, f) {
			return true
		}
	}
	return false
}

// isKnownFreeVendor 判断 vendor 是否已知的免费厂商。
func isKnownFreeVendor(vendor string) bool {
	v := strings.ToLower(vendor)
	for _, f := range officialFreeVendors {
		if strings.Contains(v, f) {
			return true
		}
	}
	return false
}

// isUsableAggModel 等旧筛选函数已废弃（2026-08-18）：DeepSeek 模式改走 isOfficialAllowed，
// 用户自定义模式走后端 model_ids。下方 paidWallVendors / isPaidWallVendor 仍被 isOfficialAllowed 之外的
// 自动发现过滤使用，保留。
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
	ID        string    `json:"id"`         // 与 /v1/models 一致的对外 ID（auto_ 可读 ID / custom::…）
	Vendor    string    `json:"vendor"`     // 厂商分组
	Name      string    `json:"name"`       // 展示名（目录 Name，无则用模型名）
	Model     string    `json:"model"`      // 真实模型名（探活/真实请求用）
	Signal    int       `json:"signal"`     // 0-4；-1 未探测
	ProbeMs   int64     `json:"probe_ms"`   // 探活实测延迟 ms（0=未测）
	RealMs    int64     `json:"real_ms"`    // 真实请求成功延迟 ms（0=暂无记录）
	LastUsed  time.Time `json:"last_used"`  // 最近真实成功时刻（零值=从未）
	Disabled  bool      `json:"disabled"`   // 不可用（淘汰/熔断/确认0格）
	Keyless   bool      `json:"keyless"`    // 免 key 网关
	InAuto    bool      `json:"in_auto"`    // 是不是 auto 链候选（聚合端口 model=auto 的梯队）
	AutoOrder int       `json:"auto_order"` // auto 链中的优先级（1 最前）
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
//   - official（默认）：官方遴选 = 全部免费模型（免费池目录 + 自动发现 + 免费厂商的
//     自定义提供方），覆盖面广不怕单一厂商下架，这是给普通用户的默认体验——
//     只用 auto 就行，不用操心选哪个模型。
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
	Mode           string   `json:"mode"`             // official=官方遴选（默认）| custom=用户自定义
	ModelIDs       []string `json:"model_ids"`        // custom 模式暴露的模型 ID（目录 / auto_ 可读 / custom::）
	LocalProxyPort int      `json:"local_proxy_port"` // 本机本地代理端口（如 9910），B.AI 海外源专用；0=自动探测 Clash/环境变量
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

// exposeOfficial 官方免费模式（2026-08-21 改策略）：暴露全部免费模型（免费池目录 +
// 自动发现 + 免费厂商的自定义提供方），不再收窄到单一厂商——DS 系经常被上游下架，
// 收窄成「只剩 DeepSeek」会让官方池经常见底，小白用户以为聚合端口坏了。
func exposeOfficial() []aggExposedModel {
	out := []aggExposedModel{}
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
		if !isOfficialAllowed(f.Model, f.Vendor) {
			continue
		}
		// 2026-08-21 修复实锤：没配 key 的目录条目之前也照样暴露进 /v1/models，
		// 外部工具（如 DeepSeek Harness）选中后一发请求就是「API key is invalid」，
		// 小白用户一头雾水以为聚合端口坏了。暴露必须以「这台机器真的能路由」为准。
		if !f.Keyless && !f.Local && !hasKey(f.ID, f.KeyEnv, entryByID, envKeys) {
			continue
		}
		// 2026-08-26：免 key 提供方由官方探测基准 + 本地日级探活驱动。真死
		// （确定性错误→signal 0）从列表隐藏；429/503 限流抖动降权保留（signal 1）。
		// 需 key 条目未探活（signal=-1），不在此过滤内。
		if f.Keyless && probeSignalByDef(f) == 0 {
			continue
		}
		out = append(out, aggExposedModel{ID: f.ID, Vendor: f.Vendor, Name: f.Name, Model: f.Model, Endpoint: f.Endpoint, Keyless: f.Keyless})
		seen[f.ID] = true
	}
	{
		for _, e := range entries {
			if isFreeCatalogID(e.ID) || (e.APIKey == "" && !e.Keyless) {
				continue
			}
			if !isKnownFreeVendor(e.Name) {
				continue // 非免费厂商的自定义提供方（可能是用户自己的付费 key）不进官方默认池
			}
			for _, m := range configuredProviderModels(e) {
				id := customModelSelectionID(e.ID, m.ID)
				if seen[id] {
					continue
				}
				if !isOfficialAllowed(m.ID, e.Name) {
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
			// ⚠️ 不再因 Disabled 跳过（08-31 用户铁律：死源可勾回，不物理抹除）。
			// custom = 用户显式逐一勾选，勾了死源就是用户主动要它；Disabled 只应
			// 影响「默认不勾」和「auto 排序沉底」，不该让用户勾选的源静默消失。
			if f.ID != id {
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

// aggCandidate 聚合端口可选模型（设置面板勾选列表，官方/自定义共用）。
type aggCandidate struct {
	ID       string `json:"id"`       // 对外暴露 ID
	Name     string `json:"name"`     // 展示名
	Vendor   string `json:"vendor"`   // 厂商分组
	Model    string `json:"model"`    // 真实模型名
	KeySet   bool   `json:"key_set"`  // 已配 key 或免 key（false = 勾了也路由不了，前端禁用）
	Chat     bool   `json:"chat"`     // 是否聊天模型
	Disabled bool   `json:"disabled"` // 死源标记（前端灰显，但可选可勾回——不再物理抹除，08-31 用户铁律）
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
		// ⚠️ 死源不再物理抹除（08-31 用户铁律）：Disabled 源仍出现在勾选列表，灰显 + 可勾回。
		// 之前 `if f.Disabled { continue }` 让被标死源从候选消失，用户找不回（连坐误杀事故）。
		keySet := f.Keyless || f.Local || hasKey(f.ID, f.KeyEnv, entryByID, envKeys)
		out = append(out, aggCandidate{ID: f.ID, Name: f.Name, Vendor: f.Vendor, Model: f.Model, KeySet: keySet, Chat: isChatModel(f.Model), Disabled: f.Disabled})
		seen[f.ID] = true
	}
	for _, dm := range discoveredFreeModels("") {
		if catalogHasModel(dm.Model, dm.Endpoint) {
			continue // 目录已覆盖，不重复
		}
		if isAutoModelDisabled(dm.Endpoint, dm.Model) {
			continue
		}
		// 2026-08-26：只收整账号免费厂商的自动发现；Kilo/Zen 通用网关全量列表
		// 绝大多数是付费模型，进候选会误导小白（与 /api/models/config 同口径）。
		if !isWholeAccountFreeVendor(dm.Vendor) {
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
		"mode":             cfg.Mode,
		"model_ids":        cfg.ModelIDs,
		"local_proxy_port": cfg.LocalProxyPort,
		"candidates":       aggregateCandidates(),
	})
}

// HandlePutAggregateConfig PUT /api/aggregate/config —— 保存暴露模式配置。
// mode ∈ {official, custom}；model_ids 只保留候选清单里真实存在的 ID（过滤脏 ID）。
func HandlePutAggregateConfig(c *gin.Context) {
	var req struct {
		Mode           string   `json:"mode"`
		ModelIDs       []string `json:"model_ids"`
		LocalProxyPort int      `json:"local_proxy_port"`
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
	// 保护：custom 模式下过滤后 model_ids 为空 → 拒绝保存，保留原配置。
	// 否则「切到空标签 / 勾选的模型全被候选白名单淘汰」时，会把空数组覆盖进
	// aggregate_config.json，导致 /v1/models 只剩 auto，用户以为聚合端口被清空。
	// 误清空危害远大于「用户想全取消」的低频需求（与前端 saveAggConfig 口径一致）。
	if req.Mode == "custom" && len(ids) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户自定义模式至少需保留一个模型，未保存"})
		return
	}
	cfg := aggregateExposeConfig{Mode: req.Mode, ModelIDs: ids, LocalProxyPort: req.LocalProxyPort}
	if err := saveAggregateExposeConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "mode": cfg.Mode, "model_ids": cfg.ModelIDs, "local_proxy_port": cfg.LocalProxyPort})
}
