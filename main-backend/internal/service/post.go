package service

import (
	"backend/internal/database"
	"backend/internal/model"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

func GetAllPosts() ([]model.Post, error) {
	rows, err := database.DB.Query("SELECT id, title, slug, content, tags, cover_image, custom_url, created_at FROM posts ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []model.Post
	for rows.Next() {
		var p model.Post
		var tagsJSON []byte
		var cover, custom sql.NullString
		err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Content, &tagsJSON, &cover, &custom, &p.CreatedAt)
		if err != nil {
			// 关键：直接返回错误，不要继续
			return nil, fmt.Errorf("扫描行失败: %w", err)
		}
		if len(tagsJSON) > 0 {
			json.Unmarshal(tagsJSON, &p.Tags)
		}
		if cover.Valid {
			p.CoverImage = cover.String
		}
		if custom.Valid {
			p.CustomURL = custom.String
		}
		posts = append(posts, p)
	}
	return posts, nil
}
func CreateNewPost(p *model.Post) error {
	p.Slug = time.Now().Format("2006-01-02-150405")
	if p.CustomURL != "" {
		p.Slug = p.CustomURL
	}
	tagsJSON, _ := json.Marshal(p.Tags)
	attJSON, _ := json.Marshal(p.Attachments)
	_, err := database.DB.Exec(`
        INSERT INTO posts (title, slug, content, tags, attachments, cover_image, custom_url)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `, p.Title, p.Slug, p.Content, tagsJSON, attJSON, p.CoverImage, p.CustomURL)
	return err
}

func DeletePost(id int) error {
	_, err := database.DB.Exec("DELETE FROM posts WHERE id = ?", id)
	return err
}
