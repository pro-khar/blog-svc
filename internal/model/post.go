package model

import (
	"encoding/json"
	"time"
)

// Post represents a blog post.
type Post struct {
	ID          int             `json:"id"`
	Title       string          `json:"title"`
	Slug        string          `json:"slug"`
	Excerpt     string          `json:"excerpt"`
	Content     json.RawMessage `json:"content"`
	Tags        []string        `json:"tags"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	PublishedAt *time.Time      `json:"published_at"`
	IsPublished bool            `json:"is_published"`
}
