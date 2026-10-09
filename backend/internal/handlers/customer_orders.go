package handlers

import (
	"net/http"

	"workshop/internal/httpapi"
)

// GetCustomerOrder handles GET /api/customer/orders/{order_number}.
//
// Skeleton stub: the customer order status ticket implements this.
func GetCustomerOrder(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, httpapi.CodeNotImplemented, "customer order lookup #10 implements this")
}

// GetCustomerInvoice handles GET /api/customer/orders/{order_number}/invoice.
//
// Skeleton stub: the customer order status ticket implements this.
func GetCustomerInvoice(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, httpapi.CodeNotImplemented, "customer invoice lookup #10 implements this")
}
