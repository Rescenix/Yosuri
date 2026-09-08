package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// GraphNode 知识图谱中的一个实体节点。
type GraphNode struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	File  string `json:"file"`
	Count int    `json:"count"`
}

// GraphLink 两个实体之间的关系。
type GraphLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Label  string `json:"label"`
}

// Graph 知识图谱。
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Links []GraphLink `json:"links"`
}

// ── 缓存 ──

var (
	graphMu    sync.Mutex
	graphCache = map[string]cachedGraph{}
)

type cachedGraph struct {
	modTime int64
	graph   *Graph
}

// getCachedGraph 取缓存的图谱（mtime 一致才命中）。
func getCachedGraph(path string) *Graph {
	graphMu.Lock()
	defer graphMu.Unlock()
	if c, ok := graphCache[path]; ok {
		if fi, err := os.Stat(path); err == nil && fi.ModTime().Unix() == c.modTime {
			return c.graph
		}
	}
	return nil
}

// setCacheGraph 写入缓存。
func setCacheGraph(path string, g *Graph) {
	graphMu.Lock()
	defer graphMu.Unlock()
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	graphCache[path] = cachedGraph{modTime: fi.ModTime().Unix(), graph: g}
}

// InvalidateGraph 清掉单个文件的图谱缓存。
func InvalidateGraph(path string) {
	graphMu.Lock()
	defer graphMu.Unlock()
	delete(graphCache, path)
}

// InvalidateAllGraphs 清空全部图谱缓存。
func InvalidateAllGraphs() {
	graphMu.Lock()
	defer graphMu.Unlock()
	graphCache = map[string]cachedGraph{}
}

// ── 解析 ──

// ParseGraph 从 LLM 返回的 JSON 文本中解析知识图谱。
// 兼容两种格式：直接 {nodes, links} 或 ```json ... ``` 包裹。
func ParseGraph(raw string) (*Graph, error) {
	raw = strings.TrimSpace(raw)
	raw = stripCodeFence(raw)

	var g Graph
	if err := json.Unmarshal([]byte(raw), &g); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	// 去重 + 清理
	g = dedupeGraph(g)
	return &g, nil
}

// stripCodeFence 去掉 ```json ... ``` 包裹。
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s[3:], "```"); idx >= 0 {
			s = s[3 : 3+idx]
		}
	}
	return strings.TrimSpace(s)
}

// dedupeGraph 去重节点和连线。
func dedupeGraph(g Graph) Graph {
	// 节点去重（同 id 合并 count）
	seenNode := map[string]int{}
	for _, n := range g.Nodes {
		n.ID = strings.TrimSpace(n.ID)
		n.Name = strings.TrimSpace(n.Name)
		n.Type = strings.TrimSpace(n.Type)
		if n.ID == "" {
			continue
		}
		if idx, ok := seenNode[n.ID]; ok {
			g.Nodes[idx].Count += n.Count
			continue
		}
		seenNode[n.ID] = len(g.Nodes)
		g.Nodes = append(g.Nodes, n)
	}
	// 连线去重（同 source+target+label 只保留一条）
	seenLink := map[string]bool{}
	var links []GraphLink
	for _, l := range g.Links {
		l.Source = strings.TrimSpace(l.Source)
		l.Target = strings.TrimSpace(l.Target)
		l.Label = strings.TrimSpace(l.Label)
		if l.Source == "" || l.Target == "" || l.Source == l.Target {
			continue
		}
		key := l.Source + "|" + l.Target + "|" + l.Label
		if seenLink[key] {
			continue
		}
		seenLink[key] = true
		links = append(links, l)
	}
	g.Links = links
	return g
}

// ── 采样 ──

// SampleText 从长文本中采样 ~maxChars 字符送给 LLM。
// 策略：取前 60% + 后 40%（保留开头定义和结尾总结）。
func SampleText(text string, maxChars int) string {
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}
	head := int(float64(maxChars) * 0.6)
	tail := maxChars - head
	return string(runes[:head]) + "\n\n...[中间省略]...\n\n" + string(runes[len(runes)-tail:])
}

// GraphForFile 构建单个文件的知识图谱（带缓存）。
// llmCall: 传入文本，返回 LLM 生成的 JSON 字符串。
func GraphForFile(path string, llmCall func(text string) string) (*Graph, error) {
	if g := getCachedGraph(path); g != nil {
		return g, nil
	}
	text, err := extractText(path)
	if err != nil {
		return nil, err
	}
	// 限制长度：LLM 上下文窗口和成本考虑
	sampled := SampleText(text, 8000)
	raw := llmCall(sampled)
	g, err := ParseGraph(raw)
	if err != nil {
		return nil, err
	}
	// 补 file 字段
	for i := range g.Nodes {
		g.Nodes[i].File = filepath.Base(path)
	}
	setCacheGraph(path, g)
	return g, nil
}

// GraphForAll 构建全部文件的知识图谱（合并）。
func GraphForAll(llmCall func(text string) string) (*Graph, error) {
	files := walkFiles()
	var allNodes []GraphNode
	var allLinks []GraphLink
	for _, f := range files {
		g, err := GraphForFile(f, llmCall)
		if err != nil {
			continue
		}
		allNodes = append(allNodes, g.Nodes...)
		allLinks = append(allLinks, g.Links...)
	}
	g := dedupeGraph(Graph{Nodes: allNodes, Links: allLinks})
	return &g, nil
}

// SortNodesByCount 按出现次数降序排节点。
func SortNodesByCount(g *Graph) {
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].Count > g.Nodes[j].Count })
}
