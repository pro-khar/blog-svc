package httpapi

// TODO: add handlers for posts CRUD
import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pro-khar/blog-svc/internal/model"
)

type Handler struct {
	DB *pgxpool.Pool
}

func (h *Handler) GetPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), "SELECT id, title, slug, excerpt, tags, created_at, updated_at, is_published FROM posts WHERE is_published = true ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	result := []model.Post{} //{} is important to initialize slice with zero values ( Frontend : null.map() vs [].map())
	for rows.Next() {
		var post model.Post
		err := rows.Scan(&post.ID, &post.Title, &post.Slug, &post.Excerpt, &post.Tags, &post.CreatedAt, &post.UpdatedAt, &post.IsPublished)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		result = append(result, post)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) GetPostBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	row := h.DB.QueryRow(r.Context(), "SELECT id, title, slug, excerpt, tags, created_at, updated_at, is_published FROM posts WHERE slug = $1 AND is_published = true", slug)
	var post model.Post
	err := row.Scan(&post.ID, &post.Title, &post.Slug, &post.Excerpt, &post.Tags, &post.CreatedAt, &post.UpdatedAt, &post.IsPublished)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(post)
}

func (h *Handler) CreatePost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}
