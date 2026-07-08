package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

// SessionStore 维护所有会话的对话历史与压缩游标
type SessionStore struct {
	mu                  sync.RWMutex
	sessions            map[string][]DSMessage
	lastCompressIndexes map[string]int // 每个 session 上次压缩的消息数量
	filePath            string
}

// persistedMessage 是 DSMessage 面向磁盘持久化的镜像。
// DSMessage.Timestamp 打了 json:"-"，是为了不把时间戳带进发给 LLM 的请求体；
// 但这导致 SessionStore 落盘/读盘时会静默丢失所有历史消息的时间戳。
// 这里单独定义一个带 timestamp 字段的结构体用于持久化，两头互不影响。
type persistedMessage struct {
	Role             string          `json:"role"`
	Content          string          `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	Timestamp        time.Time       `json:"timestamp"`
	ToolCalls        []core.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
	Model            string          `json:"model,omitempty"`
}

func toPersistedMessages(msgs []DSMessage) []persistedMessage {
	out := make([]persistedMessage, len(msgs))
	for i, m := range msgs {
		out[i] = persistedMessage{
			Role:             m.Role,
			Content:          m.Content,
			ReasoningContent: m.ReasoningContent,
			Timestamp:        m.Timestamp,
			ToolCalls:        m.ToolCalls,
			ToolCallID:       m.ToolCallID,
			Model:            m.Model,
		}
	}
	return out
}

func fromPersistedMessages(msgs []persistedMessage) []DSMessage {
	out := make([]DSMessage, len(msgs))
	for i, m := range msgs {
		out[i] = DSMessage{
			Role:             m.Role,
			Content:          m.Content,
			ReasoningContent: m.ReasoningContent,
			Timestamp:        m.Timestamp,
			ToolCalls:        m.ToolCalls,
			ToolCallID:       m.ToolCallID,
			Model:            m.Model,
		}
	}
	return out
}

// 用于 JSON 持久化的结构体
type sessionFileData struct {
	Sessions            map[string][]persistedMessage `json:"sessions"`
	LastCompressIndexes map[string]int                `json:"last_compress_indexes"`
}

func NewSessionStore(filePath string) *SessionStore {
	store := &SessionStore{
		sessions:            make(map[string][]DSMessage),
		lastCompressIndexes: make(map[string]int),
		filePath:            filePath,
	}
	dir := filepath.Dir(filePath)
	os.MkdirAll(dir, 0755)
	store.LoadFromFile(filePath)
	return store
}

// Append 追加消息，自动补时间戳，异步持久化
func (s *SessionStore) Append(sessionID string, msg DSMessage) {
	s.mu.Lock()
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	s.sessions[sessionID] = append(s.sessions[sessionID], msg)
	s.mu.Unlock()

	go func() {
		if err := s.SaveToFile(s.filePath); err != nil {
			log.Printf("保存会话失败: %v", err)
		}
	}()
}

// Get 返回指定会话的消息切片（副本）
func (s *SessionStore) Get(sessionID string) []DSMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.sessions[sessionID]
	if msgs == nil {
		return nil
	}
	copied := make([]DSMessage, len(msgs))
	copy(copied, msgs)
	return copied
}

// GetCompressIndex 获取上次压缩位置（已压缩的消息数量）
func (s *SessionStore) GetCompressIndex(sessionID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastCompressIndexes[sessionID]
}

// SetCompressIndex 更新压缩游标（通常在成功压缩后调用）
func (s *SessionStore) SetCompressIndex(sessionID string, index int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastCompressIndexes[sessionID] = index
}

// AllSessions 返回所有会话消息的快照副本（按 sessionID 分组），供统计聚合使用
func (s *SessionStore) AllSessions() map[string][]DSMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]DSMessage, len(s.sessions))
	for id, msgs := range s.sessions {
		copied := make([]DSMessage, len(msgs))
		copy(copied, msgs)
		out[id] = copied
	}
	return out
}

// List 列出所有会话摘要
func (s *SessionStore) List() []SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var infos []SessionInfo
	for id, msgs := range s.sessions {
		if len(msgs) == 0 {
			continue
		}
		title := "新对话"
		for _, m := range msgs {
			if m.Role == "user" {
				title = m.Content
				break
			}
		}
		infos = append(infos, SessionInfo{
			ID:        id,
			Title:     title,
			UpdatedAt: msgs[len(msgs)-1].Timestamp,
		})
	}
	return infos
}

// SaveToFile 持久化所有会话及压缩游标
func (s *SessionStore) SaveToFile(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	persistedSessions := make(map[string][]persistedMessage, len(s.sessions))
	for id, msgs := range s.sessions {
		persistedSessions[id] = toPersistedMessages(msgs)
	}
	data := sessionFileData{
		Sessions:            persistedSessions,
		LastCompressIndexes: s.lastCompressIndexes,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, jsonData, 0644)
}

// LoadFromFile 从 JSON 文件恢复所有会话及游标
func (s *SessionStore) LoadFromFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fileData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var data sessionFileData
	if err := json.Unmarshal(fileData, &data); err != nil {
		return err
	}

	s.sessions = make(map[string][]DSMessage, len(data.Sessions))
	for id, msgs := range data.Sessions {
		s.sessions[id] = fromPersistedMessages(msgs)
	}

	s.lastCompressIndexes = data.LastCompressIndexes
	if s.lastCompressIndexes == nil {
		s.lastCompressIndexes = make(map[string]int)
	}
	return nil
}

// SessionInfo 会话摘要
type SessionInfo struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AllMessage 所有会话的扁平消息
type AllMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// GetAllMessagesHandler 获取所有会话的全部消息（按时间排序）
func GetAllMessagesHandler(store *SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if store == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "session store not initialized"})
			return
		}
		store.mu.RLock()
		defer store.mu.RUnlock()

		var all []AllMessage
		for _, msgs := range store.sessions {
			for _, msg := range msgs {
				all = append(all, AllMessage{
					Role:      msg.Role,
					Content:   msg.Content,
					Timestamp: msg.Timestamp,
				})
			}
		}
		sort.Slice(all, func(i, j int) bool {
			return all[i].Timestamp.Before(all[j].Timestamp)
		})
		c.JSON(http.StatusOK, all)
	}
}
