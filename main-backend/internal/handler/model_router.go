package handler

// AAP 免费模型路由层（aap/agent/inference.py InferenceRouter）的 Go 移植。
//
// 路由链（按优先级）：
//  1. 用户自定义配置（设置面板填的 Key，默认条目排最前）—— ~/.Aurora/user_configs/{openid}.json
//  2. 免费模型池（参数规模降序，未知参数量排末，绝不伪造数字）
//
// 秒切 failover：任一源连不上 / 非 200 / 空响应，立刻切下一个，绝不重试当前源；
// 所有源都失败才报错，tried 轨迹完整可观测。
// 免费池的 Key 来源：设置面板保存的同 ID 条目优先，环境变量兜底。

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

type RouterBackend struct {
	Name    string
	BaseURL string
	Model   string
	APIKey  string // 空 = 免 key（本地）
	ParamsB float64
	IsLocal bool
	Timeout time.Duration
	Source  string // user / env / free / local
	// 能力元数据：前端按模型配置，决定能否识图 / 上下文窗口 / 是否支持思考强度
	Vision        bool `json:"vision"`
	ContextWindow int  `json:"context_window"`
	Reasoning     bool `json:"reasoning"`
}

// FreeModelDef 免费模型池的一项（设置面板默认展示这份目录）
type FreeModelDef struct {
	ID       string  `json:"id"`
	Vendor   string  `json:"vendor"` // 厂商分组（设置面板按此折叠）
	Name     string  `json:"name"`
	Endpoint string  `json:"endpoint"`
	Model    string  `json:"model"`
	KeyEnv   string  `json:"-"`
	ParamsB  float64 `json:"params_b"`
	Note     string  `json:"note"`
	// 能力元数据（公开已知值；未知者留 0/false，绝不伪造）
	Vision        bool `json:"vision"`
	ContextWindow int  `json:"context_window"`
	Reasoning     bool `json:"reasoning"`
	// Local=true 表示走本地 Ollama（localhost:11434/v1，OpenAI 兼容）路由到云端模型，
	// 不需要 API Key，复用现有 OpenAI 兼容链。
	Local bool `json:"local"`
	// Disabled=true 表示该模型被运行时探测判定为不可用（如提供方退役/下架），
	// 路由链与精确解析均跳过；由 nimRefresh 每日探测后动态置位。
	Disabled bool `json:"disabled"`
}

// 参数规模是公开估计值；未知者写 0，排序时排免费池末段，绝不伪造。
// Vendor 字段用于前端按厂商折叠分组（仿 Hermes 提供方分类）。
var freeModelCatalog = []FreeModelDef{
	// —— OpenRouter 已整体移除：免费档全部 slug 限流 429（连 llama-3.3-70b/405b 都 429），
	// 无专属免费模型，作为网关接入价值为零，徒增链尾失败噪声。2026-07-15 实测确认。

	// —— Google AI Studio 移除：2026-07-21 实测网络不可达（WinError 10060），大陆无法直连。 ——

	// —— NVIDIA NIM 免费试用档（integrate.api.nvidia.com，实测可完成对话；已剔除 PII/内容安全等非对话模型）——
	{ID: "free_nim_abacusai_dracarys_llama_3_1_70b_instruct", Vendor: "NVIDIA NIM", Name: "Dracarys-Llama-3.1-70B", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "abacusai/dracarys-llama-3.1-70b-instruct", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 70, Note: "NIM 免费试用档"},
	// DeepSeek-V4-Flash(NIM) 移除：2026-07-21 实测 HTTP 503 ResourceExhausted（额度耗尽）。
	// DeepSeek-V4-Pro(NIM) 移除：2026-07-21 实测 41.3s，agent 场景过慢。
	{ID: "free_nim_meta_llama_3_1_70b_instruct", Vendor: "NVIDIA NIM", Name: "Llama-3.1-70B", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "meta/llama-3.1-70b-instruct", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 70, Note: "NIM 免费试用档"},
	// Llama-3.3-70B(NIM) 移除：2026-07-21 实测 42.5s，agent 场景过慢。
	// Phi-4-mini 移除：2026-07-15 实测 HTTP 410 Gone（NIM 已永久下架）。
	// Stockmark-2-100B 移除：2026-07-15 实测 HTTP 410 Gone（NIM 已永久下架）。
	// MiniMax-M3 移除：2026-07-15 实测 HTTP 400（函数 id 在你额度/区域不可用）。
	// Mistral-Large-675B(NIM) 移除：2026-07-21 实测 30.3s，agent 场景过慢。
	{ID: "free_nim_mistralai_mistral_nemotron", Vendor: "NVIDIA NIM", Name: "Mistral-Nemotron", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "mistralai/mistral-nemotron", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 0, Note: "NIM 免费试用档"},
	{ID: "free_nim_mistralai_mistral_small_4_119b_2603", Vendor: "NVIDIA NIM", Name: "Mistral-Small-4-119B", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "mistralai/mistral-small-4-119b-2603", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 119, Note: "NIM 免费试用档"},
	{ID: "free_nim_nvidia_nemotron_3_ultra_550b_a55b", Vendor: "NVIDIA NIM", Name: "Nemotron-3-Ultra-550B", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "nvidia/nemotron-3-ultra-550b-a55b", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 550, Note: "NIM 免费试用档"},
	// Qwen3.5-397B(NIM) 移除：2026-07-21 实测 120s 超时无响应。
	{ID: "free_nim_sarvamai_sarvam_m", Vendor: "NVIDIA NIM", Name: "Sarvam-M", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "sarvamai/sarvam-m", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 0, Note: "NIM 免费试用档"},
	// GLM-5.2(NIM) 移除：2026-07-21 实测 40.4s，agent 场景过慢。

	// —— Ollama Cloud（走官方 OpenAI 兼容端点 /v1，带 CLOUD_API_KEY）——
	// 不走原生 /api/chat：那个端点的 JSON 解析器对 assistant.tool_calls 嵌套对象有 bug，
	// 历史里一带 tool_calls 就 400（"Value looks like object, but can't find closing '}'"），
	// 只能剥掉，模型因此看不见自己调用过工具，同一个工具会重复调到轮次上限。
	// /v1 走标准 OpenAI 解析器，实测同样的带 tool_calls 历史返回 200 且能正确引用工具结果。
	{ID: "free_ollama_cloud_gpt_oss_120b", Vendor: "Ollama Cloud", Name: "GPT-OSS 120B", Endpoint: "https://ollama.com/v1", Model: "gpt-oss:120b", KeyEnv: "CLOUD_API_KEY", ParamsB: 120, Note: "Ollama 官方云端（OpenAI 兼容端点，免费额度）", ContextWindow: 128000, Reasoning: true},

	// —— Cerebras 免费档（api.cerebras.ai）——
	{ID: "free_cerebras_gpt_oss_120b", Vendor: "Cerebras", Name: "gpt-oss-120b", Endpoint: "https://api.cerebras.ai/v1", Model: "gpt-oss-120b", KeyEnv: "CEREBRAS_API_KEY", ParamsB: 120, Note: "Cerebras 免费档"},
	{ID: "free_cerebras_zai_glm_4_7", Vendor: "Cerebras", Name: "zai-glm-4.7", Endpoint: "https://api.cerebras.ai/v1", Model: "zai-glm-4.7", KeyEnv: "CEREBRAS_API_KEY", ParamsB: 0, Note: "Cerebras 免费档"},

	// —— 阶跃星辰 StepFun（api.stepfun.com）——
	// 下面这几个都是拿 STEP_API_KEY 实调 /v1/models + /v1/chat/completions 逐个验证过的：
	// step-2x-large 在该 key 下返回「does not exist or you do not have access」，故未收录。
	// ContextWindow 一律留 0：/v1/models 只返回 id/created/owned_by，拿不到窗口大小，
	// 按本目录「未知者留 0，绝不伪造」的规矩不填。
	{ID: "free_step_3_7_flash", Vendor: "阶跃星辰 StepFun", Name: "step-3.7-flash", Endpoint: "https://api.stepfun.com/v1", Model: "step-3.7-flash", KeyEnv: "STEP_API_KEY", ParamsB: 0, Note: "阶跃星辰", Reasoning: true},
	{ID: "free_step_3_5_flash", Vendor: "阶跃星辰 StepFun", Name: "step-3.5-flash", Endpoint: "https://api.stepfun.com/v1", Model: "step-3.5-flash", KeyEnv: "STEP_API_KEY", ParamsB: 0, Note: "阶跃星辰", Reasoning: true},
	{ID: "free_step_1o_turbo_vision", Vendor: "阶跃星辰 StepFun", Name: "step-1o-turbo-vision", Endpoint: "https://api.stepfun.com/v1", Model: "step-1o-turbo-vision", KeyEnv: "STEP_API_KEY", ParamsB: 0, Note: "阶跃星辰（识图）", Vision: true},
	{ID: "free_step_router_v1", Vendor: "阶跃星辰 StepFun", Name: "step-router-v1", Endpoint: "https://api.stepfun.com/v1", Model: "step-router-v1", KeyEnv: "STEP_API_KEY", ParamsB: 0, Note: "阶跃星辰（自动选型）"},

	// —— 硅基流动 SiliconFlow（api.siliconflow.cn；代金券余额可用，对终端用户免费）——
	{ID: "free_sf_zai_org_glm_5_2", Vendor: "硅基流动 SiliconFlow", Name: "GLM-5.2", Endpoint: "https://api.siliconflow.cn/v1", Model: "zai-org/GLM-5.2", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）", Vision: true, ContextWindow: 128000, Reasoning: true},
	{ID: "free_sf_moonshotai_kimi_k2_7_code", Vendor: "硅基流动 SiliconFlow", Name: "Kimi-K2.7-Code", Endpoint: "https://api.siliconflow.cn/v1", Model: "moonshotai/Kimi-K2.7-Code", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
	// DeepSeek-V4-Pro(SiliconFlow) 移除：2026-07-21 实测 28.6s，agent 场景过慢。
	// DeepSeek-V4-Flash(SiliconFlow) 移除：2026-07-21 实测 120s 超时。
	{ID: "free_sf_pro_moonshotai_kimi_k2_6", Vendor: "硅基流动 SiliconFlow", Name: "Pro/Kimi-K2.6", Endpoint: "https://api.siliconflow.cn/v1", Model: "Pro/moonshotai/Kimi-K2.6", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）", Vision: true, ContextWindow: 128000},
	{ID: "free_sf_pro_zai_org_glm_5_1", Vendor: "硅基流动 SiliconFlow", Name: "Pro/GLM-5.1", Endpoint: "https://api.siliconflow.cn/v1", Model: "Pro/zai-org/GLM-5.1", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
	{ID: "free_sf_nex_agi_nex_n2_pro", Vendor: "硅基流动 SiliconFlow", Name: "Nex-N2-Pro", Endpoint: "https://api.siliconflow.cn/v1", Model: "nex-agi/Nex-N2-Pro", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
	{ID: "free_sf_minimaxai_minimax_m2_5", Vendor: "硅基流动 SiliconFlow", Name: "MiniMax-M2.5", Endpoint: "https://api.siliconflow.cn/v1", Model: "MiniMaxAI/MiniMax-M2.5", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
	{ID: "free_sf_pro_minimaxai_minimax_m2_5", Vendor: "硅基流动 SiliconFlow", Name: "Pro/MiniMax-M2.5", Endpoint: "https://api.siliconflow.cn/v1", Model: "Pro/MiniMaxAI/MiniMax-M2.5", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
	{ID: "free_sf_deepseek_ai_deepseek_v3_2", Vendor: "硅基流动 SiliconFlow", Name: "DeepSeek-V3.2", Endpoint: "https://api.siliconflow.cn/v1", Model: "deepseek-ai/DeepSeek-V3.2", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
	{ID: "free_sf_pro_deepseek_ai_deepseek_v3_2", Vendor: "硅基流动 SiliconFlow", Name: "Pro/DeepSeek-V3.2", Endpoint: "https://api.siliconflow.cn/v1", Model: "Pro/deepseek-ai/DeepSeek-V3.2", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
	{ID: "free_sf_deepseek_ai_deepseek_v3_1_terminus", Vendor: "硅基流动 SiliconFlow", Name: "DeepSeek-V3.1-Terminus", Endpoint: "https://api.siliconflow.cn/v1", Model: "deepseek-ai/DeepSeek-V3.1-Terminus", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
	{ID: "free_sf_pro_deepseek_ai_deepseek_v3_1_terminus", Vendor: "硅基流动 SiliconFlow", Name: "Pro/DeepSeek-V3.1-Terminus", Endpoint: "https://api.siliconflow.cn/v1", Model: "Pro/deepseek-ai/DeepSeek-V3.1-Terminus", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
	{ID: "free_sf_qwen_qwen3_5_397b_a17b", Vendor: "硅基流动 SiliconFlow", Name: "Qwen3.5-397B-A17B", Endpoint: "https://api.siliconflow.cn/v1", Model: "Qwen/Qwen3.5-397B-A17B", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 397, Note: "硅基流动（代金券）"},
	{ID: "free_sf_deepseek_ai_deepseek_r1", Vendor: "硅基流动 SiliconFlow", Name: "DeepSeek-R1", Endpoint: "https://api.siliconflow.cn/v1", Model: "deepseek-ai/DeepSeek-R1", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）", Reasoning: true},
}

func isFreeCatalogID(id string) bool {
	for _, f := range freeModelCatalog {
		if f.ID == id {
			return true
		}
	}
	return false
}

// resolveBackends 组装本次请求可用的路由链。
// 若 model 命中免费池 ID 或用户自定义配置 ID，则只返回那一个 backend（精确路由，
// 能力元数据随 backend 透出）；否则回退到"默认+参数降序"的全链（兼容旧行为）。
func resolveBackends(userKey string, model string) []RouterBackend {
	// —— 精确路由：前端明确选了某个模型 ——
	if model != "" && model != "auto" {
		if b := resolveExact(userKey, model); b != nil {
			return []RouterBackend{*b}
		}
	}

	var userChain, freeChain []RouterBackend

	entries, err := loadModelConfigs(userKey)
	entryByID := map[string]ModelConfigEntry{}
	if err == nil {
		for _, e := range entries {
			entryByID[e.ID] = e
		}
		// 1. 用户自定义配置（跳过免费池条目，它们下面单独走目录逻辑）
		for _, e := range entries {
			if e.APIKey == "" || isFreeCatalogID(e.ID) {
				continue
			}
			b := RouterBackend{
				Name: e.Name, BaseURL: e.Endpoint, Model: e.DefaultModel,
				APIKey: e.APIKey, Timeout: 5 * time.Minute, Source: "user",
				Vision: e.Vision, ContextWindow: e.ContextWindow, Reasoning: e.Reasoning,
			}
			if e.IsDefault {
				userChain = append([]RouterBackend{b}, userChain...)
			} else {
				userChain = append(userChain, b)
			}
		}
	}

	// 3. 免费池：Key 来源 = 用户保存的同 ID 条目 > 环境变量；没 Key 的源（Local/Ollama Cloud 走本地路由）直接不进链
	for _, f := range freeModelCatalog {
		if f.Disabled {
			continue
		}
		key := ""
		isDefault := false
		if e, ok := entryByID[f.ID]; ok {
			key = e.APIKey
			isDefault = e.IsDefault
		}
		if key == "" && !f.Local {
			key = os.Getenv(f.KeyEnv)
		}
		if key == "" && !f.Local {
			continue
		}
		source := "free"
		b := RouterBackend{
			Name: f.Name, BaseURL: f.Endpoint, Model: f.Model,
			APIKey: key, ParamsB: f.ParamsB, Timeout: 45 * time.Second, Source: source,
			Vision: f.Vision, ContextWindow: f.ContextWindow, Reasoning: f.Reasoning,
			IsLocal: f.Local,
		}
		if isDefault {
			// 用户显式把某个免费模型设为默认 → 提到链头
			userChain = append([]RouterBackend{b}, userChain...)
			continue
		}
		freeChain = append(freeChain, b)
	}
	// 参数规模降序，未知(0)排末
	sort.SliceStable(freeChain, func(i, j int) bool {
		wi, wj := freeChain[i].ParamsB, freeChain[j].ParamsB
		if wi == 0 {
			wi = -1
		}
		if wj == 0 {
			wj = -1
		}
		return wi > wj
	})

	out := userChain
	out = append(out, freeChain...)
	return out
}

// resolveExact 按模型 ID 精确解析出单个 backend（免费池或用户自定义配置）。
// 拿不到 Key 的源返回 nil（交给调用方回退全链）。
func resolveExact(userKey string, model string) *RouterBackend {
	entries, _ := loadModelConfigs(userKey)
	entryByID := map[string]ModelConfigEntry{}
	for _, e := range entries {
		entryByID[e.ID] = e
	}
	// 免费池
	for _, f := range freeModelCatalog {
		if f.Disabled {
			continue
		}
		if f.ID != model {
			continue
		}
		key := ""
		if !f.Local {
			if e, ok := entryByID[f.ID]; ok {
				key = e.APIKey
			}
			if key == "" {
				key = os.Getenv(f.KeyEnv)
			}
		}
		if key == "" && !f.Local {
			return nil
		}
		source := "free"
		return &RouterBackend{
			Name: f.Name, BaseURL: f.Endpoint, Model: f.Model,
			APIKey: key, ParamsB: f.ParamsB, Timeout: 45 * time.Second, Source: source,
			Vision: f.Vision, ContextWindow: f.ContextWindow, Reasoning: f.Reasoning,
			IsLocal: f.Local,
		}
	}
	// 用户自定义配置
	for _, e := range entries {
		if e.ID != model || isFreeCatalogID(e.ID) {
			continue
		}
		if e.APIKey == "" {
			return nil
		}
		return &RouterBackend{
			Name: e.Name, BaseURL: e.Endpoint, Model: e.DefaultModel,
			APIKey: e.APIKey, Timeout: 5 * time.Minute, Source: "user",
			Vision: e.Vision, ContextWindow: e.ContextWindow, Reasoning: e.Reasoning,
		}
	}
	return nil
}

// chatCompletionsURL 归一化 endpoint：允许用户填根地址或带 /v1 的地址。
func chatCompletionsURL(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

// ==================== 非流式：openAIChatOnce + 秒切链 ====================

// openAIChatOnce 对单个 backend 发一次非流式 OpenAI 兼容调用。
func openAIChatOnce(ctx context.Context, b RouterBackend, msgs []map[string]any, tools []map[string]any) (string, []core.ToolCall, error) {
	reqBody := map[string]any{
		"model": b.Model, "messages": msgs, "stream": false,
		"temperature": 0.2, "top_p": 0.85, "max_tokens": 4096,
	}
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}
	body, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", chatCompletionsURL(b.BaseURL), bytes.NewBuffer(body))
	if err != nil {
		return "", nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if b.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+b.APIKey)
	}

	client := &http.Client{Timeout: b.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		// 仅 401(鉴权)/403(额度) 确定性不可用才标记禁用；400 属请求格式/上游解析 bug，不禁用
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			disableFreeModel(b.Model)
		}
		return "", nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateChars(string(raw), 300))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", nil, fmt.Errorf("响应解析失败: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", nil, fmt.Errorf("empty choices")
	}
	msg := parsed.Choices[0].Message
	var calls []core.ToolCall
	for _, tc := range msg.ToolCalls {
		call := core.ToolCall{ID: tc.ID, Type: "function"}
		call.Function.Name = tc.Function.Name
		call.Function.Arguments = tc.Function.Arguments
		calls = append(calls, call)
	}
	if msg.Content == "" && len(calls) == 0 {
		return "", nil, fmt.Errorf("empty completion")
	}
	return msg.Content, calls, nil
}

// routeChatOnce 沿路由链做非流式调用：失败秒切下一个，全失败才报错。
func routeChatOnce(ctx context.Context, backends []RouterBackend, msgs []map[string]any, tools []map[string]any) (string, []core.ToolCall, error) {
	var tried []string
	for _, b := range backends {
		if ctx.Err() != nil {
			return "", nil, ctx.Err()
		}
		content, calls, err := openAIChatOnce(ctx, b, msgs, tools)
		if err != nil {
			tried = append(tried, fmt.Sprintf("%s: %v", b.Name, err))
			fmt.Printf("🔀 [路由] %s 失败，秒切下一个: %v\n", b.Name, truncateChars(err.Error(), 120))
			continue
		}
		if len(tried) > 0 {
			fmt.Printf("🔀 [路由] 最终由 %s 承接（此前 %d 个源失败）\n", b.Name, len(tried))
		}
		return content, calls, nil
	}
	return "", nil, fmt.Errorf("所有模型源不可用：%s", strings.Join(tried, "；"))
}

// ==================== 流式：首包前 failover ====================

// streamHTTPClient 返回用于流式请求的 *http.Client。
// 关键：流式响应不能用 Client.Timeout 整体计时——Go 的 Timeout 把"读取整个响应体"也算进窗口，
// 免费档/慢源一次生成常常 > 45s，会被 Client.Timeout 在流读到一半时砍断，报
// "context deadline exceeded (Client.Timeout or context cancellation while reading body)"。
// 故流式 client Timeout 置 0，只由 Transport 卡"连接 + 首字节"(ResponseHeaderTimeout)，
// 真正的取消交给请求上下文 c.Request.Context()（浏览器断开即取消）。
func streamHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ResponseHeaderTimeout: 30 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
		},
	}
}

// streamRouterRound 沿路由链做流式调用。failover 只发生在拿到 200 响应之前
// （连接失败/非200 秒切下一个）；流一旦开始就不再切换源。
// 实时把 reasoning_content/content 增量写成 thinking/intent SSE 事件。
// 返回值里带上实际承接这轮请求的 backend（而不只是个名字字符串），前端要靠它
// 拿到 vision/context_window/reasoning 这些能力元数据，决定要不要开放识图之类的功能。
func (r *WorkflowRunner) streamRouterRound(c *gin.Context, backends []RouterBackend, msgs []map[string]any, tools []map[string]any, effort string) (string, []core.ToolCall, int, int, *RouterBackend, error) {
	// 空链是真实可能的：本地兜底已于 8186699e 移除，一个 Key 都没配时链就是空的。
	// 不给这条单独的错误信息的话，用户看到的是 "所有模型源不可用：" 后面跟一片空白。
	if len(backends) == 0 {
		return "", nil, 0, 0, nil, fmt.Errorf("没有可用的模型源：请在设置面板填入至少一个 API Key，或配置环境变量")
	}
	var tried []string
	for _, b := range backends {
		if c.Request.Context().Err() != nil {
			return "", nil, 0, 0, nil, c.Request.Context().Err()
		}
		reqBody := map[string]any{
			"model": b.Model, "messages": msgs, "stream": true,
			"temperature": 0.2, "top_p": 0.85, "max_tokens": 4096,
			// 请求上游回传 usage（prompt/completion tokens）——绝大多数 OpenAI 兼容免费源支持，
			// 不影响计费，仅让前端 context 横条显示真实值而非纯字符/4 估算。
			"stream_options": map[string]any{"include_usage": true},
		}
		// 只有前端选的这个 backend 真支持思考强度时才带这个字段——不支持的源
		// 收到未知字段大概率报错，而不是安静忽略，不能无脑塞给所有 backend
		if effort != "" && b.Reasoning {
			reqBody["reasoning_effort"] = effort
		}
		if len(tools) > 0 {
			reqBody["tools"] = tools
		}
		body, _ := json.Marshal(reqBody)
		httpReq, err := http.NewRequestWithContext(c.Request.Context(), "POST", chatCompletionsURL(b.BaseURL), bytes.NewBuffer(body))
		if err != nil {
			tried = append(tried, fmt.Sprintf("%s: %v", b.Name, err))
			continue
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if b.APIKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+b.APIKey)
		}

		client := streamHTTPClient()
		resp, err := client.Do(httpReq)
		if err != nil {
			tried = append(tried, fmt.Sprintf("%s: %v", b.Name, err))
			fmt.Printf("🔀 [路由] %s 连接失败，秒切下一个\n", b.Name)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			// 仅 401(鉴权)/403(额度) 是确定性不可用，当场标记禁用；
			// 400 是请求格式/上游解析问题，属客户端侧，不该永久禁用模型，
			// 否则一棍子打死整个免费档。
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				disableFreeModel(b.Model)
			}
			tried = append(tried, fmt.Sprintf("%s: HTTP %d", b.Name, resp.StatusCode))
			fmt.Printf("🔀 [路由] %s HTTP %d，秒切下一个: %s\n", b.Name, resp.StatusCode, truncateChars(string(raw), 120))
			continue
		}

		if len(tried) > 0 {
			fmt.Printf("🔀 [路由] 流式请求由 %s 承接（此前 %d 个源失败）\n", b.Name, len(tried))
		}
		content, calls, inTok, outTok, err := drainChatStream(c, resp)
		resp.Body.Close()
		usedBackend := b
		return content, calls, inTok, outTok, &usedBackend, err
	}
	return "", nil, 0, 0, nil, fmt.Errorf("所有模型源不可用：%s", strings.Join(tried, "；"))
}

// drainChatStream 读一条已建立的 SSE 流，实时转发 thinking/intent 事件。
// 返回真实拆分的 inputTokens/outputTokens：优先取上游 usage.prompt_tokens/completion_tokens，
// 上游不回传时退化为字符/4 估算（与四态机历史口径一致）。
func drainChatStream(c *gin.Context, resp *http.Response) (string, []core.ToolCall, int, int, error) {
	reader := bufio.NewReader(resp.Body)
	var full strings.Builder
	charCount := 0
	callsMap := map[int]*core.ToolCall{}
	// 真实 usage：上游在最后一个空 choices chunk 里回传（stream_options.include_usage）
	var inTok, outTok int
	gotUsage := false

	for {
		line, rerr := reader.ReadString('\n')
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return "", nil, 0, 0, fmt.Errorf("读取流失败: %w", rerr)
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var ev map[string]any
		if json.Unmarshal([]byte(data), &ev) != nil {
			continue
		}
		choices, _ := ev["choices"].([]any)
		if len(choices) == 0 {
			// 无 choices：可能是 usage chunk（stream_options.include_usage）
			if usage, ok := ev["usage"].(map[string]any); ok {
				if pt, ok := usage["prompt_tokens"].(float64); ok {
					inTok = int(pt)
				}
				if ct, ok := usage["completion_tokens"].(float64); ok {
					outTok = int(ct)
				}
				gotUsage = true
			}
			continue
		}
		choice, _ := choices[0].(map[string]any)
		delta, _ := choice["delta"].(map[string]any)
		if delta == nil {
			continue
		}

		if rc, ok := delta["reasoning_content"].(string); ok && rc != "" {
			charCount += len(rc)
			writeCodeSSE(c, "thinking", map[string]any{"content": rc})
		}
		if ct, ok := delta["content"].(string); ok && ct != "" {
			charCount += len(ct)
			full.WriteString(ct)
			writeCodeSSE(c, "intent", map[string]any{"content": ct})
		}
		if rawCalls, ok := delta["tool_calls"].([]any); ok {
			for _, rawCall := range rawCalls {
				callMap, _ := rawCall.(map[string]any)
				idxFloat, hasIdx := callMap["index"].(float64)
				if !hasIdx {
					continue
				}
				idx := int(idxFloat)
				if _, exists := callsMap[idx]; !exists {
					callsMap[idx] = &core.ToolCall{Type: "function"}
				}
				tc := callsMap[idx]
				if id, ok := callMap["id"].(string); ok && id != "" {
					tc.ID = id
				}
				if fnMap, ok := callMap["function"].(map[string]any); ok {
					if name, ok := fnMap["name"].(string); ok && name != "" {
						tc.Function.Name = name
					}
					if argsStr, ok := fnMap["arguments"].(string); ok {
						tc.Function.Arguments += argsStr
					}
				}
			}
		}
	}

	var calls []core.ToolCall
	for i := 0; i < len(callsMap); i++ {
		if tc, ok := callsMap[i]; ok && tc.Function.Name != "" {
			calls = append(calls, *tc)
		}
	}
	// 上游没回传 usage：用字符/4 估算兜底（与四态机 input 估算口径同源）
	if !gotUsage {
		outTok = charCount / 4
	}
	return full.String(), calls, inTok, outTok, nil
}
