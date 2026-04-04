package httpapi

// TODO: add handlers for posts CRUD
import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func GetPosts(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("[]"))
}

func GetPostBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("post:" + slug))
}

func CreatePost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
}

func UpdatePost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func DeletePost(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}