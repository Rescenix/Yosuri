package main

// router.go — Rescene 免费模型路由聚合层
// 直接从 re0 的 freeModelCatalog 移植，内置所有免费模型端点
// 支持自动 failover：一个挂了秒切下一个

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
	"sync"
	"time"
)

// FreeModel 免费模型条目
type FreeModel struct {
	ID        string // 唯一标识，如 free_zen_deepseek_v4_flash
	Vendor    string
	Name      string
	Endpoint  string // 如 https://opencode.ai/zen/v1
	Model     string // 上游模型名，如 deepseek-v4-flash-free
	KeyEnv    string // 环境变量名，空=免 key
	KeyURL    string // 申请 Key 的页面
	Keyless   bool   // 免 key 网关
	Vision    bool
	Reasoning bool
	CtxWindow int
	ParamsB   float64 // 参数规模
}

// 免费模型目录 — 直接从 re0 的 freeModelCatalog 同步
var freeModels = []FreeModel{
	// —— OpenCode Zen 免 key 网关 ——
	{ID: "free_zen_deepseek_v4_flash", Vendor: "OpenCode Zen", Name: "DeepSeek V4 Flash（免费）", Endpoint: "https://opencode.ai/zen/v1", Model: "deepseek-v4-flash-free", Keyless: true, Reasoning: true},
	{ID: "free_zen_mimo_v2_5", Vendor: "OpenCode Zen", Name: "Mimo 2.5（免费）", Endpoint: "https://opencode.ai/zen/v1", Model: "mimo-v2.5-free", Keyless: true, Reasoning: true},
	{ID: "free_zen_north_mini_code", Vendor: "OpenCode Zen", Name: "North Mini Code（免费·最快）", Endpoint: "https://opencode.ai/zen/v1", Model: "north-mini-code-free", Keyless: true, Reasoning: true},
	{ID: "free_zen_nemotron_3_ultra", Vendor: "OpenCode Zen", Name: "Nemotron 3 Ultra（免费）", Endpoint: "https://opencode.ai/zen/v1", Model: "nemotron-3-ultra-free", Keyless: true, Reasoning: true},
	// 2026-08-08 重新爬取 Zen /v1/models（61 个模型，8 个 free 档）实测新增：
	// longcat-2.0-free（全新，✅ 实测可用）、laguna-s-2.1-free（✅ 恢复可用，曾被限流淘汰）、
	// ling-3.0-tiny-free / ling-3.0-flash-free（❌ 上游 Endpoint unavailable，未收录）
	{ID: "free_zen_longcat_2_0", Vendor: "OpenCode Zen", Name: "Longcat 2.0（免费·新增）", Endpoint: "https://opencode.ai/zen/v1", Model: "longcat-2.0-free", Keyless: true, Reasoning: true},
	{ID: "free_zen_laguna_s_2_1", Vendor: "OpenCode Zen", Name: "Laguna S 2.1（免费·恢复）", Endpoint: "https://opencode.ai/zen/v1", Model: "laguna-s-2.1-free", Keyless: true, Reasoning: true},

	// —— LLM7.io 免 key 网关 ——
	// 2026-08-22 实测无 Authorization 可调用；6 个 Turbo 聊天模型均声明支持 tools，
	// 其中 5 个已真实返回 tool_calls，DeepSeek 当时为上游临时繁忙。
	{ID: "free_llm7_deepseek_v4_flash", Vendor: "LLM7.io", Name: "DeepSeek V4 Flash（LLM7·免 key）", Endpoint: "https://api.llm7.io/v1", Model: "DeepSeek-V4-Flash-0731", Keyless: true, Reasoning: true, CtxWindow: 400000},
	{ID: "free_llm7_codestral", Vendor: "LLM7.io", Name: "Codestral（LLM7·免 key）", Endpoint: "https://api.llm7.io/v1", Model: "codestral-latest", Keyless: true, CtxWindow: 32000},
	{ID: "free_llm7_gemini_flash_lite", Vendor: "LLM7.io", Name: "Gemini 3.1 Flash Lite（LLM7·免 key）", Endpoint: "https://api.llm7.io/v1", Model: "gemini-3.1-flash-lite", Keyless: true, Vision: true, CtxWindow: 256000},
	{ID: "free_llm7_gpt_oss_20b", Vendor: "LLM7.io", Name: "GPT-OSS 20B（LLM7·免 key）", Endpoint: "https://api.llm7.io/v1", Model: "gpt-oss:20b", Keyless: true, ParamsB: 20, CtxWindow: 128000},
	{ID: "free_llm7_llama_3_1_8b", Vendor: "LLM7.io", Name: "Llama 3.1 8B Turbo（LLM7·免 key）", Endpoint: "https://api.llm7.io/v1", Model: "meta-Llama-3.1-8B-Instruct-Turbo", Keyless: true, ParamsB: 8, CtxWindow: 128000},
	{ID: "free_llm7_minimax_m2_7", Vendor: "LLM7.io", Name: "MiniMax M2.7（LLM7·免 key）", Endpoint: "https://api.llm7.io/v1", Model: "minimax-m2.7", Keyless: true, Reasoning: true, CtxWindow: 180000},

	// —— Google AI Studio（需免费 API Key；大陆直连可能超时）——
	{ID: "free_google_gemini_2_5_flash", Vendor: "Google AI Studio", Name: "Gemini 2.5 Flash（Google·免费额度）", Endpoint: "https://generativelanguage.googleapis.com/v1beta/openai", Model: "gemini-2.5-flash", KeyEnv: "GEMINI_API_KEY", Vision: true, Reasoning: true, CtxWindow: 1048576, KeyURL: "https://aistudio.google.com/apikey"},

	// —— OpenRouter（需免费 API Key；openrouter/free 自动路由 20+ 免费模型）——
	{ID: "free_openrouter_router", Vendor: "OpenRouter", Name: "OpenRouter Free Router（免费）", Endpoint: "https://openrouter.ai/api/v1", Model: "openrouter/free", KeyEnv: "OPENROUTER_API_KEY", Vision: true, Reasoning: true, CtxWindow: 200000, KeyURL: "https://openrouter.ai/settings/keys"},

	// —— Kilo Gateway（api.kilo.ai/api/gateway）——
	// 2026-08-08 发现：完全免 key 的 OpenAI 兼容网关，349 个模型，200 RPH 免费额度！
	// 实测 6 个 :free 模型稳定可用（含腾讯混元 hy3 / 阶跃 step-3.7 / NVIDIA Nemotron 系列），
	// 其余 :free（laguna-xs-2.1 429、north-mini-code 限流、nex-n2-pro 转付费、ling-2.6 转付费）未收录。
	// 需要浏览器 UA（Cloudflare 防护），callModel 已全局带 UA。
	{ID: "free_kilo_nemotron_3_ultra", Vendor: "Kilo Gateway", Name: "Nemotron 3 Ultra 550B（免费）", Endpoint: "https://api.kilo.ai/api/gateway", Model: "nvidia/nemotron-3-ultra-550b-a55b:free", Keyless: true, Reasoning: true, ParamsB: 550},
	{ID: "free_kilo_nemotron_3_super", Vendor: "Kilo Gateway", Name: "Nemotron 3 Super 120B（免费）", Endpoint: "https://api.kilo.ai/api/gateway", Model: "nvidia/nemotron-3-super-120b-a12b:free", Keyless: true, Reasoning: true, ParamsB: 120},
	{ID: "free_kilo_nemotron_nano_omni", Vendor: "Kilo Gateway", Name: "Nemotron Nano Omni 30B（免费）", Endpoint: "https://api.kilo.ai/api/gateway", Model: "nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free", Keyless: true, Reasoning: true, ParamsB: 30},
	{ID: "free_kilo_tencent_hy3", Vendor: "Kilo Gateway", Name: "腾讯混元 Hy3（免费）", Endpoint: "https://api.kilo.ai/api/gateway", Model: "tencent/hy3:free", Keyless: true, Reasoning: true},
	{ID: "free_kilo_step_3_7_flash", Vendor: "Kilo Gateway", Name: "阶跃 step-3.7-flash（免费）", Endpoint: "https://api.kilo.ai/api/gateway", Model: "stepfun/step-3.7-flash:free", Keyless: true, Reasoning: true},
	{ID: "free_kilo_laguna_s_2_1", Vendor: "Kilo Gateway", Name: "Laguna S 2.1（免费）", Endpoint: "https://api.kilo.ai/api/gateway", Model: "poolside/laguna-s-2.1:free", Keyless: true, Reasoning: true},

	// —— Cerebras（api.cerebras.ai/v1，OpenAI 兼容）——
	// 2026-08-08 接入：免费档 1M tokens/天（gpt-oss-120b 5 RPM / 30K TPM），
	// ⚠️ Cloudflare 地域风控：大陆 IP 直连被拒（error code 1009，实测），
	// 需海外出口（ResceneCloud Render 云端代理）或海外网络；key 已存 default.json。
	{ID: "free_cerebras_gpt_oss_120b", Vendor: "Cerebras", Name: "GPT-OSS 120B（Cerebras·免费）", Endpoint: "https://api.cerebras.ai/v1", Model: "gpt-oss-120b", KeyEnv: "CEREBRAS_API_KEY", CtxWindow: 131072, Reasoning: true, ParamsB: 120, KeyURL: "https://cloud.cerebras.ai/"},
	{ID: "free_cerebras_qwen3_235b", Vendor: "Cerebras", Name: "Qwen3-235B（Cerebras·免费）", Endpoint: "https://api.cerebras.ai/v1", Model: "qwen-3-235b-a22b-instruct-2507", KeyEnv: "CEREBRAS_API_KEY", CtxWindow: 131072, Reasoning: true, ParamsB: 235, KeyURL: "https://cloud.cerebras.ai/"},
	{ID: "free_cerebras_glm_4_7", Vendor: "Cerebras", Name: "GLM-4.7（Cerebras·免费）", Endpoint: "https://api.cerebras.ai/v1", Model: "zai-glm-4.7", KeyEnv: "CEREBRAS_API_KEY", CtxWindow: 131072, Reasoning: true, KeyURL: "https://cloud.cerebras.ai/"},

	// —— 智谱 BigModel（open.bigmodel.cn/api/paas/v4，OpenAI 兼容）——
	// 2026-08-08 接入：GLM Flash 系列永久免费（30 并发）。实测可用：
	// glm-4.5-flash / glm-4.1v-thinking-flash / glm-4-flash-250414（✅ 直接返回）；
	// glm-4.7-flash / glm-4.6v-flash 免费档访问量大（1305 稍后再试，也收录轮换）。
	{ID: "free_zhipu_glm_4_7_flash", Vendor: "智谱 BigModel", Name: "GLM-4.7-Flash（永久免费）", Endpoint: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4.7-flash", KeyEnv: "ZHIPU_API_KEY", CtxWindow: 200000, Reasoning: true, KeyURL: "https://open.bigmodel.cn/usercenter/apikeys"},
	{ID: "free_zhipu_glm_4_5_flash", Vendor: "智谱 BigModel", Name: "GLM-4.5-Flash（永久免费）", Endpoint: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4.5-flash", KeyEnv: "ZHIPU_API_KEY", CtxWindow: 128000, Reasoning: true, KeyURL: "https://open.bigmodel.cn/usercenter/apikeys"},
	{ID: "free_zhipu_glm_4_1v_thinking", Vendor: "智谱 BigModel", Name: "GLM-4.1V-Thinking-Flash（免费）", Endpoint: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4.1v-thinking-flash", KeyEnv: "ZHIPU_API_KEY", CtxWindow: 128000, Vision: true, Reasoning: true, KeyURL: "https://open.bigmodel.cn/usercenter/apikeys"},
	{ID: "free_zhipu_glm_4_flash", Vendor: "智谱 BigModel", Name: "GLM-4-Flash（永久免费）", Endpoint: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4-flash-250414", KeyEnv: "ZHIPU_API_KEY", CtxWindow: 128000, KeyURL: "https://open.bigmodel.cn/usercenter/apikeys"},
	{ID: "free_zhipu_glm_4_6v_flash", Vendor: "智谱 BigModel", Name: "GLM-4.6V-Flash（免费·视觉）", Endpoint: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4.6v-flash", KeyEnv: "ZHIPU_API_KEY", CtxWindow: 128000, Vision: true, Reasoning: true, KeyURL: "https://open.bigmodel.cn/usercenter/apikeys"},

	// —— Groq（api.groq.com/openai/v1，OpenAI 兼容）——
	// 2026-08-08 接入：免费档 30 RPM / 1000 RPD（gpt-oss-120b 等）。
	// ⚠️ 地域风控：大陆 IP 直连 403 Forbidden（无 key/假 key/真 key 同码，实测），
	// 需海外出口（ResceneCloud Render 云端代理）；key 已存 default.json。
	{ID: "free_groq_llama_3_3_70b", Vendor: "Groq Cloud", Name: "Llama 3.3 70B Versatile（Groq·免费额度）", Endpoint: "https://api.groq.com/openai/v1", Model: "llama-3.3-70b-versatile", KeyEnv: "GROQ_API_KEY", CtxWindow: 128000, ParamsB: 70, KeyURL: "https://console.groq.com/keys"},
	{ID: "free_groq_gpt_oss_120b", Vendor: "Groq Cloud", Name: "GPT-OSS 120B（Groq·免费额度）", Endpoint: "https://api.groq.com/openai/v1", Model: "openai/gpt-oss-120b", KeyEnv: "GROQ_API_KEY", CtxWindow: 131072, Reasoning: true, ParamsB: 120, KeyURL: "https://console.groq.com/keys"},
	{ID: "free_groq_gpt_oss_20b", Vendor: "Groq Cloud", Name: "GPT-OSS 20B（Groq·免费额度）", Endpoint: "https://api.groq.com/openai/v1", Model: "openai/gpt-oss-20b", KeyEnv: "GROQ_API_KEY", CtxWindow: 131072, Reasoning: true, ParamsB: 20, KeyURL: "https://console.groq.com/keys"},
	{ID: "free_groq_qwen3_32b", Vendor: "Groq Cloud", Name: "Qwen3 32B（Groq·免费额度）", Endpoint: "https://api.groq.com/openai/v1", Model: "qwen/qwen3-32b", KeyEnv: "GROQ_API_KEY", CtxWindow: 131072, Reasoning: true, ParamsB: 32, KeyURL: "https://console.groq.com/keys"},

	// —— 阶跃星辰 StepFun ——
	{ID: "free_step_1o_turbo_vision", Vendor: "阶跃星辰", Name: "step-1o-turbo-vision（识图）", Endpoint: "https://api.stepfun.com/v1", Model: "step-1o-turbo-vision", KeyEnv: "STEP_API_KEY", Vision: true, Reasoning: true, KeyURL: "https://platform.stepfun.com/"},
	{ID: "free_step_3_7_flash", Vendor: "阶跃星辰", Name: "step-3.7-flash（免费）", Endpoint: "https://api.stepfun.com/v1", Model: "step-3.7-flash", KeyEnv: "STEP_API_KEY", KeyURL: "https://platform.stepfun.com/"},
	{ID: "plan_step_gateway", Vendor: "Step Plan 订阅", Name: "step-3.7-flash（订阅 Credit）", Endpoint: "https://api.stepfun.com/step_plan/v1", Model: "step-3.7-flash", KeyEnv: "STEP_API_KEY", KeyURL: "https://platform.stepfun.com/plan-subscribe"},

	// —— SenseNova 商汤 ——
	{ID: "free_sensenova_6_7_flash_lite", Vendor: "SenseNova", Name: "SenseNova 6.7 Flash-Lite（免费）", Endpoint: "https://token.sensenova.cn/v1", Model: "sensenova-6.7-flash-lite", KeyEnv: "SENSENOVA_API_KEY", Vision: true, CtxWindow: 262144, Reasoning: true, KeyURL: "https://platform.sensenova.cn/console/keys"},
	{ID: "free_sensenova_deepseek_v4_flash", Vendor: "SenseNova", Name: "DeepSeek V4 Flash（商汤·免费）", Endpoint: "https://token.sensenova.cn/v1", Model: "deepseek-v4-flash", KeyEnv: "SENSENOVA_API_KEY", CtxWindow: 1048576, Reasoning: true, KeyURL: "https://platform.sensenova.cn/console/keys"},
	{ID: "free_sensenova_glm_5_2", Vendor: "SenseNova", Name: "GLM-5.2（商汤·免费）", Endpoint: "https://token.sensenova.cn/v1", Model: "glm-5.2", KeyEnv: "SENSENOVA_API_KEY", Reasoning: true, KeyURL: "https://platform.sensenova.cn/console/keys"},

	// —— DeepSeek 官方 API ——
	{ID: "deepseek_v4_pro", Vendor: "DeepSeek", Name: "DeepSeek V4 Pro（正式版0813）", Endpoint: "https://api.deepseek.com/v1", Model: "deepseek-v4-pro", KeyEnv: "DEEPSEEK_API_KEY", CtxWindow: 1048576, Reasoning: true, KeyURL: "https://platform.deepseek.com/api_keys"},

	// —— ModelScope 魔搭 ——
	{ID: "free_modelscope_qwen3_5_397b", Vendor: "ModelScope", Name: "Qwen3.5-397B（免费·每日2000次）", Endpoint: "https://api-inference.modelscope.cn/v1", Model: "Qwen/Qwen3.5-397B-A17B", KeyEnv: "MODELSCOPE_API_KEY", ParamsB: 397, Reasoning: true, KeyURL: "https://modelscope.cn"},
	{ID: "free_modelscope_qwen3_235b", Vendor: "ModelScope", Name: "Qwen3-235B（免费·每日2000次）", Endpoint: "https://api-inference.modelscope.cn/v1", Model: "Qwen/Qwen3-235B-A22B", KeyEnv: "MODELSCOPE_API_KEY", ParamsB: 235, Reasoning: true, KeyURL: "https://modelscope.cn"},
	{ID: "free_modelscope_glm_5_2", Vendor: "ModelScope", Name: "GLM-5.2（免费·每日2000次）", Endpoint: "https://api-inference.modelscope.cn/v1", Model: "ZhipuAI/GLM-5.2", KeyEnv: "MODELSCOPE_API_KEY", Reasoning: true, KeyURL: "https://modelscope.cn"},
	{ID: "free_modelscope_deepseek_v4_flash", Vendor: "ModelScope", Name: "DeepSeek V4 Flash（免费·每日2000次）", Endpoint: "https://api-inference.modelscope.cn/v1", Model: "deepseek-ai/DeepSeek-V4-Flash", KeyEnv: "MODELSCOPE_API_KEY", Reasoning: true, KeyURL: "https://modelscope.cn"},
	{ID: "free_modelscope_qwen2_5_vl", Vendor: "ModelScope", Name: "Qwen3-VL-235B（免费·识图）", Endpoint: "https://api-inference.modelscope.cn/v1", Model: "Qwen/Qwen3-VL-235B-A22B-Instruct", KeyEnv: "MODELSCOPE_API_KEY", Vision: true, CtxWindow: 131072, KeyURL: "https://modelscope.cn"},

	// —— Ollama Cloud ——
	{ID: "cloud_ollama_gpt_oss_120b", Vendor: "Ollama Cloud", Name: "GPT-OSS 120B（免费·云端）", Endpoint: "https://ollama.com/v1", Model: "gpt-oss:120b", KeyEnv: "OLLAMA_API_KEY", CtxWindow: 0, Reasoning: true, KeyURL: "https://ollama.com/settings/keys"},

	// —— NVIDIA NIM ——
	{ID: "free_nvidia_gpt_oss_120b", Vendor: "NVIDIA NIM", Name: "GPT-OSS 120B（英伟达·免费）", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "openai/gpt-oss-120b", KeyEnv: "NVIDIA_NIM_API_KEY", CtxWindow: 131072, KeyURL: "https://build.nvidia.com/settings/api-keys"},
	{ID: "free_nvidia_glm_5_2", Vendor: "NVIDIA NIM", Name: "GLM-5.2（英伟达·免费）", Endpoint: "https://integrate.api.nvidia.com/v1", Model: "z-ai/glm-5.2", KeyEnv: "NVIDIA_NIM_API_KEY", Reasoning: true, KeyURL: "https://build.nvidia.com/settings/api-keys"},
}

// workingModels 运行时可用模型（已配 key 或免 key）
var workingModels []FreeModel
var wmMu sync.RWMutex

// 熔断器
type circuitBreaker struct {
	openUntil time.Time
	failCount int
}

var circuits sync.Map

func circuitFail(b FreeModel) {
	k := b.Endpoint + "|" + b.Model
	now := time.Now()
	circuits.Store(k, &circuitBreaker{
		openUntil: now.Add(5 * time.Minute), // 5 分钟冷却（之前 30s 导致同一模型被反复选+429 循环）
		failCount: 1,
	})
	recordModelResult(b, false)
}

func circuitIsOpen(b FreeModel) bool {
	k := b.Endpoint + "|" + b.Model
	v, ok := circuits.Load(k)
	if !ok {
		return false
	}
	cb := v.(*circuitBreaker)
	if time.Now().After(cb.openUntil) {
		circuits.Delete(k)
		return false
	}
	return true
}

func circuitSuccess(b FreeModel) {
	k := b.Endpoint + "|" + b.Model
	circuits.Delete(k)
	recordModelResult(b, true)
}

// —— 模型信用（成功率）——
// 决策时按信用排序：成功率最高的模型优先（首选），次高的后台预备。
// 信用数据随每次调用自动积累（success/fail 计数），样本太少给中立 0.5。

type modelStat struct {
	Success int
	Fail    int
}

var (
	statsMu    sync.Mutex
	modelStats = map[string]*modelStat{}
)

func recordModelResult(b FreeModel, ok bool) {
	k := b.Endpoint + "|" + b.Model
	statsMu.Lock()
	st := modelStats[k]
	if st == nil {
		st = &modelStat{}
		modelStats[k] = st
	}
	if ok {
		st.Success++
	} else {
		st.Fail++
	}
	statsMu.Unlock()
}

// modelCredit 模型信用分：成功率为主；样本不足 3 次给中立 0.5（新模型不被歧视也不被高估）
func modelCredit(b FreeModel) float64 {
	statsMu.Lock()
	st := modelStats[b.Endpoint+"|"+b.Model]
	statsMu.Unlock()
	if st == nil {
		return 0.5
	}
	total := st.Success + st.Fail
	if total < 3 {
		return 0.5
	}
	return float64(st.Success) / float64(total)
}

// activePoolLimit 活跃池上限：超出按 LRU+信用淘汰（付费/垃圾模型自动沉底）
const activePoolLimit = 12

// modelUsed 模型最近使用时间（LRU 淘汰维度）
var modelUsed sync.Map // id -> time.Time

// markModelUsed 记录模型最近一次使用（callModel 成功后调）
func markModelUsed(id string) {
	modelUsed.Store(id, time.Now())
}

// lastUsedAt 模型最近使用时间（未用过的给 zero 时间，排最后）
func lastUsedAt(m FreeModel) time.Time {
	if v, ok := modelUsed.Load(m.ID); ok {
		return v.(time.Time)
	}
	return time.Time{}
}

// rankModels 按信用降序返回可用模型（过滤熔断的）——决策首选排最前。
// LRU+信用淘汰：信用优先，同信用随机轮换；超活跃池上限直接裁掉尾部。
func rankModels(models []FreeModel) []FreeModel {
	out := make([]FreeModel, 0, len(models))
	for _, m := range models {
		if circuitIsOpen(m) {
			continue
		}
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ci := modelCredit(out[i])
		cj := modelCredit(out[j])
		if ci != cj {
			return ci > cj
		}
		// 同信用随机轮换（避免总选同一模型被 429 循环）
		seed := time.Now().Unix() / 60
		h := hashPair(out[i].ID, out[j].ID, seed)
		return h > 0
	})
	if len(out) > activePoolLimit {
		out = out[:activePoolLimit]
	}
	return out
}

// hashPair 确定性伪随机比较（同信用换序，每分钟换一次）
func hashPair(a, b string, seed int64) int {
	s := a + "|" + b + fmt.Sprintf("|%d", seed)
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	return h % 2
}

// InitRouter 初始化路由：过滤出可用模型
func InitRouter() {
	refreshModels()
}

// isFreeModel 是否免费模型（付费模型屏蔽）。
// 规则：keyless 免 key 网关；或 ID 含 "free"（Zen 免费档后面都写了 free）；
// 显式白名单兜底免费档但 ID 不含 free 的（如 Ollama Cloud 免费档）。
func isFreeModel(m FreeModel) bool {
	if m.Keyless {
		return true
	}
	if strings.Contains(m.ID, "free") {
		return true
	}
	switch m.ID {
	case "cloud_ollama_gpt_oss_120b": // Ollama Cloud 免费档（Name 标免费·云端）
		return true
	}
	return false // plan_step_gateway 等订阅/付费模型不进免费池
}

func refreshModels() {
	byID, byEnv := userConfigKeys()
	var available []FreeModel
	for _, m := range freeModels {
		if !isFreeModel(m) {
			continue // 付费模型屏蔽（订阅 Credit 等）
		}
		if m.Keyless {
			available = append(available, m)
			continue
		}
		key := os.Getenv(m.KeyEnv)
		if key == "" {
			key = byID[m.ID] // default.json 兜底（按模型 ID）
		}
		if key == "" {
			key = byEnv[m.KeyEnv] // 同服务商 key 共用（魔搭一个 token 全模型可用）
		}
		if key != "" {
			available = append(available, m)
		}
	}
	wmMu.Lock()
	workingModels = available
	wmMu.Unlock()
}

// userConfigKeys 从 ~/rescene_data/user_configs/default.json 读取用户配置的 API key。
// 返回两张表：byID（模型 ID → key）、byEnv（KeyEnv 服务商 → key，同服务商 key 共用）。
// default.json 优先；agent-os 环境变量兜底——用户在 Rescene 前端刚保存/选定的 key
// 必须覆盖可能随父进程继承进来的陈旧环境变量，避免“聊天能用、公司 401”。
// 24H 自转才有足够大的免费模型池（ModelScope 每日 2000 次等）。
// 安全：脱敏条目（含 "..."）跳过；key 绝不打印。
func userConfigKeys() (byID, byEnv map[string]string) {
	byID = map[string]string{}
	byEnv = map[string]string{}
	home, err := os.UserHomeDir()
	if err != nil {
		return byID, byEnv
	}
	path := filepath.Join(home, "rescene_data", "user_configs", "default.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return byID, byEnv
	}
	var list []struct {
		ID           string `json:"id"`
		APIKey       string `json:"api_key"`
		Endpoint     string `json:"endpoint"`
		DefaultModel string `json:"default_model"`
	}
	if json.Unmarshal(data, &list) != nil {
		return byID, byEnv
	}
	for _, it := range list {
		if it.ID == "" || it.APIKey == "" {
			continue
		}
		if strings.Contains(it.APIKey, "...") {
			continue // 脱敏条目不可用
		}
		byID[it.ID] = it.APIKey
		// 前端创建的自定义 DeepSeek 配置使用动态 cfg_* ID；Agent OS
		// 仍需把同一把 key 映射到官方模型的服务商环境名。
		// 只认官方端点。SenseNova 等兼容服务也会把 default_model 命名为
		// deepseek-*，按模型名前缀映射会让后出现的第三方 key 覆盖官方 key。
		if strings.Contains(strings.ToLower(it.Endpoint), "api.deepseek.com") {
			byEnv["DEEPSEEK_API_KEY"] = it.APIKey
		}
		// 顺带登记服务商级 key（同 KeyEnv 的模型共用）
		for _, m := range freeModels {
			if m.ID == it.ID && m.KeyEnv != "" {
				byEnv[m.KeyEnv] = it.APIKey
			}
		}
	}
	return byID, byEnv
}

// GetWorkingModels 返回当前可用模型列表
func GetWorkingModels() []FreeModel {
	wmMu.RLock()
	defer wmMu.RUnlock()
	out := make([]FreeModel, len(workingModels))
	copy(out, workingModels)
	return out
}

// pickFreeModel 选一个免 key 免费模型（背靠全网免费算力，不烧用户付费 key）。
// 无免费模型时返回 nil（调用方应 fallback 规则或用 key 兜底）。
// 2026-08-08 修复：跳过熔断中的模型——之前随机选到熔断模型直接失败
// （100 人公司里 Zen deepseek 被反复选中 → 「模型熔断中」循环，Kilo 新模型永远轮不到）。
// 用 rankModels 同款信用排序 + 随机轮换：可用池里挑，熔断的排后面。
func pickFreeModel(seed int) *FreeModel {
	models := GetWorkingModels()
	var free []FreeModel
	for _, m := range models {
		if m.Keyless {
			free = append(free, m)
		}
	}
	if len(free) == 0 {
		return nil
	}
	// 熔断的模型沉底（不直接删：万一全熔断还能拿到一个兜底尝试）
	sort.SliceStable(free, func(i, j int) bool {
		oi := circuitIsOpen(free[i])
		oj := circuitIsOpen(free[j])
		if oi != oj {
			return !oi // 没熔断的排前
		}
		// 同状态按信用降序（成功率高的优先）
		ci := modelCredit(free[i])
		cj := modelCredit(free[j])
		if ci != cj {
			return ci > cj
		}
		h := hashPair(free[i].ID, free[j].ID, int64(seed))
		return h > 0
	})
	if seed < 0 {
		seed = int(time.Now().UnixNano())
	}
	return &free[seed%len(free)]
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	// Images 多模态图片（data URL 或 http URL），OpenAI 兼容 vision 格式
	Images []string `json:"-"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// buildWireMessages 构造发送给 API 的 messages：带图片的用 OpenAI 兼容
// content 数组格式 [{"type":"text",...},{"type":"image_url","image_url":{"url":...}}]
func buildWireMessages(msgs []ChatMessage) []any {
	out := make([]any, 0, len(msgs))
	for _, m := range msgs {
		if len(m.Images) == 0 {
			out = append(out, map[string]any{"role": m.Role, "content": m.Content})
			continue
		}
		content := make([]any, 0, 1+len(m.Images))
		if m.Content != "" {
			content = append(content, map[string]any{"type": "text", "text": m.Content})
		}
		for _, img := range m.Images {
			content = append(content, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": img},
			})
		}
		out = append(out, map[string]any{"role": m.Role, "content": content})
	}
	return out
}

// callModel 单模型调用（不处理熔断，由调用方决定策略）
// onChunk 回调参数：content（回复内容）, reasoning（思考过程，可能为空）
func callModel(ctx context.Context, m FreeModel, req ChatRequest, onChunk func(content, reasoning string)) (string, error) {
	key := ""
	if !m.Keyless {
		byID, byEnv := userConfigKeys()
		key = byID[m.ID] // Rescene 前端当前配置优先（按模型 ID）
		if key == "" {
			key = byEnv[m.KeyEnv] // 同服务商 key 共用
		}
		if key == "" {
			key = os.Getenv(m.KeyEnv) // 服务进程环境变量仅作兜底
		}
	}

	body := map[string]any{
		"model":       m.Model,
		"messages":    buildWireMessages(req.Messages),
		"stream":      req.Stream,
		"max_tokens":  req.MaxTokens,
		"temperature": req.Temperature,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	url := strings.TrimRight(m.Endpoint, "/")
	if !strings.HasSuffix(url, "/chat/completions") {
		url += "/chat/completions"
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// 浏览器 UA 必带：OpenCode Zen / Kilo 等网关有 Cloudflare 防护，
	// Go 默认 UA（Go-http-client/1.1）会吃 403 error code 1010（2026-08-08 实测根因）。
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	if key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("[%s] %v", m.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("[%s] HTTP %d: %s", m.Name, resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	markModelUsed(m.ID) // HTTP 200 = 模型可达，记 LRU（最近可用维度）

	if req.Stream {
		return readStream(resp.Body, onChunk)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("[%s] 解析响应失败: %v", m.Name, err)
	}
	if len(chatResp.Choices) > 0 {
		return chatResp.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("[%s] 空响应", m.Name)
}

// Complete 发送聊天请求到模型，自动 failover
// 策略：优先本地（有 key 的模型）→ 云端（免 key 网关）兜底
// onChunk 回调参数：content（回复内容）, reasoning（思考过程，可能为空）
func Complete(ctx context.Context, req ChatRequest, onChunk func(content, reasoning string)) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req.Temperature == 0 {
		req.Temperature = 0.2
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}

	wmMu.RLock()
	models := make([]FreeModel, len(workingModels))
	copy(models, workingModels)
	wmMu.RUnlock()

	if len(models) == 0 {
		return "", fmt.Errorf("没有可用的免费模型。请通过环境变量配置 API Key，或使用免 key 的 Zen 模型。\n\n可用环境变量:\n  SENSENOVA_API_KEY (商汤免费)\n  MODELSCOPE_API_KEY (魔搭免费)\n  STEP_API_KEY (阶跃星辰免费)\n  OLLAMA_API_KEY (Ollama Cloud 免费)\n  NVIDIA_NIM_API_KEY (NVIDIA 免费)\n\n免 key 模型（直接可用）:\n  OpenCode Zen 网关")
	}

	// 第一轮：有 key 的本地模型优先
	var lastErr error
	for _, m := range models {
		if m.Keyless {
			continue
		}
		if circuitIsOpen(m) {
			continue
		}
		content, err := callModel(ctx, m, req, onChunk)
		if err != nil {
			circuitFail(m)
			lastErr = err
			continue
		}
		circuitSuccess(m)
		return content, nil
	}

	// 第二轮：免 key 的云端网关兜底
	for _, m := range models {
		if !m.Keyless {
			continue
		}
		if circuitIsOpen(m) {
			continue
		}
		content, err := callModel(ctx, m, req, onChunk)
		if err != nil {
			circuitFail(m)
			lastErr = err
			continue
		}
		circuitSuccess(m)
		return content, nil
	}

	if lastErr == nil {
		// 没有任何模型真正尝试过：全部被熔断跳过（冷却中），报 <nil> 是误导
		return "", fmt.Errorf("所有模型均处于熔断冷却中（连续失败自动冷却，稍后恢复）")
	}
	return "", fmt.Errorf("所有模型均失败: %v", lastErr)
}

// CompleteWithModel 指定模型调用（不做 failover，供 marathon 轮询全网免费模型）
// onChunk 回调参数：content（回复内容）, reasoning（思考过程，可能为空）
func CompleteWithModel(ctx context.Context, modelID string, req ChatRequest, onChunk func(content, reasoning string)) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req.Temperature == 0 {
		req.Temperature = 0.2
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}

	wmMu.RLock()
	models := make([]FreeModel, len(workingModels))
	copy(models, workingModels)
	wmMu.RUnlock()

	var target *FreeModel
	for i := range models {
		if models[i].ID == modelID {
			target = &models[i]
			break
		}
	}
	if target == nil {
		// 付费模型不进入自动免费轮换池，但用户明确点名时允许精确调用。
		// 这条路径不做 failover，失败就把原模型错误返回给界面。
		byID, byEnv := userConfigKeys()
		for i := range freeModels {
			candidate := freeModels[i]
			if candidate.ID != modelID {
				continue
			}
			if candidate.Keyless || os.Getenv(candidate.KeyEnv) != "" || byID[candidate.ID] != "" || byEnv[candidate.KeyEnv] != "" {
				target = &candidate
			}
			break
		}
		if target == nil {
			return "", fmt.Errorf("模型不可用或未配置 key: %s", modelID)
		}
	}
	if circuitIsOpen(*target) {
		return "", fmt.Errorf("模型熔断中: %s", modelID)
	}

	content, err := callModel(ctx, *target, req, onChunk)
	if err != nil {
		circuitFail(*target)
		return "", err
	}
	circuitSuccess(*target)
	return content, nil
}

// readStream 读取 SSE 流
// onChunk 回调参数：content（回复内容）, reasoning（思考过程，可能为空）
func readStream(body io.ReadCloser, onChunk func(content, reasoning string)) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var fullContent strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			reasoning := chunk.Choices[0].Delta.ReasoningContent
			if content != "" {
				fullContent.WriteString(content)
			}
			if onChunk != nil {
				onChunk(content, reasoning)
			}
		}
	}

	return fullContent.String(), nil
}

// —— 每日探活：模型池自循环（被淘汰的模型恢复可用后重新入池） ——

// probeModels 遍历全部免费候选，最小请求验证可用性；可用 → 清熔断 + 刷新池。
// 由 trumanLoop/daemon 每 24h 调一次（异步，不阻塞生活循环）。
func probeModels() {
	cands := make([]FreeModel, 0, len(freeModels))
	for _, m := range freeModels {
		if isFreeModel(m) {
			cands = append(cands, m)
		}
	}
	okN := 0
	var failed []string
	for _, m := range cands {
		if probeOne(m) {
			okN++
			circuits.Delete(m.ID) // 探活成功：清熔断，重新可用
			markModelUsed(m.ID)
		} else {
			failed = append(failed, m.ID)
		}
	}
	refreshModels()
	// 结果记入 live.log（不打扰终端）
	if home, err := os.UserHomeDir(); err == nil {
		detail := ""
		if len(failed) > 0 {
			detail = "，失败: " + strings.Join(failed, ", ")
		}
		logLive(filepath.Join(home, "rescene_data", "daughter", "live.log"),
			fmt.Sprintf("[%s] 🛰️ 每日探活：%d/%d 免费模型可用%s", time.Now().Format("15:04"), okN, len(cands), detail))
	}
}

// probeOne 单模型探活：最小请求（MaxTokens=1），8s 超时
func probeOne(m FreeModel) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	msg := ChatRequest{
		Model:       m.Model,
		Messages:    []ChatMessage{{Role: "user", Content: "ping"}},
		Stream:      false,
		MaxTokens:   1,
		Temperature: 0,
	}
	_, err := callModel(ctx, m, msg, nil)
	return err == nil
}
