package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter returns the API router.
func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request){
		w.Write([]byte("ok"))
	})

	r.Get("/posts", h.GetPosts)
	r.Get("/posts/{slug}", h.GetPostBySlug)
	r.Post("/posts", h.CreatePost)
	r.Put("/posts/{id}", h.UpdatePost)
	r.Delete("/posts/{id}", h.DeletePost)

	return r
}
