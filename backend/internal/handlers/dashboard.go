package handlers

import (
	"net/http"
	"time"

	"workshop/internal/httpapi"
)

// GetWorkshopDashboard handles GET /api/workshop/dashboard. It answers the
// current open orders, the orders that reached "fertig" today and the gross
// revenue of the current month in cents. The route is guarded by RequireUser,
// so only a signed-in employee reaches this handler.
func GetWorkshopDashboard(w http.ResponseWriter, r *http.Request) {
	if Store == nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "service not initialized")
		return
	}

	stats, err := Store.DashboardStats(r.Context(), time.Now())
	if err != nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "dashboard unavailable")
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, stats)
}
