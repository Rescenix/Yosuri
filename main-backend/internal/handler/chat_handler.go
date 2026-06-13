package handler

type ChatHandler struct {
	memoryStore  *MemoryStore
	sessionStore *SessionStore
}

func NewChatHandler(m *MemoryStore, s *SessionStore) *ChatHandler {
	return &ChatHandler{memoryStore: m, sessionStore: s}
}
