package match

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	ID      string
	Conn    *websocket.Conn
	Send    chan []byte
	Manager *Manager
	Room    *Room
	mu      sync.Mutex
}

type Room struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	OwnerName    string   `json:"ownerName"`
	OwnerID      string   `json:"ownerId"`
	Players      int      `json:"players"`
	MaxPlayers   int      `json:"maxPlayers"`
	Difficulty   string   `json:"difficulty"`
	HasPassword  bool     `json:"hasPassword"`
	MinGearScore int      `json:"minGearScore"`
	Members      []Member `json:"members"`
	Password     string   `json:"-"`
	Status       string   `json:"status"`
}

type Member struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	GearScore int    `json:"gearScore"`
	IsOwner   bool   `json:"isOwner"`
}

type Manager struct {
	mu         sync.Mutex
	Clients    map[string]*Client
	Queue      []*Client
	Register   chan *Client
	Unregister chan *Client
	Rooms      map[string]*Room
}

type Message struct {
	Type         string `json:"type"`
	PlayerID     string `json:"playerId,omitempty"`
	PlayerName   string `json:"playerName,omitempty"`
	RoomID       string `json:"roomId,omitempty"`
	MemberID     string `json:"memberId,omitempty"`
	BossID       string `json:"bossId,omitempty"`
	MaxPlayers   int    `json:"maxPlayers,omitempty"`
	Difficulty   string `json:"difficulty,omitempty"`
	Password     string `json:"password,omitempty"`
	MinGearScore int    `json:"minGearScore,omitempty"`
	OwnerName    string `json:"ownerName,omitempty"`
	OwnerID      string `json:"ownerId,omitempty"`
}

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

func (m *Manager) Run() {
	for {
		select {
		case client := <-m.Register:
			m.mu.Lock()
			m.Clients[client.ID] = client
			m.mu.Unlock()
		case client := <-m.Unregister:
			m.mu.Lock()
			if _, ok := m.Clients[client.ID]; ok {
				delete(m.Clients, client.ID)
				if client.Room != nil {
					m.handleLeaveRoom(client)
				}
				for i, c := range m.Queue {
					if c.ID == client.ID {
						m.Queue = append(m.Queue[:i], m.Queue[i+1:]...)
						break
					}
				}
			}
			m.mu.Unlock()
		}
	}
}

func (m *Manager) CreateRoom(client *Client, msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	roomID := uuid.New().String()
	room := &Room{
		ID:           roomID,
		Name:         msg.BossID,
		OwnerName:    msg.OwnerName,
		OwnerID:      client.ID,
		Players:      1,
		MaxPlayers:   msg.MaxPlayers,
		Difficulty:   msg.Difficulty,
		HasPassword:  msg.Password != "",
		MinGearScore: msg.MinGearScore,
		Password:     msg.Password,
		Status:       "waiting",
		Members: []Member{
			{ID: client.ID, Name: msg.OwnerName, GearScore: 0, IsOwner: true},
		},
	}
	m.Rooms[roomID] = room
	client.Room = room

	resp, _ := json.Marshal(map[string]interface{}{"type": "room_created", "room": room})
	client.Send <- resp
	m.broadcastRoomList()
}

func (m *Manager) JoinRoom(client *Client, msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.Rooms[msg.RoomID]
	if !ok || room.Status != "waiting" {
		client.Send <- []byte(`{"type":"error","message":"房间不存在或已开始"}`)
		return
	}
	if room.Players >= room.MaxPlayers {
		client.Send <- []byte(`{"type":"error","message":"房间已满"}`)
		return
	}
	if room.HasPassword && msg.Password != room.Password {
		client.Send <- []byte(`{"type":"error","message":"密码错误"}`)
		return
	}
	for _, member := range room.Members {
		if member.ID == client.ID {
			client.Send <- []byte(`{"type":"error","message":"你已在房间中"}`)
			return
		}
	}

	room.Members = append(room.Members, Member{ID: client.ID, Name: msg.PlayerName, GearScore: 0, IsOwner: false})
	room.Players = len(room.Members)
	client.Room = room

	joined, _ := json.Marshal(map[string]interface{}{"type": "room_joined", "room": room})
	client.Send <- joined
	m.broadcastToRoom(room, map[string]interface{}{"type": "room_updated", "room": room})
	m.broadcastRoomList()
}

func (m *Manager) StartBattle(client *Client, msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room := client.Room
	if room == nil || room.OwnerID != client.ID {
		client.Send <- []byte(`{"type":"error","message":"无权限"}`)
		return
	}
	if room.Players != room.MaxPlayers {
		client.Send <- []byte(`{"type":"error","message":"人数不足"}`)
		return
	}
	room.Status = "playing"
	m.broadcastToRoom(room, map[string]interface{}{"type": "match_success", "roomId": room.ID})
	m.broadcastRoomList()
}

func (m *Manager) LeaveRoom(client *Client, msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handleLeaveRoom(client)
}

func (m *Manager) DisbandRoom(client *Client, msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room := client.Room
	if room == nil || room.OwnerID != client.ID {
		client.Send <- []byte(`{"type":"error","message":"无权限"}`)
		return
	}
	for _, member := range room.Members {
		if c := m.Clients[member.ID]; c != nil {
			c.Room = nil
			// 加上 roomId
			disbandMsg, _ := json.Marshal(map[string]interface{}{
				"type":   "room_disbanded",
				"roomId": room.ID,
			})
			c.Send <- disbandMsg
		}
	}
	delete(m.Rooms, room.ID)
	m.broadcastRoomList()
}
func (m *Manager) KickMember(client *Client, msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	room := client.Room
	if room == nil || room.OwnerID != client.ID {
		client.Send <- []byte(`{"type":"error","message":"无权限"}`)
		return
	}

	var target *Client
	for i, member := range room.Members {
		if member.ID == msg.MemberID {
			if member.ID == client.ID {
				client.Send <- []byte(`{"type":"error","message":"不能踢自己"}`)
				return
			}
			target = m.Clients[member.ID]
			room.Members = append(room.Members[:i], room.Members[i+1:]...)
			room.Players = len(room.Members)
			break
		}
	}
	if target != nil {
		target.Room = nil
		target.Send <- []byte(`{"type":"kicked","message":"你已被踢出房间"}`)
		// 直接给被踢者发送当前房间列表（不加锁，因为外层已持有锁）
		rooms := make([]*Room, 0, len(m.Rooms))
		for _, r := range m.Rooms {
			rooms = append(rooms, r)
		}
		resp, _ := json.Marshal(map[string]interface{}{"type": "room_list", "rooms": rooms})
		target.Send <- resp
	}
	// 现在可以正常广播了
	m.broadcastToRoom(room, map[string]interface{}{"type": "room_updated", "room": room})
	m.broadcastRoomList()
}

func (m *Manager) GetRoomList(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rooms := make([]*Room, 0, len(m.Rooms))
	for _, room := range m.Rooms {
		rooms = append(rooms, room)
	}
	resp, _ := json.Marshal(map[string]interface{}{"type": "room_list", "rooms": rooms})
	client.Send <- resp
}

func (m *Manager) BroadcastBattleAction(c *Client, message []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c.Room != nil {
		for _, member := range c.Room.Members {
			opp := m.Clients[member.ID]
			if opp != nil && opp.ID != c.ID {
				opp.Send <- message
			}
		}
	}
}

// 内部方法
func (m *Manager) handleLeaveRoom(client *Client) {
	room := client.Room
	if room == nil {
		return
	}
	client.Room = nil
	for i, member := range room.Members {
		if member.ID == client.ID {
			room.Members = append(room.Members[:i], room.Members[i+1:]...)
			room.Players = len(room.Members)
			break
		}
	}
	if room.Players == 0 {
		delete(m.Rooms, room.ID)
	} else {
		if room.OwnerID == client.ID && room.Players > 0 {
			newOwner := room.Members[0]
			room.OwnerID = newOwner.ID
			room.OwnerName = newOwner.Name
			for i := range room.Members {
				if room.Members[i].ID == newOwner.ID {
					room.Members[i].IsOwner = true
				}
			}
			if c := m.Clients[newOwner.ID]; c != nil {
				c.Send <- []byte(`{"type":"room_owner_changed","message":"你已成为房主"}`)
			}
		}
		m.broadcastToRoom(room, map[string]interface{}{"type": "room_updated", "room": room})
	}
	m.broadcastRoomList()
}

func (m *Manager) broadcastToRoom(room *Room, msg interface{}) {
	data, _ := json.Marshal(msg)
	for _, member := range room.Members {
		if c := m.Clients[member.ID]; c != nil {
			select {
			case c.Send <- data:
			default:
			}
		}
	}
}

func (m *Manager) broadcastRoomList() {
	// 删除 m.mu.Lock() 和 defer m.mu.Unlock()
	rooms := make([]*Room, 0, len(m.Rooms))
	for _, room := range m.Rooms {
		rooms = append(rooms, room)
	}
	data, err := json.Marshal(map[string]interface{}{"type": "room_list", "rooms": rooms})
	if err != nil {
		log.Printf("marshal error: %v", err)
		return
	}
	for id, c := range m.Clients {
		select {
		case c.Send <- data:
			log.Printf("broadcastRoomList: 发送给客户端 %s 成功", id)
		default:
			log.Printf("broadcastRoomList: 发送给客户端 %s 失败，channel 已满", id)
		}
	}
}
