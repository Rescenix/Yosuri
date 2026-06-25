// internal/memory/gate.go
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

// ShouldRetrieveMemory 判断是否需要检索长期记忆
// 返回 true 表示需要检索，false 表示不需要
func ShouldRetrieveMemory(history []string, currentMsg string) bool {
	// 第一道门：规则兜底，处理最明显的触发词
	if fastRuleCheck(history, currentMsg) {
		return true
	}

	// 第二道门：本地模型二分类
	result, err := callGateLLM(history, currentMsg)
	if err == nil && result != nil {
		return result.NeedMemory
	}

	// 模型调用失败，保守回退：不检索（避免无谓的调用浪费性能）
	return false
}

type GateResult struct {
	NeedMemory bool    `json:"need_memory"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

// fastRuleCheck 快速规则检查，捕获最明显的回忆意图
func fastRuleCheck(history []string, currentMsg string) bool {
	strongTriggers := []string{
		"还记得吗", "上次我们聊", "那个时候", "你之前说",
		"接着刚才", "回到之前", "继续那个话题", "刚才说到哪",
		"我之前跟你提过", "你记不记得", "回忆一下",
	}
	for _, t := range strongTriggers {
		if strings.Contains(currentMsg, t) {
			return true
		}
	}
	return false
}

// callGateLLM 调用本地模型做二分类判断
func callGateLLM(history []string, currentMsg string) (*GateResult, error) {
	context := strings.Join(history, "\n")
	prompt := fmt.Sprintf(`你是一个对话触发器。请根据对话历史和当前消息，判断用户是否需要我“回忆”过去的对话。

仅当用户在追问、回忆、或消息中明显依赖上下文时才需要。日常闲聊、简单问候、单步指令不需要。

输出JSON，不要任何解释：
{"need_memory": true/false, "reason": "简短理由", "confidence": 0.9}

对话历史：
%s

当前消息：%s

请输出JSON：`, context, currentMsg)

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

	client := &http.Client{Timeout: 2 * time.Second} // 严格控制超时，避免拖慢对话
	resp, err := client.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	jsonStr := cleanJSONResponse(result.Response)
	if jsonStr == "" {
		return nil, fmt.Errorf("empty response")
	}

	var gateResult GateResult
	if err := json.Unmarshal([]byte(jsonStr), &gateResult); err != nil {
		return nil, err
	}
	return &gateResult, nil
}
