package httpapi

// TODO: add handlers for posts CRUD
import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pro-khar/blog-svc/internal/model"
)

type Handler struct{
	DB *pgxpool.Pool
}

func (h *Handler) GetPosts(w http.ResponseWriter, r *http.Request) {
	posts := []model.Post{
		{
            ID:          1,
            Title:       "Hello World",
            Slug:        "hello-world",
            Excerpt:     "My first post",
            ContentMD:   "# Hello\nThis is content.",
            Tags:        []string{"go", "blog"},
            IsPublished: true,
            CreatedAt:   time.Now(),
            UpdatedAt:   time.Now(),
        },
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(posts)
}

func (h *Handler) GetPostBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("post:" + slug))
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