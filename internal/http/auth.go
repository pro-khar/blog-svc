package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func RequireAdmin(adminKey string) func(http.Handler) http.Handler {
	if strings.TrimSpace(adminKey) == "" {
		panic("RequireAdmin(): key found empty. check ADMIN_KEY")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			clientToken, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || subtle.ConstantTimeCompare([]byte(clientToken), []byte(adminKey)) != 1 {
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
