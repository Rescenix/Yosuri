package handler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// UserProfile 持续积累的用户画像，不依赖任何自定义指令。
// 每次工作流完成后更新，下次自动注入 system prompt。
type UserProfile struct {
	FirstSeen            time.Time      `json:"first_seen"`
	LastSeen             time.Time      `json:"last_seen"`
	TotalWorkflows       int            `json:"total_workflows"`
	InterruptedWorkflows int            `json:"interrupted_workflows"`
	TotalInputTokens     int            `json:"total_input_tokens"`
	TotalOutputTokens    int            `json:"total_output_tokens"`
	ModelUsage           map[string]int `json:"model_usage"`
	LastNModels          []string       `json:"last_n_models"`
}

func profileFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "rescene_data", "profile.json")
}

// LoadProfile 从本地文件加载用户画像。文件不存在时返回空白画像。
func LoadProfile() *UserProfile {
	p := &UserProfile{ModelUsage: make(map[string]int)}
	path := profileFilePath()
	if path == "" {
		return p
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return p
	}
	json.Unmarshal(data, p)
	if p.ModelUsage == nil {
		p.ModelUsage = make(map[string]int)
	}
	return p
}

// Save 持久化到本地文件。
func (p *UserProfile) Save() error {
	path := profileFilePath()
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// RecordWorkflow 记录一次工作流完成（成功或中断）。
func (p *UserProfile) RecordWorkflow(model string, inputTokens, outputTokens int, interrupted bool) {
	now := time.Now()
	if p.FirstSeen.IsZero() {
		p.FirstSeen = now
	}
	p.LastSeen = now

	if interrupted {
		p.InterruptedWorkflows++
	} else {
		p.TotalWorkflows++
	}
	p.TotalInputTokens += inputTokens
	p.TotalOutputTokens += outputTokens

	if model != "" {
		p.ModelUsage[model]++
		p.LastNModels = append(p.LastNModels, model)
		if len(p.LastNModels) > 5 {
			p.LastNModels = p.LastNModels[len(p.LastNModels)-5:]
		}
	}
}

// Prompt 生成一段自然语言描述，注入到 system prompt 的"记忆"段。
func (p *UserProfile) Prompt() string {
	total := p.TotalWorkflows + p.InterruptedWorkflows
	if total == 0 {
		return ""
	}

	var b []byte
	b = append(b, "\n\n【关于我们】"...)

	b = append(b, "\n我们已合作完成 "...)
	b = append(b, fmt.Sprintf("%d", p.TotalWorkflows)...)
	b = append(b, " 个任务"...)
	if p.InterruptedWorkflows > 0 {
		b = append(b, "，其中 "...)
		b = append(b, fmt.Sprintf("%d", p.InterruptedWorkflows)...)
		b = append(b, " 次被中断"...)
	}
	b = append(b, "。"...)

	totalTokens := p.TotalInputTokens + p.TotalOutputTokens
	if totalTokens > 0 {
		b = append(b, "\n累计消耗 "...)
		b = append(b, formatTokenCount(totalTokens)...)
		b = append(b, " tokens"...)
		b = append(b, "（输入 "...)
		b = append(b, formatTokenCount(p.TotalInputTokens)...)
		b = append(b, " / 输出 "...)
		b = append(b, formatTokenCount(p.TotalOutputTokens)...)
		b = append(b, "）。"...)
	}

	if len(p.ModelUsage) > 0 {
		bestModel, bestCount := "", 0
		for m, c := range p.ModelUsage {
			if c > bestCount {
				bestModel, bestCount = m, c
			}
		}
		if bestModel != "" {
			b = append(b, "\n你最常用的模型是 "...)
			b = append(b, bestModel...)
			b = append(b, "（已使用 "...)
			b = append(b, fmt.Sprintf("%d", bestCount)...)
			b = append(b, " 次）。"...)
		}
	}
	return string(b)
}

func formatTokenCount(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
