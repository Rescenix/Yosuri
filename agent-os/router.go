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
	"strings"
	"sync"
	"time"
)

// FreeModel 免费模型条目
type FreeModel struct {
	ID       string // 唯一标识，如 free_zen_deepseek_v4_flash
	Vendor   string
	Name     string
	Endpoint string // 如 https://opencode.ai/zen/v1
	Model    string // 上游模型名，如 deepseek-v4-flash-free
	KeyEnv   string // 环境变量名，空=免 key
	KeyURL   string // 申请 Key 的页面
	Keyless  bool   // 免 key 网关
	Vision   bool
	Reasoning bool
	CtxWindow int
	ParamsB  float64 // 参数规模
}

// 免费模型目录 — 直接从 re0 的 freeModelCatalog 同步
var freeModels = []FreeModel{
	// —— OpenCode Zen 免 key 网关 ——
	{ID: "free_zen_deepseek_v4_flash", Vendor: "OpenCode Zen", Name: "DeepSeek V4 Flash（免费）", Endpoint: "https://opencode.ai/zen/v1", Model: "deepseek-v4-flash-free", Keyless: true, Reasoning: true},
	{ID: "free_zen_mimo_v2_5", Vendor: "OpenCode Zen", Name: "Mimo 2.5（免费）", Endpoint: "https://opencode.ai/zen/v1", Model: "mimo-v2.5-free", Keyless: true, Reasoning: true},
	{ID: "free_zen_north_mini_code", Vendor: "OpenCode Zen", Name: "North Mini Code（免费·最快）", Endpoint: "https://opencode.ai/zen/v1", Model: "north-mini-code-free", Keyless: true, Reasoning: true},

	// —— 阶跃星辰 StepFun ——
	{ID: "free_step_1o_turbo_vision", Vendor: "阶跃星辰", Name: "step-1o-turbo-vision（识图）", Endpoint: "https://api.stepfun.com/v1", Model: "step-1o-turbo-vision", KeyEnv: "STEP_API_KEY", Vision: true, Reasoning: true, KeyURL: "https://platform.stepfun.com/"},
	{ID: "free_step_3_7_flash", Vendor: "阶跃星辰", Name: "step-3.7-flash（免费）", Endpoint: "https://api.stepfun.com/v1", Model: "step-3.7-flash", KeyEnv: "STEP_API_KEY", KeyURL: "https://platform.stepfun.com/"},

	// —— SenseNova 商汤 ——
	{ID: "free_sensenova_6_7_flash_lite", Vendor: "SenseNova", Name: "SenseNova 6.7 Flash-Lite（免费）", Endpoint: "https://token.sensenova.cn/v1", Model: "sensenova-6.7-flash-lite", KeyEnv: "SENSENOVA_API_KEY", Vision: true, CtxWindow: 262144, Reasoning: true, KeyURL: "https://platform.sensenova.cn/console/keys"},
	{ID: "free_sensenova_deepseek_v4_flash", Vendor: "SenseNova", Name: "DeepSeek V4 Flash（商汤·免费）", Endpoint: "https://token.sensenova.cn/v1", Model: "deepseek-v4-flash", KeyEnv: "SENSENOVA_API_KEY", CtxWindow: 1048576, Reasoning: true, KeyURL: "https://platform.sensenova.cn/console/keys"},
	{ID: "free_sensenova_glm_5_2", Vendor: "SenseNova", Name: "GLM-5.2（商汤·免费）", Endpoint: "https://token.sensenova.cn/v1", Model: "glm-5.2", KeyEnv: "SENSENOVA_API_KEY", Reasoning: true, KeyURL: "https://platform.sensenova.cn/console/keys"},

	// —— ModelScope 魔搭 ——
	{ID: "free_modelscope_qwen3_5_397b", Vendor: "ModelScope", Name: "Qwen3.5-397B（免费·每日2000次）", Endpoint: "https://api-inference.modelscope.cn/v1", Model: "Qwen/Qwen3.5-397B-A17B", KeyEnv: "MODELSCOPE_API_KEY", ParamsB: 397, Reasoning: true, KeyURL: "https://modelscope.cn"},
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
		openUntil: now.Add(30 * time.Second),
		failCount: 1,
	})
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
}

// InitRouter 初始化路由：过滤出可用模型
func InitRouter() {
	refreshModels()
}

func refreshModels() {
	var available []FreeModel
	for _, m := range freeModels {
		if m.Keyless {
			available = append(available, m)
			continue
		}
		key := os.Getenv(m.KeyEnv)
		if key != "" {
			available = append(available, m)
		}
	}
	wmMu.Lock()
	workingModels = available
	wmMu.Unlock()
}

// GetWorkingModels 返回当前可用模型列表
func GetWorkingModels() []FreeModel {
	wmMu.RLock()
	defer wmMu.RUnlock()
	out := make([]FreeModel, len(workingModels))
	copy(out, workingModels)
	return out
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model       string           `json:"model"`
	Messages    []ChatMessage    `json:"messages"`
	Stream      bool             `json:"stream"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature float64          `json:"temperature"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Complete 发送聊天请求到模型，自动 failover
func Complete(ctx context.Context, req ChatRequest, onChunk func(string)) (string, error) {
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

	// 优先选免 key 模型（Zen 网关），作为默认
	var lastErr error
	for _, m := range models {
		if circuitIsOpen(m) {
			continue
		}

		key := ""
		if !m.Keyless {
			key = os.Getenv(m.KeyEnv)
		}

		// 复制请求，设置当前模型
		body := map[string]any{
			"model":       m.Model,
			"messages":    req.Messages,
			"stream":      req.Stream,
			"max_tokens":  req.MaxTokens,
			"temperature": req.Temperature,
		}

		jsonBody, err := json.Marshal(body)
		if err != nil {
			lastErr = err
			continue
		}

		url := strings.TrimRight(m.Endpoint, "/")
		if !strings.HasSuffix(url, "/chat/completions") {
			url += "/chat/completions"
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
		if err != nil {
			lastErr = err
			continue
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if key != "" {
			httpReq.Header.Set("Authorization", "Bearer "+key)
		}

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			circuitFail(m)
			lastErr = fmt.Errorf("[%s] %v", m.Name, err)
			continue
		}

		if resp.StatusCode != 200 {
			bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			resp.Body.Close()
			circuitFail(m)
			lastErr = fmt.Errorf("[%s] HTTP %d: %s", m.Name, resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
			continue
		}

		// 成功
		circuitSuccess(m)

		if req.Stream {
			// 流式读取
			content, err := readStream(resp.Body, onChunk)
			resp.Body.Close()
			return content, err
		}

		// 非流式
		var chatResp ChatResponse
		if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
			resp.Body.Close()
			circuitFail(m)
			lastErr = fmt.Errorf("[%s] 解析响应失败: %v", m.Name, err)
			continue
		}
		resp.Body.Close()

		if len(chatResp.Choices) > 0 {
			return chatResp.Choices[0].Message.Content, nil
		}
		lastErr = fmt.Errorf("[%s] 空响应", m.Name)
		continue
	}

	return "", fmt.Errorf("所有模型均失败: %v", lastErr)
}

// readStream 读取 SSE 流
func readStream(body io.ReadCloser, onChunk func(string)) (string, error) {
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
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			content := chunk.Choices[0].Delta.Content
			fullContent.WriteString(content)
			if onChunk != nil {
				onChunk(content)
			}
		}
	}

	return fullContent.String(), nil
}