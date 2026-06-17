package handler

type ChatHandler struct {
	memoryStore          *MemoryStore
	sessionStore         *SessionStore
	lastImageDescription string // 临时存储图片描述
}

func NewChatHandler(m *MemoryStore, s *SessionStore) *ChatHandler {
	return &ChatHandler{memoryStore: m, sessionStore: s}
}
