package auth

import "net/http"

// RateLimit wraps the login handler to limit attempts per client. The skeleton
// stub passes every request straight through; the rate limiting ticket
// implements the real limit.
func RateLimit(next http.Handler) http.Handler {
	return next
}
