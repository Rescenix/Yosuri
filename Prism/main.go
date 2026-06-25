package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"prismd/internal/api"
	"prismd/internal/memory"
)

func main() {
	port := flag.Int("port", 5666, "监听端口")
	dbPath := flag.String("db", "./prismd.bolt", "bolt 数据库路径")
	flag.Parse()

	graph, err := memory.NewGraph(*dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer graph.Close()

	handler := api.NewPrimQLHandler(graph)
	http.Handle("/", handler)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("正在关闭 PrismD...")
		graph.Close()
		os.Exit(0)
	}()

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("PrismD 数字海马体已启动，监听 %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
