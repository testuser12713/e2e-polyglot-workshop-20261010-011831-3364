package handlers

import (
	"net/http"

	"workshop/internal/httpapi"
)

// WorkshopLogin handles POST /api/workshop/login.
//
// Skeleton stub: the workshop login ticket implements this.
func WorkshopLogin(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, httpapi.CodeNotImplemented, "workshop login #8 implements this")
}
