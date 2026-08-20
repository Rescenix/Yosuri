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
	ID      string // 免费池条目 ID（free_xxx）——Auto 排序 / 熔断追踪用；自定义源为空
	Name    string
	BaseURL string
	Model   string
	APIKey  string // 空 = 免 key（本地）
	ParamsB float64
	IsLocal bool
	Keyless bool
	Timeout time.Duration
	Source  string // user / env / free / local
	// 能力元数据：前端按模型配置，决定能否识图 / 上下文窗口 / 是否支持思考强度
	Vision        bool `json:"vision"`
	ContextWindow int  `json:"context_window"`
	Reasoning     bool `json:"reasoning"`
	// WireResponses 走 OpenAI Responses API（/responses）而非 Chat Completions。
	// DeepSeek 服务端联网搜索（web_search 工具）只在 Responses 协议下可用，
	// chat/completions 端点会 400 拒绝 web_search 工具类型（2026-08-01 实测）。
	WireResponses bool `json:"wire_responses"`
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
	// KeyURL 是提供方官网的 API Key 申请/管理页面（前端「官网获取 Key」按钮的跳转地址，
	// 打开登录即可免费领取 API Key，粘贴输入框即用）。免 Key 网关（Keyless=true）留空。
	KeyURL string `json:"key_url,omitempty"`
	// 能力元数据（公开已知值；未知者留 0/false，绝不伪造）
	Vision        bool `json:"vision"`
	ContextWindow int  `json:"context_window"`
	Reasoning     bool `json:"reasoning"`
	// Local=true 表示走本地 Ollama（localhost:11434/v1，OpenAI 兼容）路由到云端模型，
	// 不需要 API Key，复用现有 OpenAI 兼容链。
	Local bool `json:"local"`
	// Keyless=true 表示远端网关本身免 key（如 opencode zen：鉴权全程由域名承载，
	// 无需 Bearer Token），可直接进链、可直接被「提供方」勾选，无需填 Key。
	Keyless bool `json:"keyless"`
	// Responses=true 表示该模型走 OpenAI Responses API（/responses）协议而非
	// chat/completions，用于启用 DeepSeek 服务端联网搜索（web_search 工具）。
	Responses bool `json:"responses"`
	// Disabled=true 表示该模型被运行时探测判定为不可用（如提供方退役/下架），
	// 路由链与精确解析均跳过；由 nimRefresh 每日探测后动态置位。
	Disabled bool `json:"disabled"`
}

// 参数规模是公开估计值；未知者写 0，排序时排免费池末段，绝不伪造。
// Vendor 字段用于前端按厂商折叠分组（仿 Hermes 提供方分类）。
var freeModelCatalog = []FreeModelDef{
	// —— Kilo Gateway 免 key 网关（api.kilo.ai/api/gateway/v1，OpenAI 兼容）——
	// 2026-08-15 新增：Kilo 支持匿名访问 :free 后缀模型，无需 API Key，
	// 每小时 200 次/IP 限流。实测 /v1/models 返回 13 个 isFree=true 模型，全部加入。
	{ID: "kilo_auto_free", Vendor: "Kilo Gateway", Name: "Kilo Auto Free（免 key）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "kilo-auto/free", KeyEnv: "", ParamsB: 0, Note: "Kilo 免 key 自动路由·200次/h/IP", Keyless: true, Reasoning: true, ContextWindow: 256000},
	{ID: "kilo_step_3_7_flash_free", Vendor: "Kilo Gateway", Name: "Step 3.7 Flash（免 key）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "stepfun/step-3.7-flash:free", KeyEnv: "", ParamsB: 0, Note: "Kilo 免 key·step-3.7-flash·识图·262K", Keyless: true, Vision: true, Reasoning: true, ContextWindow: 262144},
	{ID: "kilo_tencent_hy3_free", Vendor: "Kilo Gateway", Name: "Tencent Hy3（免 key·295B）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "tencent/hy3:free", KeyEnv: "", ParamsB: 295, Note: "Kilo 免 key·腾讯 Hy3 295B MoE·262K", Keyless: true, Reasoning: true, ContextWindow: 262144},
	{ID: "kilo_laguna_s_2_1_free", Vendor: "Kilo Gateway", Name: "Laguna S 2.1（免 key·118B）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "poolside/laguna-s-2.1:free", KeyEnv: "", ParamsB: 118, Note: "Kilo 免 key·Poolside Laguna S 2.1·262K", Keyless: true, Reasoning: true, ContextWindow: 262144},
	{ID: "kilo_laguna_xs_2_1_free", Vendor: "Kilo Gateway", Name: "Laguna XS 2.1（免 key·33B）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "poolside/laguna-xs-2.1:free", KeyEnv: "", ParamsB: 33, Note: "Kilo 免 key·Poolside Laguna XS 2.1·262K", Keyless: true, Reasoning: true, ContextWindow: 262144},
	{ID: "kilo_north_mini_code_free", Vendor: "Kilo Gateway", Name: "North Mini Code（免 key）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "cohere/north-mini-code:free", KeyEnv: "", ParamsB: 0, Note: "Kilo 免 key·Cohere North Mini Code·256K", Keyless: true, Reasoning: true, ContextWindow: 256000},
	{ID: "kilo_nemotron_lightning_free", Vendor: "Kilo Gateway", Name: "Nemotron 3.5 Lightning（免 key·3B）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "nvidia/nemotron-3.5-lightning:free", KeyEnv: "", ParamsB: 3, Note: "Kilo 免 key·NVIDIA Nemotron 3.5 Lightning·1M", Keyless: true, Reasoning: true, ContextWindow: 1000000},
	{ID: "kilo_nemotron_nano_free", Vendor: "Kilo Gateway", Name: "Nemotron 3 Nano Omni（免 key·30B）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free", KeyEnv: "", ParamsB: 30, Note: "Kilo 免 key·NVIDIA Nemotron 3 Nano·识图·256K", Keyless: true, Vision: true, Reasoning: true, ContextWindow: 256000},
	{ID: "kilo_nemotron_super_free", Vendor: "Kilo Gateway", Name: "Nemotron 3 Super（免 key·120B）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "nvidia/nemotron-3-super-120b-a12b:free", KeyEnv: "", ParamsB: 120, Note: "Kilo 免 key·NVIDIA Nemotron 3 Super·262K", Keyless: true, Reasoning: true, ContextWindow: 262144},
	{ID: "kilo_nemotron_ultra_free", Vendor: "Kilo Gateway", Name: "Nemotron 3 Ultra（免 key·550B）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "nvidia/nemotron-3-ultra-550b-a55b:free", KeyEnv: "", ParamsB: 550, Note: "Kilo 免 key·NVIDIA Nemotron 3 Ultra·1M", Keyless: true, Reasoning: true, ContextWindow: 1000000},
	{ID: "kilo_liquid_lfm_free", Vendor: "Kilo Gateway", Name: "Liquid LFM2.5-2.6B（免 key）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "liquid/lfm-2.5-2.6b:free", KeyEnv: "", ParamsB: 2.6, Note: "Kilo 免 key·LiquidAI LFM2.5·128K", Keyless: true, Reasoning: true, ContextWindow: 128000},
	{ID: "kilo_nemotron_safety_free", Vendor: "Kilo Gateway", Name: "Nemotron 3.5 Safety（免 key·4B）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "nvidia/nemotron-3.5-content-safety:free", KeyEnv: "", ParamsB: 4, Note: "Kilo 免 key·NVIDIA 内容安全护栏·识图·非对话", Keyless: true, Vision: true, ContextWindow: 128000},
	{ID: "kilo_openrouter_free", Vendor: "Kilo Gateway", Name: "OpenRouter Free（免 key）", Endpoint: "https://api.kilo.ai/api/gateway/v1", Model: "openrouter/free", KeyEnv: "", ParamsB: 0, Note: "Kilo 免 key·OpenRouter 免费路由·200K", Keyless: true, Vision: true, Reasoning: true, ContextWindow: 200000},

	// —— OpenRouter 已整体移除：免费档全部 slug 限流 429（连 llama-3.3-70b/405b 都 429），
	// 无专属免费模型，作为网关接入价值为零，徒增链尾失败噪声。2026-07-15 实测确认。

	// —— Google AI Studio 移除：2026-07-21 实测网络不可达（WinError 10060），大陆无法直连。 ——

	// —— NVIDIA NIM 免费试用档已整体移除：2026-07-23 实测限流严重，跑 Agent 频繁 429，
	// 体验不可用。保留 nim_refresh.go 作为运行时探测骨架，目录中不再硬编码 NIM 条目。 ——

	// —— 以下模型保留：明确支持 Vision，可用于前端多模态 Agent 测试 ——

	// —— 阶跃星辰 StepFun（api.stepfun.com）——
	// 下面这几个都是拿 STEP_API_KEY 实调 /v1/models + /v1/chat/completions 逐个验证过的：
	// step-2x-large 在该 key 下返回「does not exist or you do not have access」，故未收录。
	// ContextWindow 一律留 0：/v1/models 只返回 id/created/owned_by，拿不到窗口大小，
	// 按本目录「未知者留 0，绝不伪造」的规矩不填。
	{ID: "free_step_1o_turbo_vision", Vendor: "阶跃星辰 StepFun", Name: "step-1o-turbo-vision（免费）", Endpoint: "https://api.stepfun.com/v1", Model: "step-1o-turbo-vision", KeyEnv: "STEP_API_KEY", ParamsB: 0, Note: "阶跃星辰（识图）", Vision: true, Reasoning: true, KeyURL: "https://platform.stepfun.com/"},
	{ID: "free_step_3_7_flash", Vendor: "阶跃星辰 StepFun", Name: "step-3.7-flash（免费）", Endpoint: "https://api.stepfun.com/v1", Model: "step-3.7-flash", KeyEnv: "STEP_API_KEY", ParamsB: 0, Note: "阶跃星辰免费档·agent 可用（实测 2026-08-02）", KeyURL: "https://platform.stepfun.com/"},

	// —— Step Plan 订阅已移除（2026-08-13 用户清理）：无订阅用不了，纯占位 ——

	// —— OpenCode Zen 免 key 网关（opencode.ai/zen/v1，OpenAI 兼容）——
	// 2026-07-28 用户实测接入：全程免 key，鉴权由域名承载；/v1/models 与
	// /v1/chat/completions 均免 Bearer Token（curl 空 Authorization 实测 cost=0 返回 OK）。
	// 模型列表来自 /models 实测筛选的 *-free 后缀档；初筛 6 个免费模型后逐一带
	// tools 做 agent 调用实测，淘汰 3 个不可用项（ling-3.0/限流、laguna-s-2.1/限流、
	// nemotron-3-ultra/tools 上游失败），仅保留 3 个能稳定返回 tool_calls 的。
	// 能力元数据未知者一律留 0/false，绝不伪造（与目录「未知留空」规矩一致）。
	// Keyless=true：免 key 远端网关，可直接进链、可被「提供方」直接勾选，无需填 Key。
	{ID: "free_zen_deepseek_v4_flash", Vendor: "OpenCode Zen", Name: "DeepSeek V4 Flash（免费）", Endpoint: "https://opencode.ai/zen/v1", Model: "deepseek-v4-flash-free", KeyEnv: "", ParamsB: 0, Note: "Zen 免 key 网关（免费档·agent 可用）", Keyless: true, Reasoning: true},
	{ID: "free_zen_mimo_v2_5", Vendor: "OpenCode Zen", Name: "Mimo 2.5（免费）", Endpoint: "https://opencode.ai/zen/v1", Model: "mimo-v2.5-free", KeyEnv: "", ParamsB: 0, Note: "Zen 免 key 网关（免费档·agent 可用）", Keyless: true, Reasoning: true},
	{ID: "free_zen_north_mini_code", Vendor: "OpenCode Zen", Name: "North Mini Code（免费）", Endpoint: "https://opencode.ai/zen/v1", Model: "north-mini-code-free", KeyEnv: "", ParamsB: 0, Note: "Zen 免 key 网关（免费档·agent 可用·最快）", Keyless: true, Reasoning: true},
	// 2026-08-08 重新爬取 Zen /v1/models（61 模型，8 个 free 档）实测新增：
	// Nemotron 3 Ultra 已移除：免费档思考 4 分钟级超时（2026-08-13 用户清理）
	// longcat-2.0-free（全新可用）、laguna-s-2.1-free（恢复可用）；ling-3.0-tiny/flash-free 上游挂未收录
	{ID: "free_zen_longcat_2_0", Vendor: "OpenCode Zen", Name: "Longcat 2.0（免费·新增）", Endpoint: "https://opencode.ai/zen/v1", Model: "longcat-2.0-free", KeyEnv: "", ParamsB: 0, Note: "Zen 免 key 网关（免费档·2026-08-08 新增实测可用）", Keyless: true, Reasoning: true},
	{ID: "free_zen_laguna_s_2_1", Vendor: "OpenCode Zen", Name: "Laguna S 2.1（免费·恢复）", Endpoint: "https://opencode.ai/zen/v1", Model: "laguna-s-2.1-free", KeyEnv: "", ParamsB: 0, Note: "Zen 免 key 网关（免费档·2026-08-08 恢复可用）", Keyless: true, Reasoning: true},

	// —— Kilo Gateway 已整体移除（2026-08-13 用户清理）：免费档排队+推理超时（Nemotron 思考 4 分钟元凶），
	// 与 Zen 的模型重复度高，且实测 6 个 :free 模型质量一般 ——

	// —— Cerebras 已整体移除（2026-08-13 用户清理）：大陆 IP 被 Cloudflare 地域风控（error 1009 实测必挂）——

	// —— 智谱 BigModel（open.bigmodel.cn/api/paas/v4，OpenAI 兼容）——
	// 2026-08-08 接入：GLM Flash 系列永久免费（30 并发）。实测可用：
	// glm-4.5-flash / glm-4.1v-thinking-flash / glm-4-flash-250414 ✅；
	// glm-4.7-flash / glm-4.6v-flash 免费档访问量大（1305 稍后再试，也收录轮换）。
	{ID: "free_zhipu_glm_4_7_flash", Vendor: "智谱 BigModel", Name: "GLM-4.7-Flash（永久免费）", Endpoint: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4.7-flash", KeyEnv: "ZHIPU_API_KEY", ParamsB: 0, Note: "智谱永久免费·30 并发", ContextWindow: 200000, Reasoning: true, KeyURL: "https://open.bigmodel.cn/usercenter/apikeys"},
	{ID: "free_zhipu_glm_4_5_flash", Vendor: "智谱 BigModel", Name: "GLM-4.5-Flash（永久免费）", Endpoint: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4.5-flash", KeyEnv: "ZHIPU_API_KEY", ParamsB: 0, Note: "智谱永久免费·30 并发", ContextWindow: 128000, Reasoning: true, KeyURL: "https://open.bigmodel.cn/usercenter/apikeys"},
	{ID: "free_zhipu_glm_4_1v_thinking", Vendor: "智谱 BigModel", Name: "GLM-4.1V-Thinking-Flash（免费）", Endpoint: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4.1v-thinking-flash", KeyEnv: "ZHIPU_API_KEY", ParamsB: 0, Note: "智谱免费·识图", Vision: true, ContextWindow: 128000, Reasoning: true, KeyURL: "https://open.bigmodel.cn/usercenter/apikeys"},
	{ID: "free_zhipu_glm_4_flash", Vendor: "智谱 BigModel", Name: "GLM-4-Flash（永久免费）", Endpoint: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4-flash-250414", KeyEnv: "ZHIPU_API_KEY", ParamsB: 0, Note: "智谱永久免费", ContextWindow: 128000, KeyURL: "https://open.bigmodel.cn/usercenter/apikeys"},
	{ID: "free_zhipu_glm_4_6v_flash", Vendor: "智谱 BigModel", Name: "GLM-4.6V-Flash（免费·视觉）", Endpoint: "https://open.bigmodel.cn/api/paas/v4", Model: "glm-4.6v-flash", KeyEnv: "ZHIPU_API_KEY", ParamsB: 0, Note: "智谱免费·识图", Vision: true, ContextWindow: 128000, Reasoning: true, KeyURL: "https://open.bigmodel.cn/usercenter/apikeys"},

	// —— Groq 已整体移除（2026-08-13 用户清理）：大陆 IP 地域风控 403（无 key/真 key 全挂）——

	// —— 本地 llama.cpp 已移除：维护 Vision 标签成本高于收益，识图模型由用户自行选择 ——

	// —— Ollama Cloud 已整体移除（2026-08-13 用户清理）：403 requires subscription（付费墙）——

	// —— SenseNova 商汤（token.sensenova.cn/v1，OpenAI 兼容）——
	// 2026-08-02 用户要求接入。官方文档确认免费额度：sensenova-6.7-flash-lite 每 5 小时
	// 1500 次（256K 上下文 / 64K 输出，原生多模态支持图像输入）；deepseek-v4-flash 每 5 小时
	// 500 次（1M 上下文，支持思考模式）。Key 在 platform.sensenova.cn 控制台创建 sk- 密钥。
	{ID: "free_sensenova_6_7_flash_lite", Vendor: "SenseNova", Name: "SenseNova 6.7 Flash-Lite（免费）", Endpoint: "https://token.sensenova.cn/v1", Model: "sensenova-6.7-flash-lite", KeyEnv: "SENSENOVA_API_KEY", ParamsB: 0, Note: "商汤免费·每模型 1500 次调用/5 小时·多模态", Vision: true, ContextWindow: 262144, Reasoning: true, KeyURL: "https://platform.sensenova.cn/console/keys"},
	{ID: "free_sensenova_deepseek_v4_flash", Vendor: "SenseNova", Name: "DeepSeek V4 Flash（商汤·免费）", Endpoint: "https://token.sensenova.cn/v1", Model: "deepseek-v4-flash", KeyEnv: "SENSENOVA_API_KEY", ParamsB: 0, Note: "商汤免费·每模型 500 次调用/5 小时·1M 上下文", ContextWindow: 1048576, Reasoning: true, KeyURL: "https://platform.sensenova.cn/console/keys"},
	{ID: "free_sensenova_glm_5_2", Vendor: "SenseNova", Name: "GLM-5.2（商汤·免费）", Endpoint: "https://token.sensenova.cn/v1", Model: "glm-5.2", KeyEnv: "SENSENOVA_API_KEY", ParamsB: 0, Note: "商汤免费档·agent 可用（实测 2026-08-02）", Reasoning: true, KeyURL: "https://platform.sensenova.cn/console/keys"},

	// —— ModelScope 魔搭（api-inference.modelscope.cn/v1，OpenAI 兼容）——
	// 2026-08-02 用户要求接入。官方：注册即送每日 2000 次免费调用（Qwen3-Coder 单独 500 次），
	// 支持访问令牌（SDK Token）调用，模型 ID 是 ModelScope 命名空间格式
	// （Qwen/Qwen2.5-VL-72B-Instruct，官方示例模型，多模态支持图像输入）。
	{ID: "free_modelscope_qwen2_5_vl", Vendor: "ModelScope 魔搭", Name: "Qwen3-VL-235B（免费·每日2000次）", Endpoint: "https://api-inference.modelscope.cn/v1", Model: "Qwen/Qwen3-VL-235B-A22B-Instruct", KeyEnv: "MODELSCOPE_API_KEY", ParamsB: 235, Note: "魔搭免费·访问令牌调用·2000次/天·多模态", Vision: true, ContextWindow: 131072, KeyURL: "https://modelscope.cn"},
	{ID: "free_modelscope_qwen3_5_397b", Vendor: "ModelScope 魔搭", Name: "Qwen3.5-397B（免费·每日2000次）", Endpoint: "https://api-inference.modelscope.cn/v1", Model: "Qwen/Qwen3.5-397B-A17B", KeyEnv: "MODELSCOPE_API_KEY", ParamsB: 397, Note: "魔搭免费·访问令牌调用·2000次/天（实测 2026-08-02）", Reasoning: true, KeyURL: "https://modelscope.cn"},
	{ID: "free_modelscope_qwen3_235b", Vendor: "ModelScope 魔搭", Name: "Qwen3-235B（免费·每日2000次）", Endpoint: "https://api-inference.modelscope.cn/v1", Model: "Qwen/Qwen3-235B-A22B", KeyEnv: "MODELSCOPE_API_KEY", ParamsB: 235, Note: "魔搭免费·访问令牌调用·2000次/天（实测 2026-08-02）", Reasoning: true, KeyURL: "https://modelscope.cn"},
	{ID: "free_modelscope_glm_5_2", Vendor: "ModelScope 魔搭", Name: "GLM-5.2（免费·每日2000次）", Endpoint: "https://api-inference.modelscope.cn/v1", Model: "ZhipuAI/GLM-5.2", KeyEnv: "MODELSCOPE_API_KEY", ParamsB: 0, Note: "魔搭免费·访问令牌调用·2000次/天（实测 2026-08-02）", Reasoning: true, KeyURL: "https://modelscope.cn"},
	{ID: "free_modelscope_deepseek_v4_flash", Vendor: "ModelScope 魔搭", Name: "DeepSeek V4 Flash（免费·每日2000次）", Endpoint: "https://api-inference.modelscope.cn/v1", Model: "deepseek-ai/DeepSeek-V4-Flash-0731", KeyEnv: "MODELSCOPE_API_KEY", ParamsB: 0, Note: "魔搭免费·访问令牌调用·2000次/天·2026-08-13 ID 改 -0731（旧 ID 上游已下架 400）", Reasoning: true, KeyURL: "https://modelscope.cn"},

	// —— NVIDIA NIM 已整体移除（2026-08-13 用户清理：「纯纯垃圾，不要了」，实测慢/超时）——
	// —— Step Plan 订阅已移除：无订阅用不了（2026-08-13 用户清理）——

	// —— 硅基流动 SiliconFlow（api.siliconflow.cn/v1，OpenAI 兼容）——
	// 2026-08-18 新增：硅基流动赠送免费 API 额度，注册即领 2000 万 tokens。
	// 免费模型：DeepSeek V4 Flash / Qwen3.5-397B / Qwen3-235B 等。
	{ID: "free_silicon_deepseek_v4_flash", Vendor: "硅基流动", Name: "DeepSeek V4 Flash（硅基·免费）", Endpoint: "https://api.siliconflow.cn/v1", Model: "deepseek-v4-flash", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 0, Note: "硅基流动·免费额度·2000万 tokens", ContextWindow: 1048576, Reasoning: true, KeyURL: "https://cloud.siliconflow.cn/apiKey"},
	{ID: "free_silicon_qwen3_5_397b", Vendor: "硅基流动", Name: "Qwen3.5-397B（硅基·免费）", Endpoint: "https://api.siliconflow.cn/v1", Model: "qwen3.5-397b", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 397, Note: "硅基流动·免费额度", ContextWindow: 131072, Reasoning: true, KeyURL: "https://cloud.siliconflow.cn/apiKey"},
	{ID: "free_silicon_qwen3_235b", Vendor: "硅基流动", Name: "Qwen3-235B（硅基·免费）", Endpoint: "https://api.siliconflow.cn/v1", Model: "qwen3-235b", KeyEnv: "SILICONFLOW_API_KEY", ParamsB: 235, Note: "硅基流动·免费额度", ContextWindow: 131072, Reasoning: true, KeyURL: "https://cloud.siliconflow.cn/apiKey"},

}

func isFreeCatalogID(id string) bool {
	for _, f := range freeModelCatalog {
		if f.ID == id {
			return true
		}
	}
	return false
}

// userKeysByEnv 从用户配置里按 KeyEnv 收集 key：同一厂商（同 KeyEnv，如
// STEP_API_KEY）的多个免费模型共用一把 key——用户只要给任一 step 模型配过
// 个人 key，其他 step 模型（step-3.7-flash 等）都算可用，不再要求逐个同名
// 条目（2026-08-04：step-3.7-flash 之前因无独立条目被踢出下拉/路由链）。
func userKeysByEnv(userKey string) map[string]string {
	envOfID := map[string]string{}
	for _, f := range freeModelCatalog {
		if f.KeyEnv != "" {
			envOfID[f.ID] = f.KeyEnv
		}
	}
	out := map[string]string{}
	if entries, err := loadModelConfigs(userKey); err == nil {
		for _, e := range entries {
			env, ok := envOfID[e.ID]
			if !ok || env == "" || strings.TrimSpace(e.APIKey) == "" {
				continue
			}
			if _, exists := out[env]; !exists {
				out[env] = strings.TrimSpace(e.APIKey)
			}
		}
	}
	return out
}

// searchBackend 已随 DeepSeek 服务端联网搜索一并退役（2026-08-04）：
// 联网搜索改由常驻 web_search 工具（Firecrawl）提供，模型自主判断是否调用，
// 不再需要独立的「搜索模型」配置与 Responses 服务端搜索分支。

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
		// 1. 用户自定义提供方（跳过免费池条目，它们下面单独走目录逻辑）。
		// 自动路由时每个提供方只取一个默认模型；用户在下拉框明确选择时，
		// resolveExact 会按“提供方 + 模型”精确路由到该目录里的任意模型。
		for _, e := range entries {
			if (e.APIKey == "" && !e.Keyless) || isFreeCatalogID(e.ID) {
				continue
			}
			defaultModel := strings.TrimSpace(e.DefaultModel)
			if defaultModel == "" {
				if models := configuredProviderModels(e); len(models) > 0 {
					defaultModel = models[0].ID
				}
			}
			if defaultModel == "" {
				continue
			}
			b := RouterBackend{
				Name: e.Name, BaseURL: e.Endpoint, Model: defaultModel,
				APIKey: e.APIKey, Timeout: 5 * time.Minute, Source: "user",
				Vision: e.Vision, ContextWindow: e.ContextWindow, Reasoning: e.Reasoning,
				Keyless: e.Keyless,
			}
			if e.IsDefault {
				userChain = append([]RouterBackend{b}, userChain...)
			} else {
				userChain = append(userChain, b)
			}
		}
	}

	// 3. 免费池：Key 来源 = 用户保存的同 ID 条目 > 同厂商共享 key > 环境变量；
	// 没 Key 的源（Local/Ollama Cloud 走本地路由）直接不进链
	envKeys := userKeysByEnv(userKey)
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
		if key == "" && !f.Local && !f.Keyless {
			key = envKeys[f.KeyEnv] // 同厂商共享个人 key（同 KeyEnv 任一模型配过即可）
		}
		if key == "" && !f.Local && !f.Keyless {
			key = os.Getenv(f.KeyEnv)
		}
		if key == "" && !f.Local && !f.Keyless {
			continue
		}
		source := "free"
		b := RouterBackend{
			Name: f.Name, BaseURL: f.Endpoint, Model: f.Model,
			APIKey: key, ParamsB: f.ParamsB, Timeout: 45 * time.Second, Source: source,
			Vision: f.Vision, ContextWindow: f.ContextWindow, Reasoning: f.Reasoning,
			IsLocal: f.Local, Keyless: f.Keyless, WireResponses: f.Responses,
		}
		b.ID = f.ID
		if circuitOpen(b) {
			// 熔断冷却期：Auto 路由跳过该条目，秒切下一个（精确手选不受影响）
			continue
		}
		if isDefault {
			// 用户显式把某个免费模型设为默认 → 提到链头
			userChain = append([]RouterBackend{b}, userChain...)
			continue
		}
		freeChain = append(freeChain, b)
	}
	// Auto 智能路由顺序 = 探活信号降序 → LRU 使用新鲜度 → 免费模型池排序
	// （设置面板「模型」页调整，存 ~/rescene_data/free_model_order.json）。
	// 目录里没排到的条目（新加/未保存过）按目录声明顺序排在末尾，保持稳定。
	// 信号：探活 4/3/2/1/0（-1 未探测沉底）；LRU：最近真实成功用过的排前。
	orderRank := freeOrderRank()
	sort.SliceStable(freeChain, func(i, j int) bool {
		si, sj := probeSignal(freeChain[i]), probeSignal(freeChain[j])
		if si != sj {
			if si == -1 {
				return false
			}
			if sj == -1 {
				return true
			}
			return si > sj
		}
		ui, uj := freeLastUsed(freeChain[i]), freeLastUsed(freeChain[j])
		if !ui.IsZero() && !uj.IsZero() {
			if !ui.Equal(uj) {
				return ui.After(uj)
			}
		} else if !ui.IsZero() {
			return true
		} else if !uj.IsZero() {
			return false
		}
		ri, iok := orderRank[freeChain[i].ID]
		rj, jok := orderRank[freeChain[j].ID]
		if iok && jok {
			return ri < rj
		}
		return iok // 有排序的在前，没排序的（目录顺序）在后
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
	envKeys := userKeysByEnv(userKey)
	// 免费池自动发现模型（auto_ 前缀）：用户从下拉选中的动态模型，按提供方路由
	if strings.HasPrefix(model, "auto_") {
		if b := resolveAutoDiscovered(userKey, model, envKeys); b != nil {
			return b
		}
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
		if !f.Local && !f.Keyless {
			if e, ok := entryByID[f.ID]; ok {
				key = e.APIKey
			}
			if key == "" {
				key = envKeys[f.KeyEnv] // 同厂商共享个人 key
			}
			if key == "" {
				key = os.Getenv(f.KeyEnv)
			}
		}
		if key == "" && !f.Local && !f.Keyless {
			return nil
		}
		source := "free"
		return &RouterBackend{
			Name: f.Name, BaseURL: f.Endpoint, Model: f.Model,
			APIKey: key, ParamsB: f.ParamsB, Timeout: 45 * time.Second, Source: source,
			Vision: f.Vision, ContextWindow: f.ContextWindow, Reasoning: f.Reasoning,
			IsLocal: f.Local, Keyless: f.Keyless, WireResponses: f.Responses,
		}
	}
	// 自定义提供方目录里的精确模型。选择 ID 同时编码 providerID 和上游 modelID，
	// 避免两个 OpenAI 兼容提供方都暴露同名模型时发生冲突。
	if providerID, upstreamModelID, ok := parseCustomModelSelectionID(model); ok {
		for _, e := range entries {
			if e.ID != providerID || isFreeCatalogID(e.ID) {
				continue
			}
			if e.APIKey == "" && !e.Keyless {
				return nil
			}
			var selected *ModelConfigModel
			for _, candidate := range configuredProviderModels(e) {
				if candidate.ID == upstreamModelID {
					copy := candidate
					selected = &copy
					break
				}
			}
			if selected == nil {
				return nil
			}
			return &RouterBackend{
				Name: e.Name + " · " + selected.Name, BaseURL: e.Endpoint, Model: selected.ID,
				APIKey: e.APIKey, Timeout: 5 * time.Minute, Source: "user",
				Vision:        selected.Vision || e.Vision,
				ContextWindow: selected.ContextWindow,
				Reasoning:     selected.Reasoning || e.Reasoning,
				Keyless:       e.Keyless,
			}
		}
		return nil
	}
	// 用户自定义配置
	for _, e := range entries {
		if e.ID != model || isFreeCatalogID(e.ID) {
			continue
		}
		if e.APIKey == "" && !e.Keyless {
			return nil
		}
		return &RouterBackend{
			Name: e.Name, BaseURL: e.Endpoint, Model: e.DefaultModel,
			APIKey: e.APIKey, Timeout: 5 * time.Minute, Source: "user",
			Vision: e.Vision, ContextWindow: e.ContextWindow, Reasoning: e.Reasoning,
			Keyless: e.Keyless,
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

// responsesURL 归一化 Responses API endpoint（/responses）。
// 与 chatCompletionsURL 同思路：允许用户填根地址（https://api.deepseek.com）或
// 已带 /responses 的完整地址。
func responsesURL(base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if strings.HasSuffix(base, "/responses") {
		return base
	}
	return base + "/responses"
}

// toResponsesInput 把 chat/completions 风格的 messages 转成 Responses API 的
// input items 数组。DeepSeek Responses API 与 OpenAI 兼容：
//   - 消息 → {"type":"message","role":...,"content":[{"type":"input_text","text":...}]}
//   - assistant 的 tool_calls → 拆成独立 {"type":"function_call","call_id",name,arguments}
//   - tool 结果 → {"type":"function_call_output","call_id":...,"output":...}
func toResponsesInput(msgs []map[string]any) []any {
	out := make([]any, 0, len(msgs))
	for _, m := range msgs {
		role, _ := m["role"].(string)
		switch role {
		case "tool":
			callID, _ := m["tool_call_id"].(string)
			content, _ := m["content"].(string)
			if callID == "" {
				continue
			}
			out = append(out, map[string]any{
				"type": "function_call_output", "call_id": callID, "output": content,
			})
		default:
			// user / assistant / system / developer：content 统一按字符串处理
			content, _ := m["content"].(string)
			if content == "" {
				// 有的调用方把 content 塞成数组（罕见），兜底序列化
				if raw, ok := m["content"]; ok && raw != nil {
					if bs, err := json.Marshal(raw); err == nil {
						content = string(bs)
					}
				}
			}
			// DeepSeek 思考模式：assistant 的 reasoning item 必须原样回传
			// （在对应 assistant 消息之前插入 input items），否则 400
			// "reasoning_text in the thinking mode must be passed back"。
			if role == "assistant" {
				if rawItems, ok := m["reasoning_items"]; ok && rawItems != nil {
					if bs, err := json.Marshal(rawItems); err == nil {
						var items []map[string]any
						if json.Unmarshal(bs, &items) == nil {
							for _, it := range items {
								if it["type"] == "reasoning" {
									out = append(out, it)
								}
							}
						}
					}
				}
			}
			out = append(out, map[string]any{
				"type": "message", "role": role,
				"content": []any{map[string]any{"type": "input_text", "text": content}},
			})
			// assistant 消息里的工具调用：拆成独立 function_call item。
			// 若不拆，DeepSeek 服务端会丢失上一轮的工具调用历史，多轮 Agent
			// 循环时 function_call_output 的 call_id 找不到对应声明。
			if role == "assistant" {
				if rawCalls, ok := m["tool_calls"]; ok && rawCalls != nil {
					// 宽容解析：工作流里 tool_calls 可能是 []map[string]any 或
					// []any 或经过 JSON 往返的任意形态——用 JSON 序列化统一归一，
					// 避免 Go slice 类型断言（[]map[string]any → []any 必失败）
					// 把 function_call item 整个漏掉导致 400。
					bs, err := json.Marshal(rawCalls)
					if err == nil {
						var cmList []map[string]any
						if json.Unmarshal(bs, &cmList) == nil {
							for _, cm := range cmList {
								fn, _ := cm["function"].(map[string]any)
								if fn == nil {
									continue
								}
								name, _ := fn["name"].(string)
								argsStr, _ := fn["arguments"].(string)
								callID, _ := cm["id"].(string)
								if callID == "" {
									callID = fmt.Sprintf("call_%d", len(out))
								}
								if name == "" {
									continue
								}
								out = append(out, map[string]any{
									"type": "function_call", "call_id": callID,
									"name": name, "arguments": argsStr,
								})
							}
						}
					}
				}
			}
		}
	}
	return out
}

// toResponsesTools 把 chat/completions 风格的 tools（{type:"function",
// function:{name,description,parameters}}）转成 Responses API 格式
// （{type:"function", name, description, parameters}）。
// 注：曾在这里自动附加 DeepSeek 服务端 web_search 工具，随 DS 服务端搜索
// 一并移除（2026-08-04）——联网搜索已改为常驻 web_search 工具（Firecrawl）。
func toResponsesTools(tools []map[string]any) []any {
	out := make([]any, 0, len(tools))
	for _, t := range tools {
		typ, _ := t["type"].(string)
		if typ != "function" {
			continue
		}
		fn, _ := t["function"].(map[string]any)
		if fn == nil {
			continue
		}
		item := map[string]any{
			"type":        "function",
			"name":        fn["name"],
			"description": fn["description"],
		}
		if params, ok := fn["parameters"].(map[string]any); ok && params != nil {
			item["parameters"] = params
		}
		out = append(out, item)
	}
	return out
}

// ==================== 非流式：openAIChatOnce + 秒切链 ====================

// openAIChatOnce 对单个 backend 发一次非流式 OpenAI 兼容调用。
// b.WireResponses=true 时走 Responses API（/responses），协议差异见
// streamRouterRound 的分支说明；返回口径与 chat/completions 分支完全一致。
func openAIChatOnce(ctx context.Context, b RouterBackend, msgs []map[string]any, tools []map[string]any) (string, []core.ToolCall, error) {
	if b.WireResponses {
		return responsesOnce(ctx, b, msgs, tools)
	}
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
	// 浏览器 UA 必带：OpenCode Zen / Kilo 等免 key 网关有 Cloudflare 防护，
	// Go 默认 UA 会吃 403 error code 1010（2026-08-08 实测根因，agent-os 同修）。
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	if b.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+b.APIKey)
	}

	client := &http.Client{Timeout: b.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		// 连接失败/超时：暂时性故障，计入熔断（Auto 路由冷却期内跳过）
		circuitFail(b)
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		// 401(鉴权)/403(额度)/404(模型不存在) 确定性不可用才标记禁用；
		// 400 属请求格式/上游解析 bug，不禁用。自动发现模型（auto_ 前缀，
		// 不在 freeModelCatalog 里）单独走 autoDisabled 淘汰。
		if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 {
			if strings.HasPrefix(b.ID, "auto_") {
				disableAutoModel(b.BaseURL, b.Model)
			} else {
				disableFreeModel(b.Model)
			}
		} else if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			// 限流 / 服务端故障：暂时性，计入熔断
			circuitFail(b)
			// 额度耗尽判定：429/5xx 响应体带 quota 特征 → 标记耗尽，自动降权
			if markQuotaExhaustedIfError(b, string(raw)) {
				fmt.Printf("⚠️ [额度] %s 判定额度耗尽（%s），Auto 将排到末尾\n", b.Name, truncateChars(string(raw), 120))
			}
		}
		return "", nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateChars(string(raw), 300))
	}
	// 拿到 200：上游可用，清零失败计数
	circuitSuccess(b)

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

// responsesOnce 对单个 backend 发一次非流式 Responses API 调用（/responses）。
// 请求体：input items（由 messages 转换）+ tools（function 转换 + web_search 附加）。
// 响应：output 数组里的 message.output_text 是最终回答，function_call 是工具调用。
// 返回值口径与 openAIChatOnce 的 chat/completions 分支一致，调用方无感知。
func responsesOnce(ctx context.Context, b RouterBackend, msgs []map[string]any, tools []map[string]any) (string, []core.ToolCall, error) {
	reqBody := map[string]any{
		"model": b.Model, "input": toResponsesInput(msgs), "stream": false,
		"max_output_tokens": 4096,
	}
	if len(tools) > 0 {
		reqBody["tools"] = toResponsesTools(tools)
	}
	body, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", responsesURL(b.BaseURL), bytes.NewBuffer(body))
	if err != nil {
		return "", nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// 浏览器 UA 必带：OpenCode Zen / Kilo 等免 key 网关有 Cloudflare 防护（2026-08-08 实测根因）。
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	if b.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+b.APIKey)
	}

	client := &http.Client{Timeout: b.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		// 连接失败/超时：暂时性故障，计入熔断（Auto 路由冷却期内跳过）
		circuitFail(b)
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		// 401(鉴权)/403(额度)/404(模型不存在) 确定性不可用才标记禁用；
		// 400 属请求格式/上游解析 bug，不禁用。自动发现模型走 autoDisabled。
		if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 {
			if strings.HasPrefix(b.ID, "auto_") {
				disableAutoModel(b.BaseURL, b.Model)
			} else {
				disableFreeModel(b.Model)
			}
		} else if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			// 限流 / 服务端故障：暂时性，计入熔断
			circuitFail(b)
			// 额度耗尽判定：429/5xx 响应体带 quota 特征 → 标记耗尽，自动降权
			if markQuotaExhaustedIfError(b, string(raw)) {
				fmt.Printf("⚠️ [额度] %s 判定额度耗尽（%s），Auto 将排到末尾\n", b.Name, truncateChars(string(raw), 120))
			}
		}
		return "", nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateChars(string(raw), 300))
	}
	// 拿到 200：上游可用，清零失败计数
	circuitSuccess(b)

	var parsed struct {
		Output []struct {
			Type    string `json:"type"`
			CallID  string `json:"call_id"`
			Name    string `json:"name"`
			Output  string `json:"output"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			Arguments string `json:"arguments"`
		} `json:"output"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", nil, fmt.Errorf("响应解析失败: %w", err)
	}

	var sb strings.Builder
	var calls []core.ToolCall
	for _, item := range parsed.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" {
					sb.WriteString(c.Text)
				}
			}
		case "function_call":
			calls = append(calls, core.ToolCall{
				ID: item.CallID, Type: "function",
				Function: core.ToolCallFunc{Name: item.Name, Arguments: item.Arguments},
			})
		}
	}
	content := sb.String()
	if content == "" && len(calls) == 0 {
		return "", nil, fmt.Errorf("empty completion")
	}
	return content, calls, nil
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
func (r *WorkflowRunner) streamRouterRound(c *gin.Context, backends []RouterBackend, msgs []map[string]any, tools []map[string]any, effort string, staticSum int, reasoningOut *[]map[string]any) (string, []core.ToolCall, int, int, *RouterBackend, error) {
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
		// DeepSeek 服务端搜索走 Responses API 协议（/responses）：请求体用 input
		// items + tools（自动附加 web_search），响应是语义化 SSE 事件流
		// （response.output_text.delta / web_search_call.* 等，无 [DONE]）。
		// chat/completions 端点拒绝 web_search 工具类型（400），故协议必须分支。
		if b.WireResponses {
			content, calls, inTok, outTok, err := r.streamResponsesRound(c, b, msgs, tools, effort, staticSum, reasoningOut)
			if err != nil {
				tried = append(tried, fmt.Sprintf("%s: %v", b.Name, err))
				fmt.Printf("🔀 [路由] %s (Responses) 失败，秒切下一个: %s\n", b.Name, truncateChars(err.Error(), 120))
				continue
			}
			if len(tried) > 0 {
				fmt.Printf("🔀 [路由] 最终由 %s 承接（此前 %d 个源失败）\n", b.Name, len(tried))
			}
			return content, calls, inTok, outTok, &b, nil
		}
		reqBody := map[string]any{
			"model": b.Model, "messages": msgs, "stream": true,
			// 4k 会把稍长的单文件 HTML 截在 write_file JSON 中间，随后工具必然报
			// unexpected end of JSON input，模型再整份重写，形成“失败—重试”死循环。
			// 工作流自身已有总 token 预算，这里给单轮足够空间完成长文件参数。
			"temperature": 0.2, "top_p": 0.85, "max_tokens": 16384,
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
		// 单轮模型请求的整体超时：派生自请求 context（浏览器断开仍会取消），
		// 但额外兜住「上游 200 头已回、body 流中途冻住不推数据」的场景——
		// 这种半死连接 c.Request.Context() 不会取消（浏览器没断），
		// drainChatStream 的 ReadString 会永久阻塞，表现为「工具跑完后 agent 卡死、
		// 只有心跳在跳、要再戳一下才动」。180s 足够正常长任务，超时就当本轮上游失败，
		// 走现有 err 分支发 flow_error + workflow_done(resumable)，不再冻死整个工作流。
		roundCtx, roundCancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
		defer roundCancel()
		httpReq, err := http.NewRequestWithContext(roundCtx, "POST", chatCompletionsURL(b.BaseURL), bytes.NewBuffer(body))
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
			circuitFail(b)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			// 401(鉴权)/403(额度)/404(模型不存在) 是确定性不可用，当场标记禁用；
			// 400 是请求格式/上游解析问题，属客户端侧，不该永久禁用模型，
			// 否则一棍子打死整个免费档。自动发现模型走 autoDisabled。
			if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 {
				if strings.HasPrefix(b.ID, "auto_") {
					disableAutoModel(b.BaseURL, b.Model)
				} else {
					disableFreeModel(b.Model)
				}
			} else if resp.StatusCode == 429 || resp.StatusCode >= 500 {
				// 限流 / 服务端故障：暂时性，计入熔断
				circuitFail(b)
			}
			reason := fmt.Sprintf("HTTP %d", resp.StatusCode)
			low := strings.ToLower(string(raw))
			if resp.StatusCode == 451 || strings.Contains(low, "censorship") || strings.Contains(low, "blocked") {
				// StepFun 等国内模型服务对联网搜到的敏感新闻做内容审核，整请求 451 拒收
				// （实测：censorship_blocked）。这是明确原因，别让用户误以为是网络/服务在折腾。
				reason = "内容审核拦截(HTTP 451)：模型服务商封了这段内容——联网搜到的新闻敏感时常见。换不审的源（如 Zen 免 key）或换个话题再试"
			}
			tried = append(tried, fmt.Sprintf("%s: %s", b.Name, reason))
			fmt.Printf("🔀 [路由] %s HTTP %d，秒切下一个: %s\n", b.Name, resp.StatusCode, truncateChars(string(raw), 120))
			continue
		}
		circuitSuccess(b)

		if len(tried) > 0 {
			fmt.Printf("🔀 [路由] 流式请求由 %s 承接（此前 %d 个源失败）\n", b.Name, len(tried))
			// 之前的源里如果有因内容审核（451/censorship）被拒的，failover 切下一个源
			// 成功后这段就悄悄过去了——用户只看到答案，不知道联网搜到的内容被上游
			// 审核拦过一次，容易把"换源后答案变差/变敷衍"误判成 web_search 本身坏了
			// （2026-08-20 用户反馈）。这里补一条不阻断流程的提示，让用户知道发生了什么。
			if note := censorshipNoteFromTried(tried); note != "" {
				writeCodeSSE(c, "flow_notice", map[string]any{"message": note})
			}
		}
		content, calls, inTok, outTok, err := drainChatStream(c, resp, msgs, staticSum)
		resp.Body.Close()
		if err != nil {
			// 流式中途失败（上游推到一半冻住 / 我方 180s 读取超时 / 连接被掐）：
			// 免费模型源本就不稳定，这种"进行中"的失败不该直接判死整个工作流——
			// 跟首包前的 failover 同一思路，记进 tried 并切下一个源重试这一轮，
			// 把不稳定的单源抽风吸收掉，agent 才能真正一口气跑完。
			// 先发 flow_error 让前端清掉半截 intent/thinking（避免显示残缺回答），
			// 再 continue 到下一个源。
			tried = append(tried, fmt.Sprintf("%s: %v", b.Name, err))
			fmt.Printf("🔀 [路由] %s 流式中途失败，秒切下一个: %s\n", b.Name, truncateChars(err.Error(), 120))
			writeCodeSSE(c, "flow_error", map[string]any{"message": "上游响应中断，正在切换模型源重试…"})
			continue
		}
		usedBackend := b
		return content, calls, inTok, outTok, &usedBackend, nil
	}
	return "", nil, 0, 0, nil, fmt.Errorf("所有模型源不可用：%s", strings.Join(tried, "；"))
}

// censorshipNoteFromTried 检查本轮 failover 过程中是否有源因内容审核（451/censorship）
// 被跳过；有则给前端一句人话提示，没有则返回空串（调用侧据此决定要不要发 SSE）。
func censorshipNoteFromTried(tried []string) string {
	for _, t := range tried {
		if strings.Contains(t, "内容审核拦截") {
			return "刚才联网搜到的内容被一个模型源的内容审核拦了，已自动换源重试——如果这条回答看起来比较敷衍，可以换个话题或稍后再试"
		}
	}
	return ""
}

// drainChatStream 读一条已建立的 SSE 流，实时转发 thinking/intent 事件。
// 返回真实拆分的 inputTokens/outputTokens：优先取上游 usage.prompt_tokens/completion_tokens，
// 上游不回传时退化为字符/4 估算（与四态机历史口径一致）。
func drainChatStream(c *gin.Context, resp *http.Response, msgs []map[string]any, staticSum int) (string, []core.ToolCall, int, int, error) {
	reader := bufio.NewReader(resp.Body)
	var full strings.Builder
	charCount := 0
	callsMap := map[int]*core.ToolCall{}
	// tool_calls 的 arguments 也是流式 token。以前只累计到 callsMap，等整轮结束后才
	// 发 action，导致前端无法在模型还在生成文件内容时展示红绿 diff。
	// emittedToolStarts 确保每个调用只发一次空 delta，用来尽早创建工具卡片。
	emittedToolStarts := map[int]bool{}
	// 真实 usage：上游在最后一个空 choices chunk 里回传（stream_options.include_usage）
	var inTok, outTok int
	gotUsage := false
	finishReason := ""

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
		if reason, ok := choice["finish_reason"].(string); ok && reason != "" {
			finishReason = reason
		}
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
					if tc.ID != "" && tc.Function.Name != "" && !emittedToolStarts[idx] {
						writeCodeSSE(c, "action_delta", map[string]any{
							// 少数兼容服务会先送 arguments、后送 name；把已累计部分
							// 一次补发，避免前端漏掉文件内容的开头。
							"id": tc.ID, "name": tc.Function.Name, "args_delta": tc.Function.Arguments,
						})
						emittedToolStarts[idx] = true
					}
					if argsStr, ok := fnMap["arguments"].(string); ok {
						tc.Function.Arguments += argsStr
						if tc.ID != "" && tc.Function.Name != "" && argsStr != "" {
							writeCodeSSE(c, "action_delta", map[string]any{
								"id": tc.ID, "name": tc.Function.Name, "args_delta": argsStr,
							})
						}
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
	// 截断或坏 JSON 绝不能送进工具执行。以前它会显示成一次普通工具失败，
	// 模型下一轮又重发整份长文件，即使已生成上万字符也永远无法成功。
	for _, tc := range calls {
		if finishReason == "length" || !json.Valid([]byte(tc.Function.Arguments)) {
			return full.String(), nil, inTok, outTok, fmt.Errorf(
				"模型输出在工具参数完成前被截断；本轮未执行 %s，请缩短单次内容或改用分段写入",
				tc.Function.Name,
			)
		}
	}
	// 上游没回传 usage：用字符/4 估算兜底（与四态机 input 估算口径同源）
	if !gotUsage {
		outTok = charCount / 4
		// inputTokens 也要估算：上游不返回 prompt_tokens 时，按 msgs 内容字符/4
		// 再加上静态部分（system/tools/skill/subagent/memory），得到完整 prompt tokens。
		// conversationTokens(inTok, staticSum) 才能算出正确的对话部分。
		charSum := 0
		for _, m := range msgs {
			if s, ok := m["content"].(string); ok {
				charSum += len(s)
			}
		}
		if charSum > 0 {
			inTok = charSum/4 + staticSum
		}
	}
	return full.String(), calls, inTok, outTok, nil
}

// ==================== Responses API 协议（DeepSeek 服务端搜索） ====================

// streamResponsesRound 对单个 Responses 协议 backend 发一次流式调用（/responses）。
// 请求体：input items + tools（function 转换 + 自动附加 web_search，模型可自主搜索）。
// 流式响应是语义化 SSE 事件（无 [DONE]）：output_text.delta→intent、
// reasoning_text.delta→thinking、function_call_arguments.delta→action_delta、
// web_search_call.*→search 卡片、completed→usage。返回值口径与
// drainChatStream 一致（content/calls/inTok/outTok），failover 语义由调用方处理。
func (r *WorkflowRunner) streamResponsesRound(c *gin.Context, b RouterBackend, msgs []map[string]any, tools []map[string]any, effort string, staticSum int, reasoningOut *[]map[string]any) (string, []core.ToolCall, int, int, error) {
	reqBody := map[string]any{
		"model": b.Model, "input": toResponsesInput(msgs), "stream": true,
		"max_output_tokens": 16384,
		// 引导模型执行 open_page：DS 服务端搜索的 URL 只在 open_page 动作里暴露，
		// 不 open_page 则客户端拿不到任何引用来源（2026-08-01 实测：纯 search
		// 动作只有 queries 无 url）。宣传期开放 4 个页面供视频/宣传素材引用，
		// 宣传结束后改回 1 个或关闭（见「宣传完就关」备注）。
		"instructions": "当需要引用来源时，搜索后必须使用 open_page 打开 2-4 个最相关的页面作为引用来源；不要打开超过 4 个页面，不要为了展示而重复搜索。",
	}
	if len(tools) > 0 {
		reqBody["tools"] = toResponsesTools(tools)
	}
	// DeepSeek 思考强度：responses 协议用 reasoning.effort 字段（chat/completions
	// 是 reasoning_effort 顶层字段）。只有支持思考的 backend 才带。
	if effort != "" && b.Reasoning {
		reqBody["reasoning"] = map[string]any{"effort": effort}
	}
	body, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), "POST", responsesURL(b.BaseURL), bytes.NewBuffer(body))
	if err != nil {
		return "", nil, 0, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if b.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+b.APIKey)
	}

	client := streamHTTPClient()
	resp, err := client.Do(httpReq)
	if err != nil {
		// 连接失败/超时：暂时性故障，计入熔断
		circuitFail(b)
		return "", nil, 0, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		// 401(鉴权)/403(额度)/404(模型不存在) 是确定性不可用，当场标记禁用；
		// 400 是请求格式/上游解析问题，属客户端侧，不该永久禁用模型。
		// 自动发现模型走 autoDisabled。
		if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 {
			if strings.HasPrefix(b.ID, "auto_") {
				disableAutoModel(b.BaseURL, b.Model)
			} else {
				disableFreeModel(b.Model)
			}
		} else if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			// 限流 / 服务端故障：暂时性，计入熔断
			circuitFail(b)
		}
		return "", nil, 0, 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateChars(string(raw), 300))
	}
	// 拿到 200：上游可用，清零失败计数
	circuitSuccess(b)
	defer resp.Body.Close()

	content, calls, inTok, outTok, err := drainResponsesStream(c, resp, msgs, staticSum, reasoningOut)
	return content, calls, inTok, outTok, err
}

// drainResponsesStream 读一条已建立的 Responses API SSE 流，实时转发
// thinking/intent/action_delta/search 事件。与 drainChatStream 的差异：
//   - 事件是语义化的（response.output_text.delta 等），不是 choices[].delta
//   - 流以 response.completed / response.incomplete / response.failed 结束，无 [DONE]
//   - web_search_call.* 事件映射成前端 web_search 工具卡片（action+result）
func drainResponsesStream(c *gin.Context, resp *http.Response, msgs []map[string]any, staticSum int, reasoningOut *[]map[string]any) (string, []core.ToolCall, int, int, error) {
	reader := bufio.NewReader(resp.Body)
	var full strings.Builder
	charCount := 0
	// function_call 流式参数：按 item_id 归集（call_xx 或 uuid），
	// name 从 output_item.added 的 item 里取；item_id 跨事件稳定。
	type fnState struct {
		id      string
		name    string
		args    string
		emitted bool
	}
	fnMap := map[string]*fnState{}
	fnOrder := []string{}
	// web_search_call 聚合状态：DeepSeek 一次响应里会有多次搜索动作
	// （search → open_page → find_in_page…），全部聚合到同一张
	// 「联网搜索」卡片（固定 id），前端复用同一 block 显示全部引用来源。
	// 注：DeepSeek 服务端搜索的 URL 只在 output_item.done 的 open_page
	// 动作里出现（completed 事件 action=null，2026-08-01 实测）——
	// 纯搜索动作只有 queries，模型拿到的是注入的摘要，URL 不暴露。
	type searchState struct {
		id      string
		queries []string
		urls    []string
		status  string
	}
	searchCardID := "web_search"
	searchAgg := &searchState{id: searchCardID}
	searchStarted := false
	var inTok, outTok int
	gotUsage := false

	writeSearch := func(ss *searchState, status string) {
		// 前端按 web_search 工具卡片渲染（VERBS 已有映射「联网搜索」）。
		// 注意 result 事件必须带 ok 字段——前端 t.status = d.ok ? 'ok' : 'error'
		// 只认 ok，不带就是恒"失败"（2026-08-01 实测踩坑）。
		// queries 里 DeepSeek 会混入 ws_call_id=call_xx 这种服务端追踪尾巴，
		// 拼进展示文本很难看，过滤掉。
		var cleanQueries []string
		for _, q := range ss.queries {
			if strings.Contains(q, "ws_call_id=") {
				continue
			}
			cleanQueries = append(cleanQueries, q)
		}
		args := map[string]any{"query": strings.Join(cleanQueries, "；")}
		// 引用 URL 剥掉 DeepSeek 服务端追踪尾巴（#ws_call_id=call_xx）——
		// 那是搜索的内部标识，展示给用户只会污染来源链接。
		cleanURLs := make([]string, 0, len(ss.urls))
		for _, u := range ss.urls {
			if i := strings.Index(u, "#ws_call_id="); i >= 0 {
				u = u[:i]
			}
			if strings.TrimSpace(u) == "" {
				continue // 剥完变成空串的无效 URL 不展示
			}
			cleanURLs = append(cleanURLs, u)
		}
		if len(cleanURLs) > 0 {
			args["urls"] = cleanURLs
		}
		writeCodeSSE(c, "action", map[string]any{
			"id": ss.id, "name": "web_search", "args": mustJSON(args),
		})
		writeCodeSSE(c, "result", map[string]any{
			"id": ss.id, "name": "web_search", "ok": true, "status": status,
			"output": strings.Join(cleanURLs, "\n"),
		})
	}

	// accumulateSearchAction 把一次搜索动作（queries/url）聚合进 searchAgg，
	// URL 去重——同一轮搜索可能多事件重复报同一来源。
	accumulateSearchAction := func(ss *searchState, act map[string]any) {
		if qs, ok := act["queries"].([]any); ok {
			for _, q := range qs {
				if s, ok := q.(string); ok && s != "" {
					dup := false
					for _, e := range ss.queries {
						if e == s {
							dup = true
							break
						}
					}
					if !dup {
						ss.queries = append(ss.queries, s)
					}
				}
			}
		}
		if u, ok := act["url"].(string); ok && u != "" {
			// 统一剥掉 DeepSeek 服务端追踪尾巴（#ws_call_id=call_xx），作为
			// canonical key：ss.urls 与前端查找全用干净 URL。
			if i := strings.Index(u, "#ws_call_id="); i >= 0 {
				u = u[:i]
			}
			if strings.TrimSpace(u) == "" {
				return
			}
			dup := false
			for _, e := range ss.urls {
				if e == u {
					dup = true
					break
				}
			}
			if !dup {
				ss.urls = append(ss.urls, u)
			}
		}
	}

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
		typ, _ := ev["type"].(string)
		switch typ {
		case "response.reasoning_text.delta":
			if d, ok := ev["delta"].(string); ok && d != "" {
				charCount += len(d)
				writeCodeSSE(c, "thinking", map[string]any{"content": d})
			}
		case "response.output_text.delta":
			if d, ok := ev["delta"].(string); ok && d != "" {
				charCount += len(d)
				full.WriteString(d)
				writeCodeSSE(c, "intent", map[string]any{"content": d})
			}
		case "response.output_item.added":
			item, _ := ev["item"].(map[string]any)
			if item == nil {
				continue
			}
			switch item["type"] {
			case "function_call":
				id, _ := item["id"].(string)
				name, _ := item["name"].(string)
				if id == "" {
					continue
				}
				if _, ok := fnMap[id]; !ok {
					fnMap[id] = &fnState{id: id, name: name}
					fnOrder = append(fnOrder, id)
					if name != "" {
						// 尽早创建工具卡片（同 drainChatStream 的 emittedToolStarts）
						writeCodeSSE(c, "action_delta", map[string]any{
							"id": id, "name": name, "args_delta": "",
						})
						fnMap[id].emitted = true
					}
				}
			case "web_search_call":
				// 聚合到同一张卡片：第一次出现时创建，后续只累积
				if len(searchAgg.queries) == 0 && len(searchAgg.urls) == 0 {
					writeCodeSSE(c, "action_delta", map[string]any{
						"id": searchCardID, "name": "web_search", "args_delta": "",
					})
				}
			}
		case "response.function_call_arguments.delta":
			itemID, _ := ev["item_id"].(string)
			d, _ := ev["delta"].(string)
			if itemID == "" || d == "" {
				continue
			}
			st, ok := fnMap[itemID]
			if !ok {
				st = &fnState{id: itemID}
				fnMap[itemID] = st
				fnOrder = append(fnOrder, itemID)
			}
			st.args += d
			if st.name != "" {
				writeCodeSSE(c, "action_delta", map[string]any{
					"id": itemID, "name": st.name, "args_delta": d,
				})
			}
		case "response.web_search_call.searching":
			// 搜索开始：立刻建卡片（前端显示「搜索中…」扫描线），
			// 不等 output_item.done——否则汇报完才见卡片。
			if !searchStarted {
				searchStarted = true
				writeCodeSSE(c, "action_delta", map[string]any{
					"id": searchCardID, "name": "web_search", "args_delta": "",
				})
			}
			searchAgg.status = "searching"
			// 某些实现 searching 事件不带 URL，只有 item_id；保持卡片在跑
			writeSearch(searchAgg, "searching")
		case "response.web_search_call.completed":
			itemID, _ := ev["item_id"].(string)
			if itemID == "" {
				continue
			}
			// completed 事件也可能携带完整 action（queries/url）——
			// 实测 DeepSeek 此事件 action=null，URL 在 output_item.done 才到。
			if act, ok := ev["action"].(map[string]any); ok {
				accumulateSearchAction(searchAgg, act)
			}
			searchAgg.status = "completed"
			// 实测：URL 在后续 output_item.done 才到。此时还没聚合到 URL
			// 就写 completed 空卡，前端会闪「未找到相关来源」——保持
			// 「搜索中」扫描线等 output_item.done 聚合完再一次性展示。
			if len(searchAgg.urls) == 0 {
				writeSearch(searchAgg, "searching")
			} else {
				writeSearch(searchAgg, "completed")
			}
		case "response.output_item.done":
			item, _ := ev["item"].(map[string]any)
			if item == nil {
				continue
			}
			switch item["type"] {
			case "web_search_call":
				// 所有搜索动作聚合到同一张卡片：queries/urls 持续累积
				if st, ok := item["status"].(string); ok {
					searchAgg.status = st
				}
				if act, ok := item["action"].(map[string]any); ok {
					accumulateSearchAction(searchAgg, act)
				}
				// 实测：DeepSeek 的 URL 在 output_item.done 的 open_page 动作里。
				// 聚合后写完整卡片（主网站列表）；searching 状态保持扫描线。
				// 注意：search 动作的 done 只有 queries 没有 urls，此时不能发
				// completed——前端会渲染「未找到相关来源」；必须等 open_page
				// 动作的 URL 聚合到再一次性展示（2026-08-01 实测踩坑）。
				if len(searchAgg.urls) == 0 {
					writeSearch(searchAgg, "searching")
				} else {
					writeSearch(searchAgg, searchAgg.status)
				}
			case "function_call":
				id, _ := item["id"].(string)
				if st, ok := fnMap[id]; ok {
					if name, ok := item["name"].(string); ok && name != "" {
						st.name = name
						if !st.emitted {
							writeCodeSSE(c, "action_delta", map[string]any{
								"id": id, "name": name, "args_delta": st.args,
							})
							st.emitted = true
						}
					}
				}
			case "reasoning":
				// DeepSeek 思考模式铁律：assistant 发起工具调用后，下一轮输入
				// 必须把 reasoning item（含 id + content）原样回传，否则 400
				// "reasoning_text in the thinking mode must be passed back"。
				// 这里把完整的 reasoning item 存进 reasoningOut，由工作流
				// 挂到下一轮 assistant 消息上，toResponsesInput 再转回 input items。
				if reasoningOut != nil {
					*reasoningOut = append(*reasoningOut, item)
				}
			}
		case "response.completed":
			// 最后一个事件，携带含 usage 的完整 response 对象
			if usage, ok := ev["usage"].(map[string]any); ok {
				if it, ok := usage["input_tokens"].(float64); ok {
					inTok = int(it)
				}
				if ot, ok := usage["output_tokens"].(float64); ok {
					outTok = int(ot)
				}
				gotUsage = true
			}
			// 收尾：搜索已开始但 urls 始终为空（模型只 search 未 open_page，
			// 直接基于注入摘要作答）——补发一次 completed，否则前端永远停在
			// searching 扫描线。此时「未找到相关来源」是真实状态而非中间态。
			if searchStarted && len(searchAgg.urls) == 0 {
				writeSearch(searchAgg, "completed")
			}
		case "response.incomplete":
			// 截断（如 max_output_tokens 到顶），语义同 chat/completions 的 finish_reason=length
		case "response.failed":
			errMsg, _ := ev["error"].(map[string]any)
			msg := "Responses API 失败"
			if errMsg != nil {
				if m, ok := errMsg["message"].(string); ok && m != "" {
					msg = m
				}
			}
			return full.String(), nil, inTok, outTok, fmt.Errorf("%s", truncateChars(msg, 200))
		}
	}

	var calls []core.ToolCall
	for _, id := range fnOrder {
		st := fnMap[id]
		if st == nil || st.name == "" {
			continue
		}
		calls = append(calls, core.ToolCall{
			ID: st.id, Type: "function",
			Function: core.ToolCallFunc{Name: st.name, Arguments: st.args},
		})
	}
	// 坏 JSON 参数绝不送进工具执行（与 drainChatStream 同纪律）
	for _, tc := range calls {
		if !json.Valid([]byte(tc.Function.Arguments)) {
			return full.String(), nil, inTok, outTok, fmt.Errorf(
				"模型输出在工具参数完成前被截断；本轮未执行 %s，请缩短单次内容或改用分段写入",
				tc.Function.Name,
			)
		}
	}
	// 上游没回传 usage：字符/4 估算兜底（与 drainChatStream 口径一致）
	if !gotUsage {
		outTok = charCount / 4
		charSum := 0
		for _, m := range msgs {
			if s, ok := m["content"].(string); ok {
				charSum += len(s)
			}
		}
		if charSum > 0 {
			inTok = charSum/4 + staticSum
		}
	}
	return full.String(), calls, inTok, outTok, nil
}

// mustJSON 把任意值序列化成紧凑 JSON 字符串；失败返回 "{}"（只在构造
// SSE 事件参数时使用，参数错误不该中断整个流）。
func mustJSON(v any) string {
	bs, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(bs)
}

// fetchPageTitle 曾用于抓取引用页面的 <title> 展示新闻标题；用户确认来源行
// 只显示主网站（中文站名 + 域名），不再抓标题。函数保留供未来需要时恢复。
func fetchPageTitle(rawURL string) string {
	// 剥掉 DeepSeek 追踪尾巴
	if i := strings.Index(rawURL, "#ws_call_id="); i >= 0 {
		rawURL = rawURL[:i]
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ""
	}
	// 伪装浏览器 UA：不少新闻站对裸 Go client 返回 403/重定向到验证页
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return ""
	}
	// 编码检测：多数站 UTF-8；GBK 站（老门户）转不了就靠后面正则硬解
	title := extractHTMLTitle(body)
	title = strings.TrimSpace(title)
	// 去站点后缀：「标题_澎湃新闻-The Paper」→「标题」；保留有信息量的部分
	if i := strings.LastIndex(title, "_"); i > 0 && len(title)-i <= 30 {
		title = title[:i]
	}
	title = strings.TrimSpace(title)
	if len(title) > 120 {
		title = title[:120]
	}
	return title
}

// extractHTMLTitle 从 HTML 字节里提取 <title>...</title> 内容（宽松匹配，
// 跨行/属性均可；不引入 HTML 解析依赖）。
func extractHTMLTitle(body []byte) string {
	lower := body
	// 优先处理 UTF-8；GBK 页面 title 区一般也是 ASCII 包裹，正则照常工作
	idx := bytes.Index(lower, []byte("<title"))
	if idx < 0 {
		return ""
	}
	rest := body[idx:]
	gt := bytes.IndexByte(rest, '>')
	if gt < 0 {
		return ""
	}
	inner := rest[gt+1:]
	lt := bytes.Index(inner, []byte("</title"))
	if lt < 0 {
		return ""
	}
	s := string(inner[:lt])
	// 折叠空白（title 常被格式化换行）
	return strings.Join(strings.Fields(s), " ")
}
