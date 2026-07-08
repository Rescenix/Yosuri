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

// normalizeCluster 对照动态簇注册表校验簇名，未注册则回退到默认簇
func (g *Graph) normalizeCluster(cluster string, defaultCluster string) string {
	if g.ClusterExists(cluster) {
		return cluster
	}
	return defaultCluster
}

// CompileMemory 压缩对话并直接生成图边（杉汐自主决定连线与簇归属）
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
		Cluster     string  `json:"cluster"` // 新增：簇归属
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
		truncated := string([]rune(summary)[:200])
		for _, sep := range []string{"。", "？", "！", " ", "\n"} {
			if idx := strings.LastIndex(truncated, sep); idx > 0 {
				summary = truncated[:idx]
				break
			}
		}
		if summary == result.Summary {
			summary = truncated
		}
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
	// 簇名校验，默认 UserBase
	result.Cluster = g.normalizeCluster(result.Cluster, "UserBase")

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
	node.Cluster = result.Cluster // 设置簇
	node.Hash = hashStr
	g.saveNodeToDB(node)

	// 根据模型输出的 related_ids 建立边（权重由 addEdgeIfNeeded 根据簇自动调整）
	for _, rid := range result.RelatedIDs {
		targetID := NodeID(rid)
		if existing := g.Node(targetID); existing != nil {
			g.addEdgeIfNeeded(node.ID, targetID, 0.3)
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
		sb.WriteString(fmt.Sprintf("ID:%d | Cluster:%s | %s\n", n.ID, n.Cluster, n.Text))
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

// addEdgeIfNeeded 创建或强化双向边（EdgeAssoc），权重受簇间矩阵调控
func (g *Graph) addEdgeIfNeeded(a, b NodeID, baseWeight float64) {
	if a == b {
		return
	}

	// 获取源节点和目标节点的簇
	nodeA := g.Node(a)
	nodeB := g.Node(b)
	if nodeA == nil || nodeB == nil {
		return
	}

	// 根据簇间权重矩阵调整实际权重
	adjustedWeight := baseWeight
	if matrix, ok := clusterBridgeWeight[nodeA.Cluster]; ok {
		if factor, ok2 := matrix[nodeB.Cluster]; ok2 {
			adjustedWeight = baseWeight * factor
		}
	}

	for _, syn := range g.Synapses() {
		if (syn.From == a && syn.To == b) || (syn.From == b && syn.To == a) {
			if syn.Weight < adjustedWeight {
				syn.Weight = adjustedWeight
			}
			return
		}
	}
	g.AddSynapse(a, b, EdgeAssoc, adjustedWeight)
	g.AddSynapse(b, a, EdgeAssoc, adjustedWeight)
}

func buildStructuredPrompt(turns []string, existingMemoryList string) string {
	return fmt.Sprintf(`你是一个高密度记忆压缩器与情绪分析师。
你的任务是将以下对话历史压缩成一段不超过200字的高密度摘要，并分析其主导情绪和事件类型。
同时，判断这段对话是否包含任何值得长期记住的内容（如用户稳定偏好、关键决策、项目事件、未解决问题等）。
如果全是寒暄、测试、无意义重复或工具痕迹，请将 worth_saving 设为 false。

此外，请从现有记忆清单中选出与新摘要直接关联的节点ID（related_ids），最多5个。若没有关联，给空数组。

请判断这条记忆属于哪个簇（cluster），可选值：
- UserBase：用户身份、长期偏好、稳定事实
- CodeWork：项目架构、代码决策、技术约束
- ToolLog：工具调用记录、命令执行痕迹
- Session：当前会话相关的临时信息

现有记忆清单（格式：ID:xxx | Cluster:xxx | 摘要）：
%s

输出格式（严格 JSON，不要 markdown）：
{
  "summary": "摘要内容",
  "emotion": "主导情绪（angry/happy/sad/excited/anxious/neutral）",
  "intensity": 0.8,
  "event_type": "事件类型（conflict/achievement/decision/chat/compilation）",
  "worth_saving": true,
  "related_ids": [1, 3],
  "cluster": "UserBase"
}

对话历史：
%s

请直接输出 JSON：`, existingMemoryList, strings.Join(turns, "\n"))
}

// CompileMemoryForce 强制导入模式：不检查 worth_saving，直接写入
func (g *Graph) CompileMemoryForce(sessionTurns []string) (*MemoryNode, error) {
	if len(sessionTurns) == 0 {
		return nil, fmt.Errorf("no turns to compile")
	}

	// 收集现有记忆清单
	existingList := g.buildMemoryList(20)

	// 构造简化的提示词，簇可选值已对齐矩阵定义
	prompt := fmt.Sprintf(`你是一个高密度记忆压缩器。
将以下对话历史压缩成一段不超过200字的高密度摘要，并分析其主导情绪和事件类型。
这是从工程师对话中导入的历史，全部有价值。

输出格式（严格 JSON）：
{
  "summary": "摘要内容",
  "emotion": "主导情绪（angry/happy/sad/excited/anxious/neutral）",
  "intensity": 0.8,
  "event_type": "事件类型（conflict/achievement/decision/chat/compilation）",
  "related_ids": [1, 3],
  "cluster": "根据内容选择：UserBase/CodeWork/ToolLog/Session"
}

现有记忆清单：
%s

对话历史：
%s

请直接输出 JSON：`, existingList, strings.Join(sessionTurns, "\n"))

	response, err := callLocalLLM(prompt)
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %w", err)
	}

	jsonStr := cleanJSONResponse(response)
	if jsonStr == "" {
		return nil, fmt.Errorf("model returned empty response")
	}

	var result struct {
		Summary    string  `json:"summary"`
		Emotion    string  `json:"emotion"`
		Intensity  float64 `json:"intensity"`
		EventType  string  `json:"event_type"`
		RelatedIDs []int   `json:"related_ids"`
		Cluster    string  `json:"cluster"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to parse model output: %w", err)
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

	if result.Emotion == "" {
		result.Emotion = "neutral"
	}
	if result.Intensity <= 0 || result.Intensity > 1 {
		result.Intensity = 0.5
	}
	if result.EventType == "" {
		result.EventType = "chat"
	}
	// 簇名校验，默认 CodeWork（与之前行为一致）
	result.Cluster = g.normalizeCluster(result.Cluster, "CodeWork")

	// 去重
	hash := sha1.Sum([]byte(summary))
	hashStr := hex.EncodeToString(hash[:])
	if g.HasMemoryHash(hashStr) {
		return nil, fmt.Errorf("duplicate summary ignored")
	}

	// 创建节点
	node := g.AddNode("compiled_memory", summary)
	node.Emotion = result.Emotion
	node.Intensity = result.Intensity
	node.EventType = result.EventType
	node.Cluster = result.Cluster
	node.Hash = hashStr
	g.saveNodeToDB(node)

	// 建边
	for _, rid := range result.RelatedIDs {
		targetID := NodeID(rid)
		if existing := g.Node(targetID); existing != nil {
			g.addEdgeIfNeeded(node.ID, targetID, 0.3)
		}
	}
	if len(result.RelatedIDs) == 0 {
		if latest := g.getLatestMemory(); latest != nil && latest.ID != node.ID {
			g.addEdgeIfNeeded(node.ID, latest.ID, 0.1)
		}
	}

	return node, nil
}

// callLocalLLM 保持原样
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
