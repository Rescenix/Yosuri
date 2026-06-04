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

	"github.com/gin-gonic/gin"
)

// SessionStore 用内存 map 维护所有会话的对话历史
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string][]DSMessage
	filePath string
}

func NewSessionStore(filePath string) *SessionStore {
	store := &SessionStore{
		sessions: make(map[string][]DSMessage),
		filePath: filePath,
	}
	dir := filepath.Dir(filePath)
	os.MkdirAll(dir, 0755)
	store.LoadFromFile(filePath)
	return store
}

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

func (s *SessionStore) Get(sessionID string) []DSMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sessionID]
}

type SessionInfo struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *SessionStore) List() []SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	var infos []SessionInfo
	for id, msgs := range s.sessions {
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

func (s *SessionStore) SaveToFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(s.sessions)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *SessionStore) LoadFromFile(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &s.sessions)
}

// ---------- 新增：获取所有会话的所有消息（按时间排序） ----------
type AllMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// GetAllMessages 获取所有会话的全部消息（按时间排序）
// GetAllMessages 获取所有会话的全部消息（按时间排序）
func GetAllMessages(c *gin.Context, store *SessionStore) {
	if store == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session store not initialized"})
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()

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
