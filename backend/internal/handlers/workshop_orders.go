package handlers

import (
	"net/http"

	"workshop/internal/httpapi"
)

// ListWorkshopOrders handles GET /api/workshop/orders.
//
// Skeleton stub: the workshop order list ticket implements this.
func ListWorkshopOrders(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, httpapi.CodeNotImplemented, "workshop order list #20 implements this")
}

// GetWorkshopOrder handles GET /api/workshop/orders/{order_number}.
//
// Skeleton stub: the workshop order list ticket implements this.
func GetWorkshopOrder(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, httpapi.CodeNotImplemented, "workshop order detail #20 implements this")
}
