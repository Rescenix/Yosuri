package handler

import (
	"container/list"
	"sync"
)

// LRUMemory 纯本地 LRU 记忆缓存
type LRUMemory struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*list.Element
	order    *list.List
}

type lruEntry struct {
	key   string
	value string
}

// NewLRUMemory 创建一个新的 LRU 缓存
func NewLRUMemory(capacity int) *LRUMemory {
	return &LRUMemory{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

// Add 添加或更新一条记忆
func (m *LRUMemory) Add(key, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if elem, ok := m.items[key]; ok {
		elem.Value.(*lruEntry).value = value
		m.order.MoveToFront(elem)
		return
	}

	if m.order.Len() >= m.capacity {
		oldest := m.order.Back()
		if oldest != nil {
			m.order.Remove(oldest)
			delete(m.items, oldest.Value.(*lruEntry).key)
		}
	}

	entry := &lruEntry{key: key, value: value}
	elem := m.order.PushFront(entry)
	m.items[key] = elem
}

// Get 获取一条记忆，不存在返回空字符串
func (m *LRUMemory) Get(key string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if elem, ok := m.items[key]; ok {
		m.order.MoveToFront(elem)
		return elem.Value.(*lruEntry).value
	}
	return ""
}

// GetRecent 获取最近 N 条记忆
func (m *LRUMemory) GetRecent(n int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var results []string
	count := 0
	for e := m.order.Front(); e != nil && count < n; e = e.Next() {
		results = append(results, e.Value.(*lruEntry).value)
		count++
	}
	return results
}

// Len 返回当前记忆数量
func (m *LRUMemory) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.order.Len()
}
