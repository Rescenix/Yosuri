// handler/memory.go — 极简记忆存储，不依赖 BGE

package handler

import (
	"backend/internal/database"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type MemoryRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	ID        string    `json:"id,omitempty"`
}

type MemoryStore struct {
	filePath  string
	records   []MemoryRecord
	prismAddr string
}

func NewMemoryStore(path string) *MemoryStore {
	store := &MemoryStore{
		filePath: path,
	}
	if path != "" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			panic("无法创建记忆数据目录: " + err.Error())
		}
		data, err := os.ReadFile(path)
		if err == nil {
			json.Unmarshal(data, &store.records)
		}
		fmt.Printf("📂 记忆文件路径: %s\n", path)
	}
	return store
}

func (m *MemoryStore) ConnectPrism(addr string) error {
	resp, err := http.Get("http://" + addr)
	if err != nil {
		return err
	}
	resp.Body.Close()
	m.prismAddr = addr
	return nil
}

func (m *MemoryStore) sendPrimQL(ql string) (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post("http://"+m.prismAddr, "text/plain", strings.NewReader(ql))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

// SmartAppend 不再生成向量，直接写入 PrismD（如果可用），否则回退 JSON
func (m *MemoryStore) SmartAppend(role, content string) error {
	_, err := database.PrismDB.Exec("ENGRAM " + role + " " + content)
	return err
}

// SearchSimilar 不再调用 BGE/LOOM，返回空
func (m *MemoryStore) SearchSimilar(query string, topK int) []MemoryRecord {
	// 暂时停用外部检索
	fmt.Println("⚠️ SearchSimilar 已停用，返回空")
	return nil
}

func (m *MemoryStore) GetRecent(limit int) []MemoryRecord {
	if len(m.records) <= limit {
		return m.records
	}
	return m.records[len(m.records)-limit:]
}

func (m *MemoryStore) SaveMemoryHandler(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{"error": "记忆写入已禁用，由对话流自动保存"})
}

func (m *MemoryStore) RecallMemoryHandler(c *gin.Context) {
	query := c.Query("q")
	topK := 20
	if query != "" {
		records := m.SearchSimilar(query, topK)
		c.JSON(http.StatusOK, records)
		return
	}
	records := m.GetRecent(topK)
	c.JSON(http.StatusOK, records)
}
