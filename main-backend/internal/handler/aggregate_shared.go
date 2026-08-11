package handler

// aggregate_shared.go — 共用函数：callLocalAggregate
// 原定义在 ai_write_handler.go（2026-08-08 该文件随「写作工坊」删除），
// 但 ai_ppt_handler.go / ai_project_handler.go / company_workflow.go 仍在使用，
// 故移到此文件保留。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// callLocalAggregate 调本地 /v1/chat/completions（聚合免费模型池）
func callLocalAggregate(prompt string) (string, error) {
	body := map[string]any{
		"model": "auto",
		"messages": []map[string]any{
			{"role": "system", "content": "你是 Rescene AI 公司的作者，写真实有温度的中文内容。"},
			{"role": "user", "content": prompt},
		},
		"max_tokens":   2048,
		"temperature":  0.8,
		"stream":       false,
	}
	reqBytes, _ := json.Marshal(body)

	client := &http.Client{Timeout: 90 * time.Second}
	req, _ := http.NewRequest("POST", "http://127.0.0.1:8080/v1/chat/completions", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+aggregateAPIKey()) // 聚合 API 鉴权（之前漏了 → 一直 401）
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("聚合 API HTTP %d", resp.StatusCode)
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
		return "", fmt.Errorf("空响应")
	}
	return out.Choices[0].Message.Content, nil
}
