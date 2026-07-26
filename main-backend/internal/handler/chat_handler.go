package handler

type ChatHandler struct {
	sessionStore         *SessionStore
	lastImageDescription string // 临时存储图片描述
}

func NewChatHandler(s *SessionStore) *ChatHandler {
	return &ChatHandler{sessionStore: s}
}
