// internal/memory/retriever.go
package memory

import (
	"sort"
	"strings"
	"time"
)

// keywordMatch 计算两个字符串的关键词匹配度
func keywordMatch(query, text string) float64 {
	if query == "" || text == "" {
		return 0.0
	}
	qLower := strings.ToLower(query)
	tLower := strings.ToLower(text)
	words := strings.Fields(qLower)
	hits := 0
	for _, w := range words {
		if strings.Contains(tLower, w) {
			hits++
		}
	}
	return min(1.0, float64(hits)/10.0)
}

// recencyScore 根据上次访问时间计算时效分数
func recencyScore(lastAccess time.Time) float64 {
	elapsed := time.Since(lastAccess).Hours()
	if elapsed <= 0 {
		return 1.0
	}
	return 0.5 / (1.0 + elapsed/24.0)
}

// candidateRecall 多信号候选召回
func (g *Graph) candidateRecall(query string) []*MemoryNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var candidates []*MemoryNode
	for _, node := range g.nodes {
		if keywordMatch(query, node.Text) > 0.3 {
			candidates = append(candidates, node)
			continue
		}
		if node.Intensity > 0.8 {
			candidates = append(candidates, node)
			continue
		}
		if strings.Contains(query, node.Emotion) {
			candidates = append(candidates, node)
		}
	}
	return candidates
}

// rankCandidates 计算相关性分数
func (g *Graph) rankCandidates(candidates []*MemoryNode, query string) []*MemoryNode {
	type scoredNode struct {
		node  *MemoryNode
		score float64
	}
	var list []scoredNode
	for _, node := range candidates {
		score := keywordMatch(query, node.Text)*0.4 +
			float64(node.Intensity)*0.3 +
			recencyScore(node.LastAccessAt)*0.3
		list = append(list, scoredNode{node, score})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].score > list[j].score })

	result := make([]*MemoryNode, len(list))
	for i, s := range list {
		result[i] = s.node
	}
	return result
}
