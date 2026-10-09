package handlers

import (
	"net/http"

	"workshop/internal/domain"
	"workshop/internal/httpapi"
)

// Health answers GET /api/health. It pings the database so a 200 proves the
// api can serve, not merely that a port is bound.
func Health(w http.ResponseWriter, r *http.Request) {
	if Store == nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "service not initialized")
		return
	}
	if err := Store.Ping(r.Context()); err != nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "database unavailable")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, domain.HealthResponse{Status: "ok"})
}
