package handler

// AAP 免费模型路由层（aap/agent/inference.py InferenceRouter）的 Go 移植。
//
// 路由链（按优先级）：
//  1. 用户自定义配置（设置面板填的 Key，默认条目排最前）—— ~/.Aurora/user_configs/{openid}.json
//  2. DEEPSEEK_API_KEY 环境变量（旧部署兼容，付费档）
//  3. 免费模型池（参数规模降序，未知参数量排末，绝不伪造数字）
//  4. 本地 Ollama 兜底（离线 $0，傻但永不掉线）
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
	Vision        bool  `json:"vision"`
	ContextWindow int   `json:"context_window"`
	Reasoning     bool  `json:"reasoning"`
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
	// 不需要 API Key，复用现有 OpenAI 兼容链；与 localLLMBackend() 兜底共享同一 base。
	Local bool `json:"local"`
	// CloudNative=true 表示走 Ollama 官方云端 API（https://ollama.com/api/chat，Ollama 原生
	// 格式，非 OpenAI 兼容），用 CLOUD_API_KEY；实测 gpt-oss:120b 可跑且原生支持 tool_calls。
	CloudNative bool `json:"cloud_native"`
	// Disabled=true 表示该模型被运行时探测判定为不可用（如提供方退役/下架），
	// 路由链与精确解析均跳过；由 nimRefresh 每日探测后动态置位。
	Disabled bool `json:"disabled"`
}

// 参数规模是公开估计值；未知者写 0，排序时排免费池末段，绝不伪造。
// Vendor 字段用于前端按厂商折叠分组（仿 Hermes 提供方分类）。
var freeModelCatalog = []FreeModelDef{
	// —— OpenRouter 已整体移除：免费档全部 slug 限流 429（连 llama-3.3-70b/405b 都 429），
	// 无专属免费模型，作为网关接入价值为零，徒增链尾失败噪声。2026-07-15 实测确认。

	// —— Google AI Studio 免费档（Gemini flash，实测 2026-07-12 通过；几百B MoE）——
	{ID: "free_google_gemini_2_5_flash", Vendor: "Google", Name: "Gemini 2.5 Flash", Endpoint: "https://generativelanguage.googleapis.com/v1beta/openai", Model: "gemini-2.5-flash", KeyEnv: "GOOGLE_API_KEY", ParamsB: 0, Note: "Google AI Studio 免费档（每日限流，路由自动 failover）", Vision: true, ContextWindow: 1048576, Reasoning: true},
	{ID: "free_google_gemini_3_flash_preview", Vendor: "Google", Name: "Gemini 3 Flash Preview", Endpoint: "https://generativelanguage.googleapis.com/v1beta/openai", Model: "gemini-3-flash-preview", KeyEnv: "GOOGLE_API_KEY", ParamsB: 0, Note: "Google AI Studio 免费档（每日限流）", Vision: true, ContextWindow: 1048576, Reasoning: true},
	{ID: "free_google_gemini_flash_latest", Vendor: "Google", Name: "Gemini Flash Latest", Endpoint: "https://generativelanguage.googleapis.com/v1beta/openai", Model: "gemini-flash-latest", KeyEnv: "GOOGLE_API_KEY", ParamsB: 0, Note: "Google AI Studio 免费档（每日限流）", Vision: true, ContextWindow: 1048576, Reasoning: true},

	// —— NVIDIA NIM 免费试用档（integrate.api.nvidia.com，实测可完成对话；已剔除 PII/内容安全等非对话模型）——
	{ID: "free_nim_abacusai_dracarys_llama_3_1_70b_instruct", Vendor: "NVIDIA NIM", Name: "Dracarys-Llama-3.1-70B", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "abacusai/dracarys-llama-3.1-70b-instruct", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 70, Note: "NIM 免费试用档"},
	{ID: "free_nim_deepseek_ai_deepseek_v4_flash", Vendor: "NVIDIA NIM", Name: "DeepSeek-V4-Flash", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "deepseek-ai/deepseek-v4-flash", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 0, Note: "NIM 免费试用档"},
	{ID: "free_nim_deepseek_ai_deepseek_v4_pro", Vendor: "NVIDIA NIM", Name: "DeepSeek-V4-Pro", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "deepseek-ai/deepseek-v4-pro", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 0, Note: "NIM 免费试用档"},
	{ID: "free_nim_meta_llama_3_1_70b_instruct", Vendor: "NVIDIA NIM", Name: "Llama-3.1-70B", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "meta/llama-3.1-70b-instruct", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 70, Note: "NIM 免费试用档"},
	{ID: "free_nim_meta_llama_3_3_70b_instruct", Vendor: "NVIDIA NIM", Name: "Llama-3.3-70B", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "meta/llama-3.3-70b-instruct", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 70, Note: "NIM 免费试用档"},
	// Phi-4-mini 移除：2026-07-15 实测 HTTP 410 Gone（NIM 已永久下架）。
	// Stockmark-2-100B 移除：2026-07-15 实测 HTTP 410 Gone（NIM 已永久下架）。
	// MiniMax-M3 移除：2026-07-15 实测 HTTP 400（函数 id 在你额度/区域不可用）。
	{ID: "free_nim_mistralai_mistral_large_3_675b_instruct_2512", Vendor: "NVIDIA NIM", Name: "Mistral-Large-3-675B", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "mistralai/mistral-large-3-675b-instruct-2512", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 675, Note: "NIM 免费试用档"},
	{ID: "free_nim_mistralai_mistral_nemotron", Vendor: "NVIDIA NIM", Name: "Mistral-Nemotron", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "mistralai/mistral-nemotron", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 0, Note: "NIM 免费试用档"},
	{ID: "free_nim_mistralai_mistral_small_4_119b_2603", Vendor: "NVIDIA NIM", Name: "Mistral-Small-4-119B", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "mistralai/mistral-small-4-119b-2603", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 119, Note: "NIM 免费试用档"},
	{ID: "free_nim_nvidia_nemotron_3_ultra_550b_a55b", Vendor: "NVIDIA NIM", Name: "Nemotron-3-Ultra-550B", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "nvidia/nemotron-3-ultra-550b-a55b", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 550, Note: "NIM 免费试用档"},
	{ID: "free_nim_qwen_qwen3_5_397b_a17b", Vendor: "NVIDIA NIM", Name: "Qwen3.5-397B-A17B", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "qwen/qwen3.5-397b-a17b", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 397, Note: "NIM 免费试用档"},
	{ID: "free_nim_sarvamai_sarvam_m", Vendor: "NVIDIA NIM", Name: "Sarvam-M", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "sarvamai/sarvam-m", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 0, Note: "NIM 免费试用档"},
	{ID: "free_nim_z_ai_glm_5_2", Vendor: "NVIDIA NIM", Name: "GLM-5.2 (NIM)", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "z-ai/glm-5.2", KeyEnv: "NVIDIA_NIM_API_KEY", ParamsB: 0, Note: "NIM 免费试用档"},

	// —— Ollama Cloud（官方云端 API，ollama.com/api/chat 原生格式，带 CLOUD_API_KEY；
	// 实测 gpt-oss:120b 可跑且原生支持 tool_calls）——
	{ID: "free_ollama_cloud_gpt_oss_120b", Vendor: "Ollama Cloud", Name: "GPT-OSS 120B", Endpoint: "https://ollama.com/api/chat", Model: "gpt-oss:120b", KeyEnv: "CLOUD_API_KEY", ParamsB: 120, CloudNative: true, Note: "Ollama 官方云端 API（免费额度，原生 tool_calls）", ContextWindow: 128000, Reasoning: true},

	// —— Cerebras 免费档（api.cerebras.ai）——
	{ID: "free_cerebras_gpt_oss_120b", Vendor: "Cerebras", Name: "gpt-oss-120b", Endpoint: "https://api.cerebras.ai/v1", Model: "gpt-oss-120b", KeyEnv: "CEREBRAS_API_KEY", ParamsB: 120, Note: "Cerebras 免费档"},
	{ID: "free_cerebras_zai_glm_4_7", Vendor: "Cerebras", Name: "zai-glm-4.7", Endpoint: "https://api.cerebras.ai/v1", Model: "zai-glm-4.7", KeyEnv: "CEREBRAS_API_KEY", ParamsB: 0, Note: "Cerebras 免费档"},

	// —— 硅基流动 SiliconFlow（api.siliconflow.cn；代金券余额可用，对终端用户免费）——
	{ID: "free_sf_zai_org_glm_5_2", Vendor: "硅基流动 SiliconFlow", Name: "GLM-5.2", Endpoint: "https://api.siliconflow.cn/v1", Model: "zai-org/GLM-5.2", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）", Vision: true, ContextWindow: 128000, Reasoning: true},
	{ID: "free_sf_moonshotai_kimi_k2_7_code", Vendor: "硅基流动 SiliconFlow", Name: "Kimi-K2.7-Code", Endpoint: "https://api.siliconflow.cn/v1", Model: "moonshotai/Kimi-K2.7-Code", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
	{ID: "free_sf_deepseek_ai_deepseek_v4_pro", Vendor: "硅基流动 SiliconFlow", Name: "DeepSeek-V4-Pro", Endpoint: "https://api.siliconflow.cn/v1", Model: "deepseek-ai/DeepSeek-V4-Pro", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
	{ID: "free_sf_deepseek_ai_deepseek_v4_flash", Vendor: "硅基流动 SiliconFlow", Name: "DeepSeek-V4-Flash", Endpoint: "https://api.siliconflow.cn/v1", Model: "deepseek-ai/DeepSeek-V4-Flash", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
	{ID: "free_sf_pro_moonshotai_kimi_k2_6", Vendor: "硅基流动 SiliconFlow", Name: "Pro/Kimi-K2.6", Endpoint: "https://api.siliconflow.cn/v1", Model: "Pro/moonshotai/Kimi-K2.6", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动（代金券）"},
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

func localLLMBackend() RouterBackend {
	base := os.Getenv("LOCAL_OPENAI_BASE")
	if base == "" {
		base = "http://localhost:11434/v1"
	}
	model := os.Getenv("LOCAL_MODEL")
	if model == "" {
		model = "qwen2.5-coder:7b"
	}
	return RouterBackend{Name: "本地 " + model, BaseURL: base, Model: model, IsLocal: true, Timeout: 3 * time.Minute, Source: "local"}
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
		if key == "" {
			if f.CloudNative {
				key = os.Getenv("CLOUD_API_KEY")
			} else if !f.Local {
				key = os.Getenv(f.KeyEnv)
			}
		}
		if key == "" && !f.Local && !f.CloudNative {
			continue
		}
		source := "free"
		if f.CloudNative {
			source = "ollama-cloud"
		}
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

	// 2. 旧部署兼容：环境变量里的 DeepSeek（付费档，排在用户配置之后、免费池之前）
	if k := os.Getenv("DEEPSEEK_API_KEY"); k != "" {
		model := os.Getenv("DEEPSEEK_MODEL")
		if model == "" {
			model = "deepseek-v4-flash"
		}
		out = append(out, RouterBackend{
			Name: "DeepSeek(env)", BaseURL: "https://api.deepseek.com", Model: model,
			APIKey: k, ParamsB: 236, Timeout: 5 * time.Minute, Source: "env",
		})
	}

	out = append(out, freeChain...)
	// 4. 本地兜底恒排最后（不被参数数字欺骗）
	out = append(out, localLLMBackend())
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
		if f.CloudNative {
			key = os.Getenv("CLOUD_API_KEY")
		} else if !f.Local {
			if e, ok := entryByID[f.ID]; ok {
				key = e.APIKey
			}
			if key == "" {
				key = os.Getenv(f.KeyEnv)
			}
		}
		if key == "" && !f.Local && !f.CloudNative {
			return nil
		}
		source := "free"
		if f.CloudNative {
			source = "ollama-cloud"
		}
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
		// 400/401/403 是确定性不可用（模型下架/额度/鉴权），当场标记，后续不再选
		if resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 {
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

// streamRouterRound 沿路由链做流式调用。failover 只发生在拿到 200 响应之前
// （连接失败/非200 秒切下一个）；流一旦开始就不再切换源。
// 实时把 reasoning_content/content 增量写成 thinking/intent SSE 事件。
// 返回值里带上实际承接这轮请求的 backend（而不只是个名字字符串），前端要靠它
// 拿到 vision/context_window/reasoning 这些能力元数据，决定要不要开放识图之类的功能。
func (r *WorkflowRunner) streamRouterRound(c *gin.Context, backends []RouterBackend, msgs []map[string]any, tools []map[string]any, effort string) (string, []core.ToolCall, int, *RouterBackend, error) {
	var tried []string
	for _, b := range backends {
		if c.Request.Context().Err() != nil {
			return "", nil, 0, nil, c.Request.Context().Err()
		}
		// Ollama 官方云端 API 是原生格式（非 OpenAI 兼容），单独走流式分支
		if b.Source == "ollama-cloud" {
			return r.ollamaCloudStreamRound(c, b, msgs, tools, effort)
		}
		reqBody := map[string]any{
			"model": b.Model, "messages": msgs, "stream": true,
			"temperature": 0.2, "top_p": 0.85, "max_tokens": 4096,
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

		client := &http.Client{Timeout: b.Timeout}
		resp, err := client.Do(httpReq)
		if err != nil {
			tried = append(tried, fmt.Sprintf("%s: %v", b.Name, err))
			fmt.Printf("🔀 [路由] %s 连接失败，秒切下一个\n", b.Name)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			// 400/401/403 确定性不可用，当场标记
			if resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 {
				disableFreeModel(b.Model)
			}
			tried = append(tried, fmt.Sprintf("%s: HTTP %d", b.Name, resp.StatusCode))
			fmt.Printf("🔀 [路由] %s HTTP %d，秒切下一个: %s\n", b.Name, resp.StatusCode, truncateChars(string(raw), 120))
			continue
		}

		if len(tried) > 0 {
			fmt.Printf("🔀 [路由] 流式请求由 %s 承接（此前 %d 个源失败）\n", b.Name, len(tried))
		}
		content, calls, outTok, err := drainChatStream(c, resp)
		resp.Body.Close()
		usedBackend := b
		return content, calls, outTok, &usedBackend, err
	}
	return "", nil, 0, nil, fmt.Errorf("所有模型源不可用：%s", strings.Join(tried, "；"))
}

// ollamaCloudStreamRound 走 Ollama 官方云端 API（ollama.com/api/chat，原生格式）。
// 与 OpenAI 兼容链的区别：请求体同构（model/messages/stream/tools），但响应是
// 每行一个独立 JSON 对象（非 data: 前缀 SSE），工具调用在 message.tool_calls
// 顶层、arguments 是对象（需转 JSON 字符串喂给 core.ToolCall）。
func (r *WorkflowRunner) ollamaCloudStreamRound(c *gin.Context, b RouterBackend, msgs []map[string]any, tools []map[string]any, effort string) (string, []core.ToolCall, int, *RouterBackend, error) {
	reqBody := map[string]any{
		"model": b.Model, "messages": msgs, "stream": true,
		"options": map[string]any{"temperature": 0.2, "top_p": 0.85},
	}
	// Ollama 原生 API 用 "think" 字段控制推理强度（bool 或 "low"/"medium"/"high"，
	// 取决于模型是否支持分级——直接透传字符串，模型不支持就当无效字段忽略）
	if effort != "" && b.Reasoning {
		reqBody["think"] = effort
	}
	if len(tools) > 0 {
		reqBody["tools"] = tools
	}
	body, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), "POST", b.BaseURL, bytes.NewBuffer(body))
	if err != nil {
		return "", nil, 0, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if b.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+b.APIKey)
	}
	client := &http.Client{Timeout: b.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", nil, 0, nil, fmt.Errorf("Ollama Cloud 连接失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return "", nil, 0, nil, fmt.Errorf("Ollama Cloud HTTP %d: %s", resp.StatusCode, truncateChars(string(raw), 300))
	}
	content, calls, outTok, err := drainOllamaCloudStream(c, resp)
	resp.Body.Close()
	used := b
	return content, calls, outTok, &used, err
}

// drainOllamaCloudStream 读 Ollama 原生流（每行一个 JSON 对象），实时转发
// thinking/intent 事件，并把原生 tool_calls 聚合成 core.ToolCall 数组。
func drainOllamaCloudStream(c *gin.Context, resp *http.Response) (string, []core.ToolCall, int, error) {
	reader := bufio.NewReader(resp.Body)
	var full strings.Builder
	charCount := 0
	callsMap := map[int]*core.ToolCall{}

	for {
		line, rerr := reader.ReadString('\n')
		if rerr != nil && rerr != io.EOF {
			if full.Len() > 0 {
				break
			}
			return "", nil, 0, fmt.Errorf("读取 Ollama Cloud 流失败: %w", rerr)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if rerr == io.EOF {
				break
			}
			continue
		}
		var obj struct {
			Message struct {
				Content   string `json:"content"`
				Thinking  string `json:"thinking"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments any    `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			Done bool `json:"done"`
		}
		if json.Unmarshal([]byte(line), &obj) != nil {
			if rerr == io.EOF {
				break
			}
			continue
		}

		if obj.Message.Thinking != "" {
			charCount += len(obj.Message.Thinking)
			writeCodeSSE(c, "thinking", map[string]any{"content": obj.Message.Thinking})
		}
		if obj.Message.Content != "" {
			charCount += len(obj.Message.Content)
			full.WriteString(obj.Message.Content)
			writeCodeSSE(c, "intent", map[string]any{"content": obj.Message.Content})
		}
		for i, tc := range obj.Message.ToolCalls {
			ct := &core.ToolCall{Type: "function", ID: tc.ID}
			ct.Function.Name = tc.Function.Name
			if tc.Function.Arguments != nil {
				if s, ok := tc.Function.Arguments.(string); ok {
					ct.Function.Arguments = s
				} else if b, err := json.Marshal(tc.Function.Arguments); err == nil {
					ct.Function.Arguments = string(b)
				}
			}
			callsMap[i] = ct
		}

		if obj.Done {
			break
		}
		if rerr == io.EOF {
			break
		}
	}

	var calls []core.ToolCall
	for i := 0; i < len(callsMap); i++ {
		if tc, ok := callsMap[i]; ok {
			calls = append(calls, *tc)
		}
	}
	return full.String(), calls, charCount, nil
}

// drainChatStream 读一条已建立的 SSE 流，实时转发 thinking/intent 事件。
func drainChatStream(c *gin.Context, resp *http.Response) (string, []core.ToolCall, int, error) {
	reader := bufio.NewReader(resp.Body)
	var full strings.Builder
	charCount := 0
	callsMap := map[int]*core.ToolCall{}

	for {
		line, rerr := reader.ReadString('\n')
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return "", nil, 0, fmt.Errorf("读取流失败: %w", rerr)
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
	return full.String(), calls, charCount / 4, nil
}
