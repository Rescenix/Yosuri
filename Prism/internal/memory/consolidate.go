// internal/memory/consolidate.go
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

// ConsolidateResult 杉汐的判断结果
type ConsolidateResult struct {
	Actions []ConsolidateAction `json:"actions"`
}

type ConsolidateAction struct {
	Action   string `json:"action"`    // "merge" | "discard"
	SourceID uint64 `json:"source_id"` // 被合并或被丢弃的节点
	TargetID uint64 `json:"target_id"` // 合并目标（仅merge时有效）
	Reason   string `json:"reason"`    // 杉汐的理由
}

// ConsolidateMemory 让杉汐整理整个记忆场，自动合并重复，丢弃无用记忆
func (g *Graph) ConsolidateMemory() error {
	// 1. 收集所有 compiled_memory 节点
	var nodes []*MemoryNode
	for _, n := range g.Nodes() {
		if n.Role == "compiled_memory" && n.BaseEnergy > 0 {
			nodes = append(nodes, n)
		}
	}
	if len(nodes) < 2 {
		return nil // 没什么好整理的
	}

	// 2. 构造提示词：把记忆清单和规则一起给杉汐
	prompt := buildConsolidatePrompt(nodes)

	// 3. 调用本地模型
	response, err := callLocalLLM(prompt)
	if err != nil {
		return fmt.Errorf("杉汐整理失败: %w", err)
	}

	// 4. 解析杉汐的判断
	jsonStr := cleanJSONResponse(response)
	var result ConsolidateResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return fmt.Errorf("解析杉汐输出失败: %w", err)
	}

	// 5. 执行杉汐的决定
	for _, action := range result.Actions {
		switch action.Action {
		case "merge":
			source := g.Node(NodeID(action.SourceID))
			target := g.Node(NodeID(action.TargetID))
			if source != nil && target != nil && source.ID != target.ID {
				// 合并：保留 target 节点，吸收 source 节点
				// 1. 让杉汐重写摘要，而不是简单拼接
				newSummary, err := rewriteSummary(target.Text, source.Text)
				if err == nil && len(newSummary) > 10 {
					target.Text = newSummary
				} else {
					// 降级：只保留主节点文本，不拼接
				}
				target.LastAccessAt = time.Now()
				target.BaseEnergy = min(0.99, target.BaseEnergy+source.BaseEnergy*0.5)

				// 2. 迁移 source 的所有边到 target（防止成为孤岛）
				for _, synID := range source.OutEdges {
					if s := g.Synapse(synID); s != nil {
						if s.From == source.ID {
							g.addEdgeIfNeeded(target.ID, s.To, s.Weight)
						} else if s.To == source.ID {
							g.addEdgeIfNeeded(s.From, target.ID, s.Weight)
						}
					}
				}

				// 3. 软删除 source 节点
				source.BaseEnergy = 0
				g.saveNodeToDB(target)
				g.saveNodeToDB(source)
			}
		case "discard":
			if n := g.Node(NodeID(action.SourceID)); n != nil {
				n.BaseEnergy = 0 // 软删除
				g.saveNodeToDB(n)
			}
		}
	}

	return nil
}

// buildConsolidatePrompt 构造整理提示词
func buildConsolidatePrompt(nodes []*MemoryNode) string {
	var sb strings.Builder
	sb.WriteString("你是记忆整理师。以下是一个数字记忆场的所有记忆节点。\n\n")
	sb.WriteString("请找出：\n")
	sb.WriteString("1. 描述同一件事或同一偏好的重复记忆 → action: \"merge\"\n")
	sb.WriteString("2. 无意义的、过时的、矛盾的记忆 → action: \"discard\"\n")
	sb.WriteString("3. 有价值的记忆 → 不输出（保持原样）\n\n")
	sb.WriteString("规则：\n")
	sb.WriteString("- merge时必须指定target_id（保留哪个节点，把source_id合并进去）\n")
	sb.WriteString("- 宁可保留，不要误删。不确定的不要动。\n")
	sb.WriteString("- 每条action都要给reason\n\n")
	sb.WriteString("记忆清单：\n")
	for _, n := range nodes {
		sb.WriteString(fmt.Sprintf("ID:%d | Cluster:%s | %s | Energy:%.2f | %s\n",
			n.ID, n.Cluster, n.Text, n.BaseEnergy, n.Emotion))
	}
	sb.WriteString("\n输出JSON（不要markdown）：\n")
	sb.WriteString(`{"actions": [{"action": "merge|discard", "source_id": 1, "target_id": 2, "reason": "..."}]}`)
	return sb.String()
}

// rewriteSummary 让杉汐把两条旧摘要融合成一条新高密度摘要
func rewriteSummary(textA, textB string) (string, error) {
	prompt := fmt.Sprintf(`请将以下两条关于同一件事的记忆摘要，融合成一条更高密度的摘要（不超过150字），保留所有关键信息，去除冗余。

摘要A：%s

摘要B：%s

直接输出摘要，不要解释：`, textA, textB)

	reqBody := map[string]interface{}{
		"model":  "qwen2.5-coder:7b",
		"prompt": prompt,
		"stream": false,
		"options": map[string]interface{}{
			"temperature": 0.1,
			"num_predict": 200,
		},
	}
	body, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Post("http://localhost:11434/api/generate", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var result struct{ Response string }
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", err
	}
	summary := strings.TrimSpace(result.Response)
	if len([]rune(summary)) > 200 {
		summary = string([]rune(summary)[:200])
	}
	return summary, nil
}

// min 辅助函数（如果 graph.go 里已存在，这里可省略）
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
