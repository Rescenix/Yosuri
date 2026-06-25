// internal/memory/query_analyzer.go
package memory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type UserIntent struct {
	Intent   string   `json:"intent"`
	Topic    string   `json:"topic"`
	Emotion  string   `json:"emotion"`
	Entities []string `json:"entities"`
	Keywords []string `json:"keywords"`
}

func AnalyzeUserIntent(message string) (*UserIntent, error) {
	prompt := fmt.Sprintf(`你是一个查询意图分析器。
请分析用户消息，提取核心意图、话题、情绪倾向、关键实体和扩展关键词。

直接输出一个JSON对象，不要任何解释，不要markdown代码块。

输出格式：
{"intent":"memory_recall","topic":"emotion","emotion":"angry","entities":["抄袭","代码"],"keywords":["最生气","愤怒","抄袭代码"]}

用户消息：
%s

请直接输出JSON：`, message)

	response, err := callAnalyzerLLM(prompt)
	if err != nil {
		return nil, fmt.Errorf("intent analysis failed: %w", err)
	}

	jsonStr := cleanJSONResponse(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("empty response from intent analysis")
	}

	var intent UserIntent
	if err := json.Unmarshal([]byte(jsonStr), &intent); err != nil {
		return nil, fmt.Errorf("failed to parse intent JSON: %w, raw: %s", err, jsonStr)
	}

	return &intent, nil
}

func callAnalyzerLLM(prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":  "qwen2.5-coder:7b",
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.1,
			"top_p":       0.8,
			"num_ctx":     2048,
		},
	}
	body, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return strings.TrimSpace(result.Response), nil
}

func cleanJSONResponse(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimSuffix(raw, "```")
	} else if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
	}
	return strings.TrimSpace(raw)
}
