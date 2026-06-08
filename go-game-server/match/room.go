package match

// Room 代表一个对战房间，包含两个客户端
type Room struct {
	ID      string
	Clients [2]*Client
}

// NewRoom 创建房间
func NewRoom(id string, c1, c2 *Client) *Room {
	return &Room{
		ID:      id,
		Clients: [2]*Client{c1, c2},
	}
}

// Broadcast 向房间内所有客户端广播消息（除自己外）
func (r *Room) Broadcast(sender *Client, message []byte) {
	for _, c := range r.Clients {
		if c.ID != sender.ID {
			select {
			case c.Send <- message:
			default:
				close(c.Send)
			}
		}
	}
}
