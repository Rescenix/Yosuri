// internal/memory/inverted.go
package memory

import (
	"sync"

	"github.com/go-ego/gse"
)

var (
	seg     gse.Segmenter
	segOnce sync.Once
)

// initTokenizer 初始化 gse 分词器（加载默认词典，只执行一次）
func initTokenizer() {
	segOnce.Do(func() {
		seg.LoadDict() // 加载默认中文词典
	})
}

// tokenize 使用 gse 搜索引擎模式分词，适合倒排索引
func tokenize(text string) []string {
	initTokenizer()
	return seg.CutSearch(text, true) // CutSearch 搜索引擎模式，返回索引专用切分
}

// InvertedIndex 倒排索引：token → nodeIDs
type InvertedIndex struct {
	mu  sync.RWMutex
	idx map[string][]NodeID
}

func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{idx: make(map[string][]NodeID)}
}

// Add 将一个节点的文本分词后加入索引
func (inv *InvertedIndex) Add(id NodeID, text string) {
	tokens := tokenize(text)
	inv.mu.Lock()
	defer inv.mu.Unlock()
	for _, t := range tokens {
		inv.idx[t] = appendUniq(inv.idx[t], id)
	}
}

// Remove 删除某个节点的所有 token
func (inv *InvertedIndex) Remove(id NodeID, text string) {
	tokens := tokenize(text)
	inv.mu.Lock()
	defer inv.mu.Unlock()
	for _, t := range tokens {
		if list, ok := inv.idx[t]; ok {
			inv.idx[t] = removeID(list, id)
		}
	}
}

// Query 返回包含任意一个查询 token 的节点 ID 集合
func (inv *InvertedIndex) Query(query string) map[NodeID]bool {
	tokens := tokenize(query)
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	result := make(map[NodeID]bool)
	for _, t := range tokens {
		for _, id := range inv.idx[t] {
			result[id] = true
		}
	}
	return result
}

func appendUniq(list []NodeID, id NodeID) []NodeID {
	for _, v := range list {
		if v == id {
			return list
		}
	}
	return append(list, id)
}

func removeID(list []NodeID, id NodeID) []NodeID {
	result := make([]NodeID, 0, len(list))
	for _, v := range list {
		if v != id {
			result = append(result, v)
		}
	}
	return result
}
