package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// localNotification 本地通知（不依赖云端，脚本/前端可直接创建）。
type localNotification struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Icon      string `json:"icon,omitempty"`
	CreatedAt string `json:"created_at"`
	IsRead    bool   `json:"is_read"`
}

var (
	localNotifMu   sync.Mutex
	localNotifFile string // 懒初始化
)

func localNotifPath() string {
	if localNotifFile != "" {
		return localNotifFile
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	localNotifFile = filepath.Join(home, "rescene_data", "notifications_local.json")
	return localNotifFile
}

func loadLocalNotifs() []localNotification {
	raw, err := os.ReadFile(localNotifPath())
	if err != nil {
		return nil
	}
	var arr []localNotification
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	// 过滤掉太旧的（只留 30 天）
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	var filtered []localNotification
	for _, n := range arr {
		t, err := time.Parse(time.RFC3339, n.CreatedAt)
		if err != nil || t.After(cutoff) {
			filtered = append(filtered, n)
		}
	}
	return filtered
}

func saveLocalNotifs(arr []localNotification) {
	path := localNotifPath()
	os.MkdirAll(filepath.Dir(path), 0700)
	data, _ := json.MarshalIndent(arr, "", "  ")
	os.WriteFile(path, data, 0600)
}

// HandleLocalNotifCreate POST /api/notifications/local
// 创建本地通知，body: {title, body, icon?}
func HandleLocalNotifCreate(c *gin.Context) {
	var req struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		Icon  string `json:"icon,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Body = strings.TrimSpace(req.Body)
	if req.Title == "" && req.Body == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title 或 body 至少填一个"})
		return
	}

	localNotifMu.Lock()
	defer localNotifMu.Unlock()

	arr := loadLocalNotifs()
	n := localNotification{
		ID:        "local_" + time.Now().Format("20060102_150405.000"),
		Title:     req.Title,
		Body:      req.Body,
		Icon:      req.Icon,
		CreatedAt: time.Now().Format(time.RFC3339),
		IsRead:    false,
	}
	arr = append([]localNotification{n}, arr...)
	saveLocalNotifs(arr)

	c.JSON(http.StatusOK, gin.H{"ok": true, "id": n.ID})
}

// HandleLocalNotifList GET /api/notifications/local
// 返回本地通知列表（按时间倒序）+ unread_count
func HandleLocalNotifList(c *gin.Context) {
	localNotifMu.Lock()
	arr := loadLocalNotifs()
	localNotifMu.Unlock()

	unread := 0
	for _, n := range arr {
		if !n.IsRead {
			unread++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"notifications":  arr,
		"unread_count": unread,
	})
}

// HandleLocalNotifRead POST /api/notifications/read/local
// 标记本地通知已读，body: {ids: ["local_xxx", ...]} 或 {}（全部标已读）
func HandleLocalNotifRead(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	c.ShouldBindJSON(&req)

	localNotifMu.Lock()
	defer localNotifMu.Unlock()

	arr := loadLocalNotifs()
	if len(req.IDs) > 0 {
		idSet := make(map[string]bool, len(req.IDs))
		for _, id := range req.IDs {
			idSet[id] = true
		}
		for i := range arr {
			if idSet[arr[i].ID] {
				arr[i].IsRead = true
			}
		}
	} else {
		for i := range arr {
			arr[i].IsRead = true
		}
	}
	saveLocalNotifs(arr)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}