package handlers

import (
	"net/http"

	"workshop/internal/httpapi"
)

// UpdateOrderStatus handles POST /api/workshop/orders/{order_number}/status.
//
// Skeleton stub: the order status lifecycle ticket implements this.
func UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, httpapi.CodeNotImplemented, "order status #7 implements this")
}
