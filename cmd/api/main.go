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

//yahan se main server start hota hai
func main() {
	godotenv.Load()

	ctx := context.Background()

	//Read database URL from env
	dsn := os.Getenv("DATABASE_URL")

	//kill the program if DATABASE_URL is not set
	if dsn == ""{
		slog.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	//Now initialize the database connection pool
	pool, err := db.NewPool(ctx, dsn)

	//kill the program if we fail to connect to database
	if err != nil{
		slog.Error("Failed to connect to Database")
		os.Exit(1)
	}

	//defer closing the pool when main function exits
	defer pool.Close()

	slog.Info("Database connected")

	//Now start the http server by initializing the router
	router := httpapi.NewRouter()
	slog.Info("Server running on :8080")

	//listen and serve on port 8080, kill the program if server fails to start
	if err := http.ListenAndServe(":8080", router); err != nil{
		slog.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}

