package handlers

import (
	"net/http"

	"workshop/internal/httpapi"
)

// GetWorkshopDashboard handles GET /api/workshop/dashboard.
//
// Skeleton stub: the workshop dashboard ticket implements this.
func GetWorkshopDashboard(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, httpapi.CodeNotImplemented, "workshop dashboard #17 implements this")
}
