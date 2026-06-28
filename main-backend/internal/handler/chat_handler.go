package handler

import "sync"

type ChatHandler struct {
	memoryStore          *MemoryStore
	sessionStore         *SessionStore
	lastImageDescription string // 临时存储图片描述
	// 新增本地模型熔断字段
	localModelFails   int
	localModelBlocked bool
	localModelMu      sync.Mutex
}

func NewChatHandler(m *MemoryStore, s *SessionStore) *ChatHandler {
	return &ChatHandler{memoryStore: m, sessionStore: s}
}
