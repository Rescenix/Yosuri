// backend/internal/handler/blog.go
package handler

import (
	"backend/internal/database"
	"fmt"
	"time"
)

func generateBlogPost(topic string) string {
	prompt := fmt.Sprintf(`请以杉汐的口吻，写一篇简短的博客文章，主题是：%s。
要求：300-500字，自然、温馨、有个人风格。`, topic)
	content := askDeepSeekSimple(prompt)

	if content != "" {
		title := topic
		if len([]rune(title)) > 50 {
			title = string([]rune(title)[:50]) + "..."
		}
		slug := time.Now().Format("2006-01-02-150405")
		// 插入时增加 tags 字段，默认值为 '["故事"]'
		_, err := database.DB.Exec("INSERT INTO posts (title, slug, content, tags) VALUES (?, ?, ?, ?)",
			title, slug, content, `["故事"]`)
		if err != nil {
			fmt.Printf("⚠️ 博客发布失败: %v\n", err)
		} else {
			fmt.Println("✅ 博客已自动发布到数据库")
		}
	}
	return content
}
