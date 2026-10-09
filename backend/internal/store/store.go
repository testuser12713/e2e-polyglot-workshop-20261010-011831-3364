package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the data access layer of the api. Its area methods live in
// store/*.go; every SQL statement is parametrized.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Pool exposes the underlying connection pool to the store's own area files.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// Ping reports whether the database is reachable.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// EnsureSeedEmployee and FindEmployeeByEmail live in store/users.go.
