package memory

import "time"

// MemoryAtom 记忆原子：一条经过提炼的记忆
type MemoryAtom struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Keywords  []string  `json:"keywords"`
	Summary   string    `json:"summary"`
	Original  string    `json:"original"`
	Role      string    `json:"role"`
}

// IndexEntry 索引条目：关键词 → 记忆ID列表
type IndexEntry struct {
	Keyword   string    `json:"keyword"`
	MemoryIDs []string  `json:"memory_ids"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MemoryIndex 网状索引：多个 IndexEntry 的集合
type MemoryIndex struct {
	Entries    map[string]*IndexEntry `json:"entries"`
	LastUpdate time.Time              `json:"last_update"`
}
