package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Iska kaam hai connection pool bana ke return karna, aur test karna
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return pool, nil
}
