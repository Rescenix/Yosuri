package market

import (
	"context"
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

var (
	DB  *sql.DB
	RDB *redis.Client
	Ctx = context.Background()
)

func InitDB() {
	dsn := os.Getenv("DATABASE_URL")
	log.Printf("DATABASE_URL loaded: %s", dsn) // 调试
	if dsn == "" {
		dsn = "postgres://postgres:123456@localhost:5432/startrail?sslmode=disable"
	}
	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("PostgreSQL connect failed: %v", err)
	}
	DB.SetMaxOpenConns(25)
	DB.SetMaxIdleConns(10)
	if err = DB.Ping(); err != nil {
		log.Fatalf("PostgreSQL ping failed: %v", err)
	}
	log.Println("PostgreSQL connected")
}

func InitRedis() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	RDB = redis.NewClient(&redis.Options{Addr: addr, DB: 0})
	if err := RDB.Ping(Ctx).Err(); err != nil {
		log.Printf("Redis connect failed: %v (caching disabled)", err)
		RDB = nil
	} else {
		log.Println("Redis connected")
	}
}
