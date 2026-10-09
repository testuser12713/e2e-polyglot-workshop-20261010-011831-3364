package auth

import "context"

// User is the authenticated workshop employee carried in a request context.
type User struct {
	ID    int
	Name  string
	Email string
}

type userContextKey struct{}

// WithUser returns a copy of ctx carrying u. Together with UserFromContext it
// lets every workshop handler be tested without going through a login.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userContextKey{}, u)
}

// UserFromContext returns the user stored by WithUser, if any.
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userContextKey{}).(User)
	return u, ok
}

// HashPassword returns the password hash to store for a workshop employee.
//
// Skeleton stub: the workshop login ticket implements this.
func HashPassword(password string) (string, error) {
	return "", nil
}

// CheckPassword reports whether password matches hash.
//
// Skeleton stub: the workshop login ticket implements this.
func CheckPassword(hash, password string) bool {
	return false
}

// IssueToken returns the signed session token for u.
//
// Skeleton stub: the workshop login ticket implements this.
func IssueToken(u User) (string, error) {
	return "", nil
}
