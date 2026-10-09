package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User is the authenticated workshop employee carried in a request context.
type User struct {
	ID    int
	Name  string
	Email string
}

type userContextKey struct{}

// tokenTTL is how long an issued session token stays valid.
const tokenTTL = 12 * time.Hour

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
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword reports whether password matches hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// tokenClaims is the payload carried by a session token.
type tokenClaims struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Exp   int64  `json:"exp"`
}

// tokenSecret reads the signing secret lazily from the environment. A missing
// secret is a startup defect of the api and is reported to whoever called.
func tokenSecret() ([]byte, error) {
	secret := os.Getenv("AUTH_TOKEN_SECRET")
	if secret == "" {
		return nil, errors.New("AUTH_TOKEN_SECRET is not set (see RUN.json)")
	}
	return []byte(secret), nil
}

// sign returns the base64url HMAC-SHA256 of the encoded payload.
func sign(secret []byte, encodedPayload string) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// IssueToken returns the signed session token for u. The token carries the
// employee id, name and e-mail and expires after tokenTTL.
func IssueToken(u User) (string, error) {
	secret, err := tokenSecret()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(tokenClaims{
		ID:    u.ID,
		Name:  u.Name,
		Email: u.Email,
		Exp:   time.Now().Add(tokenTTL).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("marshal token claims: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return encoded + "." + sign(secret, encoded), nil
}

// ParseToken verifies the signature and expiry of token and returns the user
// it carries. Any malformed, tampered or expired token yields an error.
func ParseToken(token string) (User, error) {
	secret, err := tokenSecret()
	if err != nil {
		return User{}, err
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return User{}, errors.New("malformed token")
	}
	encoded, signature := parts[0], parts[1]
	if !hmac.Equal([]byte(signature), []byte(sign(secret, encoded))) {
		return User{}, errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return User{}, errors.New("malformed token payload")
	}
	var claims tokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return User{}, errors.New("malformed token claims")
	}
	if time.Now().Unix() >= claims.Exp {
		return User{}, errors.New("token expired")
	}
	return User{ID: claims.ID, Name: claims.Name, Email: claims.Email}, nil
}
