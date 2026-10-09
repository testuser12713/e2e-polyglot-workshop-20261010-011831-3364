package handlers

import (
	"errors"
	"net/http"

	"workshop/internal/auth"
	"workshop/internal/domain"
	"workshop/internal/httpapi"
	"workshop/internal/store"
)

// ListWorkshopOrders handles GET /api/workshop/orders?status=&license_plate=.
// An unknown status value is a validation error (400); on success the response
// is the uniform {"orders":[OrderDetail]} body.
func ListWorkshopOrders(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserFromContext(r.Context()); !ok {
		httpapi.WriteError(w, httpapi.CodeUnauthorized, "authentication required")
		return
	}

	filter := store.OrderFilter{LicensePlate: r.URL.Query().Get("license_plate")}
	if raw := r.URL.Query().Get("status"); raw != "" {
		status := domain.OrderStatus(raw)
		if !isKnownStatus(status) {
			httpapi.WriteError(w, httpapi.CodeValidationError, "unknown status")
			return
		}
		filter.Status = &status
	}

	orders, err := Store.ListOrders(r.Context(), filter)
	if err != nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "could not load orders")
		return
	}
	if orders == nil {
		orders = []domain.OrderDetail{}
	}
	httpapi.WriteJSON(w, http.StatusOK, domain.OrderListResponse{Orders: orders})
}

// GetWorkshopOrder handles GET /api/workshop/orders/{order_number}. It answers
// the full OrderDetail or a not_found for an unknown number.
func GetWorkshopOrder(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserFromContext(r.Context()); !ok {
		httpapi.WriteError(w, httpapi.CodeUnauthorized, "authentication required")
		return
	}

	orderNumber := r.PathValue("order_number")
	detail, err := Store.OrderForWorkshop(r.Context(), orderNumber)
	if errors.Is(err, store.ErrOrderNotFound) {
		httpapi.WriteError(w, httpapi.CodeNotFound, "order not found")
		return
	}
	if err != nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "could not load order")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, detail)
}

// isKnownStatus reports whether status is one of the lifecycle states.
func isKnownStatus(status domain.OrderStatus) bool {
	for _, known := range domain.OrderStatusOrder {
		if status == known {
			return true
		}
	}
	return false
}
