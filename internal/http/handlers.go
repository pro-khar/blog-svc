package httpapi

// TODO: add handlers for posts CRUD
import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pro-khar/blog-svc/internal/model"
)

type Handler struct {
	DB *pgxpool.Pool
}

func (h *Handler) GetPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), "SELECT id, title, slug, excerpt, tags, created_at, updated_at, is_published, published_at FROM posts WHERE is_published = true ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	result := []model.List{} //{} is important to initialize slice with zero values ( Frontend : null.map() vs [].map())
	for rows.Next() {
		var post model.List
		err := rows.Scan(&post.ID, &post.Title, &post.Slug, &post.Excerpt, &post.Tags, &post.CreatedAt, &post.UpdatedAt, &post.IsPublished, &post.PublishedAt)
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

func (h *Handler) GetPostsInternal(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), "SELECT id, title, slug, excerpt, tags, created_at, updated_at, is_published, published_at FROM posts ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	result := []model.List{} //{} is important to initialize slice with zero values ( Frontend : null.map() vs [].map())
	for rows.Next() {
		var post model.List
		err := rows.Scan(&post.ID, &post.Title, &post.Slug, &post.Excerpt, &post.Tags, &post.CreatedAt, &post.UpdatedAt, &post.IsPublished, &post.PublishedAt)
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
	row := h.DB.QueryRow(r.Context(), "SELECT id, title, slug, excerpt, content_md, tags, created_at, updated_at, is_published FROM posts WHERE slug = $1 AND is_published = true", slug)
	var post model.Post
	err := row.Scan(&post.ID, &post.Title, &post.Slug, &post.Excerpt, &post.ContentMD, &post.Tags, &post.CreatedAt, &post.UpdatedAt, &post.IsPublished)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(post)
}

func (h *Handler) CreatePost(w http.ResponseWriter, r *http.Request) {
	var post model.Post
	err := json.NewDecoder(r.Body).Decode(&post)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	slugHasSpace := regexp.MustCompile(`\s`).MatchString(post.Slug)
	slugHasCapital := regexp.MustCompile(`[A-Z]`).MatchString(post.Slug)
	if strings.TrimSpace(post.Slug) == "" {
		http.Error(w, "Slug cannot be empty", http.StatusUnprocessableEntity)
		return
	} else if slugHasSpace || slugHasCapital {
		http.Error(w, "Malformed Slug, please watch for spaces and capitals", http.StatusUnprocessableEntity)
		return
	}
	if strings.TrimSpace(post.Title) == "" {
		http.Error(w, "Title cannot be empty", http.StatusUnprocessableEntity)
		return
	}
	if strings.TrimSpace(post.ContentMD) == "" {
		http.Error(w, "Content cannot be empty", http.StatusUnprocessableEntity)
		return
	}
	if post.Tags == nil {
		post.Tags = []string{}
	}

	row := h.DB.QueryRow(r.Context(),
		`	INSERT INTO posts (title, slug, excerpt, content_md, tags)
		VALUES($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`,
		post.Title, post.Slug, post.Excerpt, post.ContentMD, post.Tags)

	err = row.Scan(&post.ID, &post.CreatedAt, &post.UpdatedAt)
	var pgErr *pgconn.PgError
	if err != nil {
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				http.Error(w, "Slug already exists", http.StatusConflict)
				return
			}
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(post)
}

func (h *Handler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi((chi.URLParam(r, "id")))
	if err != nil {
		http.Error(w, "Only integer ids allowed!", http.StatusBadRequest)
		return
	}

	var post model.Post
	err = json.NewDecoder(r.Body).Decode(&post)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	slugHasSpace := regexp.MustCompile(`\s`).MatchString(post.Slug)
	slugHasCapital := regexp.MustCompile(`[A-Z]`).MatchString(post.Slug)
	if strings.TrimSpace(post.Slug) == "" {
		http.Error(w, "Slug cannot be empty", http.StatusUnprocessableEntity)
		return
	} else if slugHasSpace || slugHasCapital {
		http.Error(w, "Malformed Slug, please watch for spaces and capitals", http.StatusUnprocessableEntity)
		return
	}
	if strings.TrimSpace(post.Title) == "" {
		http.Error(w, "Title cannot be empty", http.StatusUnprocessableEntity)
		return
	}
	if strings.TrimSpace(post.ContentMD) == "" {
		http.Error(w, "Content cannot be empty", http.StatusUnprocessableEntity)
		return
	}
	if post.Tags == nil {
		post.Tags = []string{}
	}

	row := h.DB.QueryRow(r.Context(),
		`UPDATE posts 
		SET title = $1, slug = $2, excerpt = $3, content_md = $4, tags = $5, updated_at = now()
		WHERE id = $6
		RETURNING id, created_at, updated_at, published_at, is_published`,
		post.Title, post.Slug, post.Excerpt, post.ContentMD, post.Tags, id)

	err = row.Scan(&post.ID, &post.CreatedAt, &post.UpdatedAt, &post.PublishedAt, &post.IsPublished)
	var pgErr *pgconn.PgError
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if errors.As(err, &pgErr) {
			if pgErr.Code == "23505" {
				http.Error(w, "Slug already exists", http.StatusConflict)
				return
			}
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(post)
}

func (h *Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Only Integer ids allowed!", http.StatusBadRequest)
		return
	}

	row, err := h.DB.Exec(r.Context(),
		`DELETE FROM posts
	WHERE id = $1
	`,
		id)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if row.RowsAffected() == 0 {
		http.Error(w, "Nothing to delete", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PublishPost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Malformed id", http.StatusBadRequest)
		return
	}

	var post model.List
	row := h.DB.QueryRow(r.Context(), `UPDATE posts SET is_published = true, published_at = COALESCE(published_at, now()) WHERE id = $1 RETURNING id, title, created_at, is_published, published_at`, id)
	err = row.Scan(&post.ID, &post.Title, &post.CreatedAt, &post.IsPublished, &post.PublishedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "post not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(post)
}

func (h *Handler) UnPublishPost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "Malformed id", http.StatusBadRequest)
		return
	}

	var post model.List
	row := h.DB.QueryRow(r.Context(), `UPDATE posts SET is_published = false, published_at = NULL WHERE id = $1 RETURNING id, title, created_at, is_published, published_at`, id)
	err = row.Scan(&post.ID, &post.Title, &post.CreatedAt, &post.IsPublished, &post.PublishedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "post not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(post)
}
