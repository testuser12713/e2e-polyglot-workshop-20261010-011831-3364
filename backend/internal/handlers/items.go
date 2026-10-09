package handlers

import (
	"net/http"

	"workshop/internal/httpapi"
)

// CreateOrderItem handles POST /api/workshop/orders/{order_number}/items.
//
// Skeleton stub: the order items ticket implements this.
func CreateOrderItem(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, httpapi.CodeNotImplemented, "order items #11 implements this")
}

// UpdateOrderItem handles PUT /api/workshop/orders/{order_number}/items/{item_id}.
//
// Skeleton stub: the order items ticket implements this.
func UpdateOrderItem(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, httpapi.CodeNotImplemented, "order items #11 implements this")
}
