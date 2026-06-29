package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"prismd/internal/api"
	"prismd/internal/domain"
	"syscall"
)

// 在文件头部添加 import

func main() {
	port := flag.Int("port", 5666, "监听端口")
	dataDir := flag.String("data", "./data", "数据目录")
	startDomain := flag.String("domain", "Atri", "启动时激活的域名称")
	flag.Parse()

	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	// 初始化域管理器
	manager := domain.NewManager(*dataDir)
	if *startDomain != "" {
		if err := manager.Use(*startDomain); err != nil {
			log.Fatalf("启动域失败: %v", err)
		}
	}

	log.Printf("已从数据库恢复 %d 个节点, %d 条突触 (域: %s)",
		len(manager.CurrentGraph().Nodes()), len(manager.CurrentGraph().Synapses()), manager.CurrentDomain())

	handler := api.NewPrimQLHandler(manager)
	http.Handle("/", handler)

	// 优雅停机
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("正在关闭所有域...")
		manager.Close()
		os.Exit(0)
	}()

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[PrismD] 数字海马体已启动，监听 %s (域: %s)", addr, manager.CurrentDomain())
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
