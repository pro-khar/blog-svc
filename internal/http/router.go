package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter returns the API router.
func NewRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request){
		w.Write([]byte("ok"))
	})

	r.Get("/posts", GetPosts)
	r.Get("/posts/{slug}", GetPostBySlug)
	r.Post("/posts", CreatePost)
	r.Put("/posts/{id}", UpdatePost)
	r.Delete("/posts/{id}", DeletePost)

	return r
}
