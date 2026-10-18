package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/pro-khar/blog-svc/internal/db"
	httpapi "github.com/pro-khar/blog-svc/internal/http"
)

// yahan se main server start hota hai
func main() {
	godotenv.Load()

	ctx := context.Background()

	//Read database URL from env
	dbURL := os.Getenv("DATABASE_URL")
	adminKey := os.Getenv("ADMIN_KEY")

	//kill the program if DATABASE_URL is not set
	if dbURL == "" {
		slog.Error("DATABASE_URL is not set")
		os.Exit(1)
	}
	if adminKey == "" {
		slog.Error("ADMIN_KEY is missing")
		os.Exit(1)
	}

	//Now initialize the database connection pool
	pool, err := db.NewPool(ctx, dbURL)

	//kill the program if we fail to connect to database
	if err != nil {
		slog.Error("Failed to connect to Database")
		os.Exit(1)
	}

	//defer closing the pool when main function exits
	defer pool.Close()

	slog.Info("Database connected")

	h := &httpapi.Handler{DB: pool} //jo pool uppar banaya tha usko isme pel diya
	//Now start the http server by initializing the router
	router := httpapi.NewRouter(h, adminKey)
	slog.Info("Server running on :8080")

	//listen and serve on port 8080, kill the program if server fails to start
	if err := http.ListenAndServe(":8080", router); err != nil {
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}
