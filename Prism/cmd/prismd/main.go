package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"prismd/internal/api"
	"prismd/internal/memory"
)

func main() {
	port := flag.Int("port", 5666, "监听端口")
	dataDir := flag.String("data", "./data", "数据目录")
	domain := flag.String("domain", "Atri", "域名称")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	// 每个域一个独立的数据库文件
	dbPath := filepath.Join(*dataDir, fmt.Sprintf("prismd_%s.bolt", *domain))
	graph, err := memory.NewGraph(dbPath)
	if err != nil {
		log.Fatalf("无法打开数据库: %v", err)
	}
	defer graph.Close()

	log.Printf("已从数据库恢复 %d 个节点, %d 条突触 (域: %s)", len(graph.Nodes()), len(graph.Synapses()), *domain)

	handler := api.NewPrimQLHandler(graph)
	http.Handle("/", handler)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("正在关闭数据库...")
		graph.Close()
		os.Exit(0)
	}()

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[PrismD] 数字海马体已启动，监听 %s (域: %s)", addr, *domain)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
