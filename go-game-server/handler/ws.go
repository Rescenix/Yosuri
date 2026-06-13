package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"star-trail/game-server/match"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var Manager = match.NewManager()

func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	clientID := uuid.New().String()
	client := &match.Client{
		ID:      clientID,
		Conn:    conn,
		Send:    make(chan []byte, 256),
		Manager: Manager,
	}

	Manager.Register <- client
	go writePump(client)
	go readPump(client)

	client.Send <- []byte(`{"type":"welcome","yourId":"` + clientID + `"}`)
	log.Printf("New client connected: %s", clientID)
}

func readPump(c *match.Client) {
	defer func() {
		c.Manager.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(4096)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}

		var msg match.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Invalid message from %s: %v", c.ID, err)
			continue
		}

		switch msg.Type {
		case "get_rooms":
			Manager.GetRoomList(c)
		case "create_room":
			Manager.CreateRoom(c, msg)
		case "join_room":
			Manager.JoinRoom(c, msg)
		case "leave_room":
			Manager.LeaveRoom(c, msg)
		case "disband_room":
			Manager.DisbandRoom(c, msg)
		case "kick_member":
			Manager.KickMember(c, msg)
		case "start_battle":
			Manager.StartBattle(c, msg) // 新增
		case "battle_action":
			Manager.BroadcastBattleAction(c, message)
		default:
			c.Send <- []byte(`{"type":"error","message":"unknown message type"}`)
		}
	}
}

func writePump(c *match.Client) {
	ticker := time.NewTicker(50 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
