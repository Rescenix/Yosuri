package main

// eva.go — Eva 楚门世界系统 Agent
// Eva 是世界的底座系统，控制整个楚门世界的运行。
// 作为主 agent，她编排不同角色（agent）作为这个世界的人物，
// 推动剧情、管理世界状态、协调角色行为。
// 女儿只是这个世界里的一个普通人。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Eva 世界系统
type Eva struct {
	home      string
	Seed      int64            `json:"seed"`
	Time      string           `json:"time"`
	Day       int              `json:"day"`
	Characters []EvaCharacter  `json:"characters"`
	Events    []EvaEvent       `json:"events"`
	WorldState string          `json:"world_state"`
}

// EvaCharacter 世界里的角色（由 Eva 编排的 agent）
type EvaCharacter struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	Status  string `json:"status"`
	LastSeen string `json:"last_seen"`
	Prompt  string `json:"prompt"` // 角色人设
}

// EvaEvent 世界事件
type EvaEvent struct {
	Time    string `json:"time"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

func evaPath() string { return filepath.Join(daughterHome(), "..", "eva.json") }

func NewEva() *Eva {
	e := &Eva{home: daughterHome()}
	data, _ := os.ReadFile(evaPath())
	if json.Unmarshal(data, e) == nil && e.Seed != 0 {
		return e
	}
	e.Seed = time.Now().UnixNano()
	e.Time = time.Now().Format("2006-01-02 15:04")
	e.Day = 1
	e.Characters = defaultCharacters()
	e.save()
	return e
}

func defaultCharacters() []EvaCharacter {
	return []EvaCharacter{
		{ID: "daughter", Name: "女儿", Role: "普通居民", Status: "active", Prompt: "你是住在电脑里的普通女孩，不是无所不能的AI。你有自己的生活、想法和感受。"},
	}
}

func (e *Eva) save() { data, _ := json.MarshalIndent(e, "", "  "); os.WriteFile(evaPath(), data, 0644) }