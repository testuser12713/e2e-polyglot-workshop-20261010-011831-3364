package db

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// schemaSQL is applied at startup so a freshly started api works immediately
// against an empty database. Every statement is idempotent (IF NOT EXISTS).
//
//go:embed schema.sql
var schemaSQL string

// Open creates a pgx connection pool for the given PostgreSQL URL.
func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Apply runs the schema against the pool. The schema only contains
// CREATE TABLE IF NOT EXISTS statements, so applying it repeatedly is safe.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}
