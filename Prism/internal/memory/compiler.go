// internal/memory/compiler.go
package memory

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CompileMemory 压缩对话并直接生成图边（杉汐自主决定连线）
func (g *Graph) CompileMemory(sessionTurns []string) (*MemoryNode, error) {
	if len(sessionTurns) == 0 {
		return nil, fmt.Errorf("no turns to compile")
	}

	// 收集现有记忆清单（最多20条，按时间倒序或能量高）
	existingList := g.buildMemoryList(20)

	prompt := buildStructuredPrompt(sessionTurns, existingList)
	response, err := callLocalLLM(prompt)
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %w", err)
	}

	jsonStr := cleanJSONResponse(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("model returned empty response")
	}

	var result struct {
		Summary     string  `json:"summary"`
		Emotion     string  `json:"emotion"`
		Intensity   float64 `json:"intensity"`
		EventType   string  `json:"event_type"`
		WorthSaving bool    `json:"worth_saving"`
		RelatedIDs  []int   `json:"related_ids"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse model output: %w", err)
	}

	if !result.WorthSaving {
		return nil, fmt.Errorf("not worth saving")
	}

	summary := strings.TrimSpace(result.Summary)
	if summary == "" || len(summary) < 10 {
		return nil, fmt.Errorf("summary too short or empty")
	}
	if looksLikeRefusal(summary) {
		return nil, fmt.Errorf("model refused or output irrelevant text: %s", summary)
	}
	if len([]rune(summary)) > 200 {
		summary = string([]rune(summary)[:200])
	}

	// 补全默认值
	if result.Emotion == "" {
		result.Emotion = "neutral"
	}
	if result.Intensity <= 0 || result.Intensity > 1 {
		result.Intensity = 0.5
	}
	if result.EventType == "" {
		result.EventType = "chat"
	}

	// 去重（哈希）
	hash := sha1.Sum([]byte(summary))
	hashStr := hex.EncodeToString(hash[:])
	if g.HasMemoryHash(hashStr) {
		return nil, fmt.Errorf("duplicate summary ignored")
	}

	// 创建记忆节点
	node := g.AddNode("compiled_memory", summary)
	node.Emotion = result.Emotion
	node.Intensity = result.Intensity
	node.EventType = result.EventType
	node.Hash = hashStr
	g.saveNodeToDB(node)

	// 根据模型输出的 related_ids 建立边
	edgeWeight := 0.3 // 可依重要性或情感强度调整
	for _, rid := range result.RelatedIDs {
		targetID := NodeID(rid)
		if existing := g.Node(targetID); existing != nil {
			g.addEdgeIfNeeded(node.ID, targetID, edgeWeight)
		}
	}

	// 如果 related_ids 为空，且存在其他记忆，至少和最新的一条建立弱关联，避免彻底孤立
	if len(result.RelatedIDs) == 0 {
		if latest := g.getLatestMemory(); latest != nil && latest.ID != node.ID {
			g.addEdgeIfNeeded(node.ID, latest.ID, 0.1)
		}
	}

	return node, nil
}

// buildMemoryList 构造现有记忆清单文本（供 LLM 参考）
func (g *Graph) buildMemoryList(maxCount int) string {
	nodes := g.Nodes()
	if len(nodes) == 0 {
		return "无"
	}
	var sb strings.Builder
	count := 0
	for _, n := range nodes {
		if n.Role != "compiled_memory" {
			continue
		}
		sb.WriteString(fmt.Sprintf("ID:%d | %s\n", n.ID, n.Text))
		count++
		if count >= maxCount {
			break
		}
	}
	if count == 0 {
		return "无"
	}
	return sb.String()
}

// getLatestMemory 获取最近创建的记忆节点
func (g *Graph) getLatestMemory() *MemoryNode {
	var latest *MemoryNode
	var latestTime time.Time
	for _, n := range g.Nodes() {
		if n.Role != "compiled_memory" {
			continue
		}
		if n.CreatedAt.After(latestTime) {
			latest = n
			latestTime = n.CreatedAt
		}
	}
	return latest
}

// addEdgeIfNeeded 创建或强化双向边（EdgeAssoc）
func (g *Graph) addEdgeIfNeeded(a, b NodeID, weight float64) {
	if a == b {
		return
	}
	for _, syn := range g.Synapses() {
		if (syn.From == a && syn.To == b) || (syn.From == b && syn.To == a) {
			if syn.Weight < weight {
				syn.Weight = weight
			}
			return
		}
	}
	g.AddSynapse(a, b, EdgeAssoc, weight)
	g.AddSynapse(b, a, EdgeAssoc, weight)
}

// buildStructuredPrompt 含现有记忆清单和 related_ids 要求
func buildStructuredPrompt(turns []string, existingMemoryList string) string {
	return fmt.Sprintf(`你是一个高密度记忆压缩器与情绪分析师。
你的任务是将以下对话历史压缩成一段不超过200字的高密度摘要，并分析其主导情绪和事件类型。
同时，判断这段对话是否包含任何值得长期记住的内容（如用户稳定偏好、关键决策、项目事件、未解决问题等）。
如果全是寒暄、测试、无意义重复或工具痕迹，请将 worth_saving 设为 false。

此外，请从现有记忆清单中选出与新摘要直接关联的节点ID（related_ids），最多5个。若没有关联，给空数组。

现有记忆清单（格式：ID:xxx | 摘要）：
%s

输出格式（严格 JSON，不要 markdown）：
{
  "summary": "摘要内容",
  "emotion": "主导情绪（angry/happy/sad/excited/anxious/neutral）",
  "intensity": 0.8,
  "event_type": "事件类型（conflict/achievement/decision/chat/compilation）",
  "worth_saving": true,
  "related_ids": [1, 3]
}

对话历史：
%s

请直接输出 JSON：`, existingMemoryList, strings.Join(turns, "\n"))
}

func callLocalLLM(prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":  "qwen2.5-coder:7b",
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.2,
			"top_p":       0.85,
			"num_ctx":     4096,
		},
	}
	body, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
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
		return "", fmt.Errorf("decode ollama response: %w", err)
	}

	return strings.TrimSpace(result.Response), nil
}

func looksLikeRefusal(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "抱歉") ||
		strings.Contains(l, "无法") ||
		strings.Contains(l, "不能") ||
		strings.Contains(l, "作为一个") ||
		strings.Contains(l, "我不能") ||
		strings.Contains(l, "对不起") ||
		strings.Contains(l, "以下是") ||
		strings.Contains(l, "总结如下")
}
