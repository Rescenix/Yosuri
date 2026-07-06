package aap

import (
	"time"
)

// 事件类型常量
const (
	TypeMemorySync     = "MEMORY_SYNC"
	TypeAgentBroadcast = "AGENT_BROADCAST"
	TypeAgentResult    = "AGENT_RESULT"
	TypeChainHash      = "CHAIN_HASH"
)

// MemoryCoord 表示 PrismD 记忆场的当前世界线坐标
type MemoryCoord struct {
	ActiveNodes int     `json:"active_nodes"`
	Conductance float64 `json:"conductance"`
	Chaos       float64 `json:"chaos"`
	Alpha       float64 `json:"alpha"`
	Lambda      float64 `json:"lambda"`
	DataAge     int64   `json:"data_age"` // 坐标数据的陈旧程度（秒），0 表示实时
}

// AAPEvent 是 AAP 协议的标准通信帧
type AAPEvent struct {
	ID          string       `json:"id"`
	Type        string       `json:"type"`         // MEMORY_SYNC | AGENT_BROADCAST | AGENT_RESULT | CHAIN_HASH
	From        string       `json:"from"`         // 发送者 Agent 名
	To          string       `json:"to,omitempty"` // 目标 Agent 名，空表示广播
	Payload     interface{}  `json:"payload"`      // 具体任务内容
	MemoryCoord *MemoryCoord `json:"memory_coord,omitempty"`
	CausalHash  string       `json:"causal_hash,omitempty"`
	Timestamp   int64        `json:"timestamp"`
}

// NewAAPEvent 创建一个带有时间戳和记忆坐标的 AAP 事件
func NewAAPEvent(eventType, from string, payload interface{}, coord *MemoryCoord) AAPEvent {
	return AAPEvent{
		ID:          generateEventID(),
		Type:        eventType,
		From:        from,
		Payload:     payload,
		MemoryCoord: coord,
		Timestamp:   time.Now().Unix(),
	}
}

// generateEventID 生成一个简单的唯一事件 ID
func generateEventID() string {
	return "evt_" + time.Now().Format("20060102150405.000000")
}
