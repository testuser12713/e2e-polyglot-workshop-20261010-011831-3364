package auth

import (
	"net/http"

	"workshop/internal/httpapi"
)

// RequireUser wraps a workshop handler so it is only reachable with a valid
// session. Until the workshop login ticket implements it, it answers the
// uniform 501 body for every request.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteError(w, httpapi.CodeNotImplemented, "workshop login #8 implements this")
	})
}
