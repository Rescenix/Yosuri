package match

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client 代表一个 WebSocket 连接的玩家
type Client struct {
	ID       string
	Conn     *websocket.Conn
	Send     chan []byte // 发送消息的缓冲通道
	Manager  *Manager
	Room     *Room
	Opponent *Client // 匹配到的对手
	mu       sync.Mutex
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

// NewClient 初始化客户端
func NewClient(id string, conn *websocket.Conn, mgr *Manager) *Client {
	return &Client{
		ID:      id,
		Conn:    conn,
		Send:    make(chan []byte, 256),
		Manager: mgr,
	}
}

// ReadPump 从 WebSocket 读取消息，调用 Manager 处理
func (c *Client) ReadPump() {
	defer func() {
		c.Manager.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		// 将消息交给 Manager 处理
		c.Manager.HandleMessage(c, message)
	}
}

// WritePump 向 WebSocket 写入消息
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
