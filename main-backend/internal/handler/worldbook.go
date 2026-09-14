package handler

// worldbook.go —— 世界书（Lorebook / 设定集）。
//
// 一张世界书 = 一组「条目」。每条有触发关键词和一段设定正文：
// 用户/角色的话里命中关键词，这条设定就在合适的深度注入上下文，
// 模型于是「知道」这个世界里王都的税率、剑的名称、某段旧事。
//
// 分两种作用域：
//   - 全局 ~/rescene_data/worldbook.json —— 世界观、地点、规则，跨角色共用
//   - 角色 ~/rescene_data/agents/<id>/worldbook.json —— 只属于这张卡私有的设定
//
// 读取时全局在前、角色在后合并，同 uid 的条目以角色卡版本为准（角色可覆盖世界设定）。
// 常驻条目（constant=true）无条件注入；其余按 bigram 重合度扫描最近对话触发。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backend/internal/memorydir"
)

// LoreEntry 一条世界书条目。
type LoreEntry struct {
	UID       int      `json:"uid"`
	Name      string   `json:"name"`       // 条目标题（面板显示用）
	Keys      []string `json:"keys"`       // 触发词，命中任一即激活；空=常驻
	Content   string   `json:"content"`    // 设定正文
	Constant  bool     `json:"constant"`   // 常驻：无条件注入（世界观底色）
	Selective bool     `json:"selective"`  // true 时需 primary 与 secondary 同时命中
	Keys2     []string `json:"keys2"`      // 次级触发词（配合 Selective）
	CaseMatch bool     `json:"caseMatch"`  // 区分大小写
	Depth     int      `json:"depth"`      // 注入深度：0=最后一条消息之前，越大越早
	Order     int      `json:"order"`      // 同深度内的排序，小者在前
	Position  string   `json:"position"`   // "before"（默认，进系统提示）| "after"（进对话流）
	Enabled   bool     `json:"enabled"`    // 关掉但不删
	Token     int      `json:"token"`      // 估算 token，预算裁剪用（0=现算）
}

// WorldBook 一本世界书。
type WorldBook struct {
	Name    string      `json:"name"`
	Entries []LoreEntry `json:"entries"`
}

// loreMu 串行化读改写。世界书面板编辑是低频操作，粗粒度锁足够。
var loreMu sync.Mutex

// loreCache 2s 内存缓存：每轮工作流都要读世界书，不能每轮都解一遍 JSON。
var loreCache = struct {
	sync.Mutex
	byPath map[string]cacheEntry
}{byPath: map[string]cacheEntry{}}

type cacheEntry struct {
	lb     *WorldBook
	when   time.Time
}

const loreCacheTTL = 2 * time.Second

func loreFilePath() string { return filepath.Join(resceneUserDataDir(), "worldbook.json") }

func agentLoreFilePath(agentID string) string {
	d := memorydir.AgentDir(agentID)
	if d == "" {
		return ""
	}
	return filepath.Join(d, "worldbook.json")
}

// readLoreFile 读单本世界书；文件不存在返回空书（不是错误——新用户本来就没有）。
func readLoreFile(path string) *WorldBook {
	if path == "" {
		return &WorldBook{}
	}
	loreCache.Lock()
	if c, ok := loreCache.byPath[path]; ok && time.Since(c.when) < loreCacheTTL {
		loreCache.Lock()
		defer loreCache.Unlock()
		return c.lb
	}
	loreCache.Unlock()

	lb := &WorldBook{}
	if data, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		_ = json.Unmarshal(data, lb)
	}
	loreCache.Lock()
	loreCache.byPath[path] = cacheEntry{lb: lb, when: time.Now()}
	loreCache.Unlock()
	return lb
}

func writeLoreFile(path string, lb *WorldBook) error {
	if path == "" {
		return fmt.Errorf("世界书路径非法")
	}
	data, err := json.MarshalIndent(lb, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	loreCache.Lock()
	loreCache.byPath[path] = cacheEntry{lb: lb, when: time.Now()}
	loreCache.Unlock()
	return nil
}

// loadWorldBook 合并全局 + 该 Agent 私有设定，返回可直接注入的一本。
// agentID 为空时只有全局部分。
func loadWorldBook(agentID string) *WorldBook {
	merged := &WorldBook{Name: "worldbook"}
	seen := map[string]int{} // 标题 -> 在 merged 里的下标，用于角色覆盖全局
	add := func(lb *WorldBook) {
		for _, e := range lb.Entries {
			if !e.Enabled {
				continue
			}
			key := strings.TrimSpace(e.Name)
			if key != "" {
				if i, ok := seen[key]; ok {
					merged.Entries[i] = e
					continue
				}
			}
			if key != "" {
				seen[key] = len(merged.Entries)
			}
			merged.Entries = append(merged.Entries, e)
		}
	}
	add(readLoreFile(loreFilePath()))
	if agentID != "" {
		add(readLoreFile(agentLoreFilePath(agentID)))
	}
	return merged
}
