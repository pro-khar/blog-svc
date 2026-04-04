package model

// Post represents a blog post.
type Post struct {
	ID          string
	Title       string
	Slug        string
	Excerpt     string
	ContentMD   string
	Tags        string
	CreatedAt   string
	UpdatedAt   string
	PublishedAt string
	IsPublished bool
}
