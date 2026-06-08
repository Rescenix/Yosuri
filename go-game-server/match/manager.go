package match

import (
	"encoding/json"

	"log"
	"sync"

	"github.com/google/uuid"
)

// Manager 管理所有客户端连接和匹配队列
type Manager struct {
	mu         sync.Mutex
	Clients    map[string]*Client // 所有连接
	Queue      []*Client          // 等待匹配的队列
	Register   chan *Client
	Unregister chan *Client
	Rooms      map[string]*Room
}

// Message 客户端发送的通用 JSON 消息
type Message struct {
	Type     string          `json:"type"`
	PlayerID string          `json:"playerId,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

// MatchSuccess 匹配成功响应
type MatchSuccess struct {
	Type       string `json:"type"`
	RoomID     string `json:"roomId"`
	OpponentID string `json:"opponentId"`
	YourID     string `json:"yourId"`
}

// NewManager 创建管理器并启动事件循环
func NewManager() *Manager {
	m := &Manager{
		Clients:    make(map[string]*Client),
		Rooms:      make(map[string]*Room),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
	go m.Run()
	return m
}

// Run 事件循环
func (m *Manager) Run() {
	for {
		select {
		case client := <-m.Register:
			m.mu.Lock()
			m.Clients[client.ID] = client
			m.mu.Unlock()
			log.Printf("Client registered: %s", client.ID)

		case client := <-m.Unregister:
			m.mu.Lock()
			// 移除客户端
			if _, ok := m.Clients[client.ID]; ok {
				delete(m.Clients, client.ID)
				// 从匹配队列移除
				for i, c := range m.Queue {
					if c.ID == client.ID {
						m.Queue = append(m.Queue[:i], m.Queue[i+1:]...)
						break
					}
				}
				// 如果该客户端在房间中，通知对手离开
				if client.Room != nil {
					client.Room.Broadcast(client, []byte(`{"type":"opponent_left"}`))
					delete(m.Rooms, client.Room.ID)
					// 移除对手的房间引用
					for _, c := range client.Room.Clients {
						if c.ID != client.ID {
							c.Room = nil
							c.Opponent = nil
						}
					}
				}
			}
			m.mu.Unlock()
			log.Printf("Client unregistered: %s", client.ID)
		}
	}
}

// HandleMessage 处理客户端消息
func (m *Manager) HandleMessage(client *Client, message []byte) {
	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Invalid message from %s: %v", client.ID, err)
		return
	}

	switch msg.Type {
	case "match_request":
		m.handleMatchRequest(client)
	case "battle_action":
		// 转发给对手
		m.mu.Lock()
		if client.Room != nil {
			client.Room.Broadcast(client, message)
		}
		m.mu.Unlock()
	default:
		client.Send <- []byte(`{"error":"unknown type"}`)
	}
}

// handleMatchRequest 将客户端加入匹配队列，尝试配对
func (m *Manager) handleMatchRequest(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 已在队列中或已经在房间中，忽略
	if client.Room != nil {
		client.Send <- []byte(`{"error":"already in match"}`)
		return
	}
	for _, c := range m.Queue {
		if c.ID == client.ID {
			client.Send <- []byte(`{"type":"queue_status","msg":"already in queue"}`)
			return
		}
	}

	// 加入队列
	m.Queue = append(m.Queue, client)
	client.Send <- []byte(`{"type":"queue_status","msg":"looking for opponent..."}`)

	// 检查是否凑齐两人
	if len(m.Queue) >= 2 {
		c1 := m.Queue[0]
		c2 := m.Queue[1]
		m.Queue = m.Queue[2:]

		roomID := uuid.New().String()
		room := NewRoom(roomID, c1, c2)
		m.Rooms[roomID] = room

		c1.Room = room
		c1.Opponent = c2
		c2.Room = room
		c2.Opponent = c1

		// 发送匹配成功消息
		msg1, _ := json.Marshal(MatchSuccess{
			Type:       "match_success",
			RoomID:     roomID,
			OpponentID: c2.ID,
			YourID:     c1.ID,
		})
		msg2, _ := json.Marshal(MatchSuccess{
			Type:       "match_success",
			RoomID:     roomID,
			OpponentID: c1.ID,
			YourID:     c2.ID,
		})
		c1.Send <- msg1
		c2.Send <- msg2

		log.Printf("Match created: Room %s, Player1 %s, Player2 %s", roomID, c1.ID, c2.ID)
	}
}
