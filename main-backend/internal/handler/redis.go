package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	redisClient  *redis.Client
	redisEnabled bool
)

func init() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redisClient = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		fmt.Printf("[WARN] Redis 连接失败，将直接计算分页: %v\n", err)
		redisEnabled = false
	} else {
		redisEnabled = true
		fmt.Println("[INFO] Redis 已连接，分页缓存启用")
	}
}

func getPagesFromCache(key string) ([]string, error) {
	// 如果 Redis 没开，直接返回“缓存未命中”
	if !redisEnabled {
		return nil, nil
	}

	ctx := context.Background()
	val, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var pages []string
	if err := json.Unmarshal([]byte(val), &pages); err != nil {
		return nil, err
	}
	return pages, nil
}

func setPagesToCache(key string, pages []string) error {
	// 如果 Redis 没开，直接跳过缓存写入
	if !redisEnabled {
		return nil
	}

	ctx := context.Background()
	data, _ := json.Marshal(pages)
	return redisClient.Set(ctx, key, data, 24*time.Hour).Err()
}
