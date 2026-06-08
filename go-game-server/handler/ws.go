package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"star-trail/game-server/match"
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
	client := match.NewClient(clientID, conn, Manager)

	Manager.Register <- client
	go client.WritePump()
	go client.ReadPump()

	client.Send <- []byte(`{"type":"welcome","yourId":"` + clientID + `"}`)
	log.Printf("New client connected: %s", clientID)
	time.Sleep(10 * time.Millisecond)
}
