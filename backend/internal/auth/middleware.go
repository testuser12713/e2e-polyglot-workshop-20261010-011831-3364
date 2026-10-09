package auth

import (
	"net/http"
	"strings"

	"workshop/internal/httpapi"
)

const bearerPrefix = "Bearer "

// RequireUser wraps a workshop handler so it is only reachable with a valid
// session. A missing or invalid bearer token answers the uniform 401 body and
// never invokes the wrapped handler, so no data is changed. On success the
// authenticated user is put into the request context, where handlers read it
// with UserFromContext.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, bearerPrefix) {
			httpapi.WriteError(w, httpapi.CodeUnauthorized, "authentication required")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
		user, err := ParseToken(token)
		if err != nil {
			httpapi.WriteError(w, httpapi.CodeUnauthorized, "invalid session")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
	})
}
