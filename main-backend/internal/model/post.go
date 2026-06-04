package model

import "time"

type Post struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"`
	Attachments []string  `json:"attachments"`
	CoverImage  string    `json:"cover_image"`
	CustomURL   string    `json:"custom_url"`
	CreatedAt   time.Time `json:"created_at"`
}
