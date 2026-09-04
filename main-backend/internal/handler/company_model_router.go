package handler

// company_model_router.go —— 公司生产专用的智能 auto 路由 + 并发竞速负载均衡（2026-09-03）。
//
// 为什么独立：聚合池的 auto（/v1/chat/completions model=auto）内部按「探活信号」把模型排成单链，
// 探活是日级刷新，一旦把慢/思考档排到链头，每次请求都先撞它，撞满 45s 才切下一个；公司侧再套
// 180s×3 重试就变成 9 分钟死等。这里的公司路由**绕开聚合池 auto**：
//   - 健康模型池：复用 resolveBackends 组装好的链（已处理 key/探活/熔断/排序），取前几个健康源；
//   - 并发竞速：同时请求多个健康模型，谁先完整返回非空内容就用谁（快源赢），不再死等单个慢源；
//   - 快速失败：每源短超时，慢/挂源被并发淘汰，绝不阻塞整条生产；
//   - 熔断：连续失败的源由下一轮 resolveBackends 的 circuitOpen 自动剔除。
//
// 这保证：一次指令生产的每个阶段都尽量命中「当前最快、最健康」的模型，坏源自动退场，不会卡死。

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
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// companyRaceN 公司生产并发竞速同时请求的健康模型数（2~3 个最稳：太快源赢、不烧额度）。
var companyRaceN = 3

// companyRaceTimeout 单源超时：并发竞速下，慢源超时不会拖累整体（快的先回就赢了）。
const companyRaceTimeout = 20 * time.Second

// companyStreamTimeout 流式源超时：出字比整段返回慢，给更长的窗口，但仍有上限防挂死。
const companyStreamTimeout = 90 * time.Second

// newLineScanner SSE 行读取器（响应体按行切 data: 帧）。
func newLineScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return scanner
}

// companySysPrompt 公司生产的系统身份提示（与聚合池同类，保证输出真实有温度的中文）。
const companySysPrompt = "你是 Rescene AI 公司的作者，写真实有温度的中文内容。"

// companyModelBackends 返回公司生产可用的健康模型 backend（跳过走 /responses 的源、禁用/熔断源）。
// 复用 resolveBackends("", "auto")：它已把 key、探活信号、熔断、排序全部处理妥当，
// 我们只取前 companyRaceN 个「可并发打上游」的 backend（chat/completions 协议）。
func companyModelBackends() []RouterBackend {
	chain := resolveBackends("", "auto")
	if len(chain) == 0 {
		return nil
	}
	var out []RouterBackend
	for _, b := range chain {
		if b.WireResponses { // 走 /responses 的源（联网搜索类）不参与竞速，chat/completions 更稳
			continue
		}
		if len(out) >= companyRaceN {
			break
		}
		out = append(out, b)
	}
	return out
}

// companyModelRace 并发竞速：同时请求多个健康模型，谁先完整返回非空内容就用谁。
// 其余请求在胜出后经 ctx cancel 放弃，不阻塞、不等待死源。所有源都失败才报错。
func companyModelRace(prompt string) (string, error) {
	backends := companyModelBackends()
	if len(backends) == 0 {
		return "", fmt.Errorf("公司模型池无可用健康源")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		content string
		err     error
	}
	ch := make(chan result, len(backends))
	var wg sync.WaitGroup
	for _, b := range backends {
		wg.Add(1)
		go func(bk RouterBackend) {
			defer wg.Done()
			content, err := chatBackend(ctx, bk, prompt)
			select {
			case ch <- result{content, err}:
			case <-ctx.Done():
			}
		}(b)
	}

	var firstValid string
	done := 0
	for firstValid == "" && done < len(backends) {
		r := <-ch
		done++
		if r.err == nil && strings.TrimSpace(r.content) != "" {
			firstValid = r.content
			break
		}
	}
	// 胜出后取消并等待其余 goroutine 退出，避免泄漏。
	cancel()
	wg.Wait()

	if firstValid != "" {
		return firstValid, nil
	}
	return "", fmt.Errorf("公司模型池所有源调用均失败")
}

// chatBackend 对单个 OpenAI 兼容 backend 发一次 chat/completions，受 ctx 控制（超时/取消）。
func chatBackend(ctx context.Context, b RouterBackend, prompt string) (string, error) {
	body := map[string]any{
		"model": b.Model,
		"messages": []map[string]any{
			{"role": "system", "content": companySysPrompt},
			{"role": "user", "content": prompt},
		},
		// 产出 token 上限：由前端「公司设置」里的「产出输出 token 数」可配置（默认 16384）。
		// 之前写死 max_tokens=2048 会把模型产物截断在半截（按钮在、事件没绑上 → 点击无反应）。
		"max_tokens":  companyMaxTokens(),
		"temperature": 0.8,
		"stream":      false,
	}
	reqBytes, _ := json.Marshal(body)

	// 每源单独超时：并发竞速下，超时的源快速失败，不拖垮整体。
	srcCtx, cancel := context.WithTimeout(ctx, companyRaceTimeout)
	defer cancel()

	base := strings.TrimRight(b.BaseURL, "/")
	url := base + "/chat/completions"
	req, _ := http.NewRequestWithContext(srcCtx, "POST", url, bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	if b.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+b.APIKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("%s 聚合 API HTTP %d", b.Name, resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("%s 空响应", b.Name)
	}
	return out.Choices[0].Message.Content, nil
}

// chatBackendStream 对单个 backend 发一次流式 chat/completions，把增量文本回调给 onDelta。
// 直播大屏靠它看到模型正在写什么，而不是等 20 秒后整段砸出来。
func chatBackendStream(ctx context.Context, b RouterBackend, prompt string, onDelta func(string)) (string, error) {
	body := map[string]any{
		"model": b.Model,
		"messages": []map[string]any{
			{"role": "system", "content": companySysPrompt},
			{"role": "user", "content": prompt},
		},
		"max_tokens":  companyMaxTokens(),
		"temperature": 0.8,
		"stream":      true,
	}
	reqBytes, _ := json.Marshal(body)
	srcCtx, cancel := context.WithTimeout(ctx, companyStreamTimeout)
	defer cancel()

	base := strings.TrimRight(b.BaseURL, "/")
	req, _ := http.NewRequestWithContext(srcCtx, "POST", base+"/chat/completions", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if b.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+b.APIKey)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("%s 流式 API HTTP %d", b.Name, resp.StatusCode)
	}

	var full strings.Builder
	scanner := newLineScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(payload), &chunk) != nil || len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}
		full.WriteString(delta)
		if onDelta != nil {
			onDelta(delta)
		}
	}
	out := full.String()
	if strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("%s 空响应", b.Name)
	}
	return out, nil
}

// companyModelRaceStream 并发竞速 + 流式直播：谁先出字，大屏就跟着谁的字往外长。
// 与 companyModelRace 同样的取舍——快源赢，慢源被 cancel；全部失败才报错。
func companyModelRaceStream(prompt string, onDelta func(string)) (string, error) {
	backends := companyModelBackends()
	if len(backends) == 0 {
		return "", fmt.Errorf("公司模型池无可用健康源")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		content string
		err     error
	}
	ch := make(chan result, len(backends))
	var wg sync.WaitGroup
	var mu sync.Mutex
	winner := ""
	for _, b := range backends {
		wg.Add(1)
		go func(bk RouterBackend) {
			defer wg.Done()
			content, err := chatBackendStream(ctx, bk, prompt, func(delta string) {
				mu.Lock()
				if winner == "" {
					winner = bk.Name // 第一个出字的源接管大屏
				}
				lead := winner == bk.Name
				mu.Unlock()
				if lead && onDelta != nil {
					onDelta(delta)
				}
			})
			select {
			case ch <- result{content, err}:
			case <-ctx.Done():
			}
		}(b)
	}
	var firstValid string
	done := 0
	for firstValid == "" && done < len(backends) {
		r := <-ch
		done++
		if r.err == nil && strings.TrimSpace(r.content) != "" {
			firstValid = r.content
		}
	}
	cancel()
	wg.Wait()
	if firstValid != "" {
		return firstValid, nil
	}
	return "", fmt.Errorf("公司模型池所有源调用均失败")
}

// companyModelConfig 公司生产模型配置（前端「公司设置」可改，存 ~/rescene_data/company/config.json）。
type companyModelConfig struct {
	MaxTokens int `json:"max_tokens"`
}

func companyModelConfigPath() string { return filepath.Join(companyDir(), "config.json") }

// loadCompanyModelConfig 读取公司模型配置；缺失/非法时回退默认 16384，并做 clamp（512~65536）。
func loadCompanyModelConfig() companyModelConfig {
	def := companyModelConfig{MaxTokens: 16384}
	data, err := os.ReadFile(companyModelConfigPath())
	if err != nil {
		return def
	}
	var c companyModelConfig
	if json.Unmarshal(data, &c) != nil {
		return def
	}
	if c.MaxTokens <= 0 {
		c.MaxTokens = 16384
	}
	if c.MaxTokens < 512 {
		c.MaxTokens = 512
	}
	if c.MaxTokens > 65536 {
		c.MaxTokens = 65536
	}
	return c
}

// companyMaxTokens 公司生产模型调用的产出 token 上限（前端可配置）。
func companyMaxTokens() int { return loadCompanyModelConfig().MaxTokens }

// HandleCompanyModelConfigGET GET /api/company/model-config —— 前端「公司设置」读取当前产出 token 上限。
func HandleCompanyModelConfigGET(c *gin.Context) {
	c.JSON(http.StatusOK, loadCompanyModelConfig())
}

// HandleCompanyModelConfigPUT PUT /api/company/model-config —— 前端「公司设置」保存产出 token 上限。
func HandleCompanyModelConfigPUT(c *gin.Context) {
	var req companyModelConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	cfg := loadCompanyModelConfig()
	if req.MaxTokens > 0 {
		cfg.MaxTokens = req.MaxTokens
	}
	if cfg.MaxTokens < 512 {
		cfg.MaxTokens = 512
	}
	if cfg.MaxTokens > 65536 {
		cfg.MaxTokens = 65536
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(companyModelConfigPath(), b, 0o644)
	c.JSON(http.StatusOK, cfg)
}
