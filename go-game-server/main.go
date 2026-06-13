package main

import (
	"log"
	"net/http"
	"os"

	"star-trail/game-server/handler"
	"star-trail/game-server/market"

	"github.com/joho/godotenv"
)

// CORS 中间件
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-user-id")

		// 直接响应预检请求
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func main() {
	godotenv.Load()

	// 初始化数据库和 Redis
	market.InitDB()
	market.InitRedis()

	// 注册交易行路由（全部加上 CORS 中间件）
	http.HandleFunc("/api/market/list", corsMiddleware(market.ListItem))
	http.HandleFunc("/api/market/buy", corsMiddleware(market.BuyItem))
	http.HandleFunc("/api/market/search", corsMiddleware(market.SearchItems))
	http.HandleFunc("/api/market/cancel", corsMiddleware(market.CancelListing))

	// WebSocket（一般不需要 CORS，但可以单独处理）
	http.HandleFunc("/ws", handler.WebSocketHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server running on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
