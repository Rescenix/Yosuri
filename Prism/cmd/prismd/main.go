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

	// 首次启动播种默认簇（注册表已有内容时为 no-op），保证旧硬编码系统平滑过渡
	defaultClusters := map[string]string{
		"UserBase":     "用户身份、长期偏好、稳定事实",
		"CodeWork":     "项目架构、代码决策、技术约束",
		"Capabilities": "工具能力与可用功能",
		"Decisions":    "关键决策与结论",
	}
	if g := manager.CurrentGraph(); g != nil {
		g.SeedDefaultClusters(defaultClusters)
	}

	// RESTful 簇管理端点 + PrimQL 文本协议 catch-all
	mux := http.NewServeMux()
	mux.HandleFunc("/clusters", handler.HandleListClusters)  // GET
	mux.HandleFunc("/cluster", handler.HandleCreateCluster)  // POST
	mux.HandleFunc("/cluster/", handler.HandleDeleteCluster) // DELETE /cluster/:name
	mux.HandleFunc("/kv/", handler.HandleKV)                 // GET/PUT/DELETE /kv/:domain/:key，绕开图语义的原始存取
	mux.Handle("/", handler)

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
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
