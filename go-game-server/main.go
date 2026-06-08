package main

import (
	"log"
	"net/http"
	"os"

	"star-trail/game-server/handler"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/ws", handler.WebSocketHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	log.Printf("Go game server with matching listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
