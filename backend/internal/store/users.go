package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Employee is a workshop account as stored, including the password hash.
type Employee struct {
	ID           int
	Name         string
	Email        string
	PasswordHash string
}

// seedEmployeeName is the display name of the workshop admin created at
// startup from configuration; the configuration only names the e-mail.
const seedEmployeeName = "Administrator"

// EnsureSeedEmployee creates the workshop admin employee from configuration if
// no employee with that e-mail exists yet. The password is only ever passed as
// its hash; the plaintext never reaches this layer.
func (s *Store) EnsureSeedEmployee(ctx context.Context, email, passwordHash string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO employees (name, email, password_hash)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (email) DO NOTHING`,
		seedEmployeeName, email, passwordHash)
	if err != nil {
		return fmt.Errorf("ensure seed employee: %w", err)
	}
	return nil
}

// FindEmployeeByEmail returns the employee with the given e-mail, or nil when
// no such employee exists. The password hash is included so the login handler
// can compare it.
func (s *Store) FindEmployeeByEmail(ctx context.Context, email string) (*Employee, error) {
	var e Employee
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, email, password_hash FROM employees WHERE email = $1`,
		email).Scan(&e.ID, &e.Name, &e.Email, &e.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find employee by email: %w", err)
	}
	return &e, nil
}
