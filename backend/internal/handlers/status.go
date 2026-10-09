package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"workshop/internal/auth"
	"workshop/internal/domain"
	"workshop/internal/httpapi"
	"workshop/internal/store"
)

// UpdateOrderStatus handles POST /api/workshop/orders/{order_number}/status.
// It moves the order one step along the lifecycle, records the change with the
// employee and the timestamp and returns the fresh OrderDetail. An unknown
// status value is 400, an unknown order 404, a transition the lifecycle forbids
// 409, and a missing session 401. Every rejection leaves the database
// unchanged.
func UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpapi.WriteError(w, httpapi.CodeUnauthorized, "authentication required")
		return
	}

	var req domain.UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, httpapi.CodeValidationError, "invalid request body")
		return
	}
	if !domain.IsKnownStatus(req.Status) {
		httpapi.WriteError(w, httpapi.CodeValidationError, "unknown status")
		return
	}

	orderNumber := r.PathValue("order_number")

	var publish store.PublishOrderCompletedFunc
	if req.Status == domain.StatusFertig {
		publish = func(ctx context.Context, orderID int, number string) error {
			return Queue.PublishOrderCompleted(ctx, orderID, number)
		}
	}

	detail, err := Store.ApplyStatusTransition(r.Context(), orderNumber, req.Status, user.Name, publish)
	switch {
	case errors.Is(err, store.ErrOrderNotFound):
		httpapi.WriteError(w, httpapi.CodeNotFound, "order not found")
	case errors.Is(err, store.ErrInvalidTransition):
		httpapi.WriteError(w, httpapi.CodeInvalidTransition, "invalid status transition")
	case err != nil:
		httpapi.WriteError(w, httpapi.CodeInternalError, "internal server error")
	default:
		httpapi.WriteJSON(w, http.StatusOK, detail)
	}
}
