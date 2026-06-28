// handler/session_v2.go
package handler

import (
	"fmt"
	"net/http"
	"time"

	"backend/internal/database"

	"github.com/gin-gonic/gin"
)

// SessionV2 会话元数据
type SessionV2 struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateSession 创建新会话（ENGRAM session）
func CreateSession(c *gin.Context) {
	now := time.Now()
	title := "新对话"
	if reqTitle := c.Query("title"); reqTitle != "" {
		title = reqTitle
	}
	sql := fmt.Sprintf("ENGRAM session %s", title)
	result, err := database.PrismDB.Exec(sql)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	id, _ := result.LastInsertId()
	sessionID := fmt.Sprintf("%d", id)

	c.JSON(http.StatusOK, SessionV2{
		ID:        sessionID,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// ListSessions 列出所有会话（STATS FULL 过滤 session 角色）
func ListSessions(c *gin.Context) {
	rows, err := database.PrismDB.Query("STATS FULL")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var sessions []SessionV2
	for rows.Next() {
		var id, role, content, energyStr string
		if err := rows.Scan(&id, &role, &content, &energyStr); err != nil {
			continue
		}
		if role != "session" {
			continue
		}
		sessions = append(sessions, SessionV2{
			ID:    id,
			Title: content,
		})
	}
	if sessions == nil {
		sessions = []SessionV2{}
	}
	c.JSON(http.StatusOK, sessions)
}

// RenameSession 重命名会话（REFRACT）
func RenameSession(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		Title string `json:"title" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少标题"})
		return
	}
	sql := fmt.Sprintf(`REFRACT {"id": %s, "content": "%s"}`, id, body.Title)
	if _, err := database.PrismDB.Exec(sql); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// DeleteSession 删除会话（PRUNE）
func DeleteSession(c *gin.Context) {
	id := c.Param("id")
	sql := fmt.Sprintf("PRUNE %s", id)
	if _, err := database.PrismDB.Exec(sql); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
