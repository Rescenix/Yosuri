package aap

import (
	"log"
	"sync"
)

// AgentNode 代表一个挂载在 AAP 总线上的自治服务节点
type AgentNode struct {
	Name   string
	Send   func(event AAPEvent) error // 发送事件到该节点的函数
	IsDead bool
}

// AAPServer 是 Aurora 应用协议的核心服务端，负责管理 Agent 节点和事件广播
type AAPServer struct {
	mu       sync.RWMutex
	agents   map[string]*AgentNode // 已注册的 Agent 节点
	eventBus chan AAPEvent         // 事件广播总线
	quit     chan struct{}
}

// NewAAPServer 创建并启动一个 AAP 服务器实例
func NewAAPServer() *AAPServer {
	s := &AAPServer{
		agents:   make(map[string]*AgentNode),
		eventBus: make(chan AAPEvent, 256),
		quit:     make(chan struct{}),
	}
	// 启动后台事件监听协程
	go s.processEvents()
	return s
}

// Register 将一个 Agent 节点注册到 AAP 总线上
func (s *AAPServer) Register(name string, sendFunc func(AAPEvent) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[name] = &AgentNode{
		Name: name,
		Send: sendFunc,
	}
	log.Printf("[AAP] Agent registered: %s", name)
}

// Unregister 从总线上移除一个 Agent 节点
func (s *AAPServer) Unregister(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.agents, name)
	log.Printf("[AAP] Agent unregistered: %s", name)
}

// Broadcast 将事件广播给所有已注册的 Agent 节点。发送失败或节点已死亡会被自动跳过。
func (s *AAPServer) Broadcast(event AAPEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for name, agent := range s.agents {
		if agent.IsDead {
			continue
		}
		if err := agent.Send(event); err != nil {
			log.Printf("[AAP] Agent %s send error: %v", name, err)
			agent.IsDead = true // 标记为死亡，下次心跳时清除
		}
	}
}

// processEvents 是后台协程，持续监听事件总线并触发广播
func (s *AAPServer) processEvents() {
	for {
		select {
		case event := <-s.eventBus:
			s.Broadcast(event)
		case <-s.quit:
			return
		}
	}
}

// Publish 将事件推送到事件总线，触发异步广播
func (s *AAPServer) Publish(event AAPEvent) {
	s.eventBus <- event
}

// Shutdown 优雅地关闭 AAP 服务器
func (s *AAPServer) Shutdown() {
	close(s.quit)
}
