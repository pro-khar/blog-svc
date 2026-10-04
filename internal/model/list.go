package model

import "time"

// List represents a blog post list item.
type List struct {
	ID          int `json:"id"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Excerpt     string `json:"excerpt"` 
	Tags        []string `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	PublishedAt *time.Time `json:"published_at"`
	IsPublished bool `json:"is_published"`
}
