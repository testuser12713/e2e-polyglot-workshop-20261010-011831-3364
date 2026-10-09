package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"workshop/internal/auth"
	"workshop/internal/domain"
	"workshop/internal/httpapi"
	"workshop/internal/store"
)

// CreateOrderItem handles POST /api/workshop/orders/{order_number}/items. It
// records one labor or part position and answers 201 with the updated
// OrderDetail. A malformed body or invalid position is 400, an unknown order
// 404, a missing session 401.
func CreateOrderItem(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserFromContext(r.Context()); !ok {
		httpapi.WriteError(w, httpapi.CodeUnauthorized, "authentication required")
		return
	}

	req, message := decodeItemRequest(r)
	if message != "" {
		httpapi.WriteError(w, httpapi.CodeValidationError, message)
		return
	}

	detail, err := Store.AddOrderItem(r.Context(), r.PathValue("order_number"), req)
	if err != nil {
		writeItemError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, detail)
}

// UpdateOrderItem handles PUT /api/workshop/orders/{order_number}/items/{item_id}.
// It replaces the addressed position and answers 200 with the updated
// OrderDetail. Validation matches creation; an unknown order or item is 404
// and a missing session 401.
func UpdateOrderItem(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.UserFromContext(r.Context()); !ok {
		httpapi.WriteError(w, httpapi.CodeUnauthorized, "authentication required")
		return
	}

	itemID, err := strconv.Atoi(r.PathValue("item_id"))
	if err != nil || itemID <= 0 {
		httpapi.WriteError(w, httpapi.CodeValidationError, "invalid item id")
		return
	}

	req, message := decodeItemRequest(r)
	if message != "" {
		httpapi.WriteError(w, httpapi.CodeValidationError, message)
		return
	}

	detail, err := Store.UpdateOrderItem(r.Context(), r.PathValue("order_number"), itemID, req)
	if err != nil {
		writeItemError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, detail)
}

// decodeItemRequest decodes and validates the shared item body. It returns a
// non-empty message when the body is not a valid labor or part position.
func decodeItemRequest(r *http.Request) (domain.ItemRequest, string) {
	var req domain.ItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return domain.ItemRequest{}, "invalid request body"
	}
	if message := validateItemRequest(req); message != "" {
		return domain.ItemRequest{}, message
	}
	return req, ""
}

// validateItemRequest enforces the shared item contract: kind is "labor" or
// "part", the matching field (hours or quantity) is present and positive, and
// the unit price is present and positive in whole cents.
func validateItemRequest(req domain.ItemRequest) string {
	switch req.Kind {
	case "labor":
		if req.Hours == nil || *req.Hours <= 0 {
			return "hours must be present and positive for a labor item"
		}
	case "part":
		if req.Quantity == nil || *req.Quantity <= 0 {
			return "quantity must be present and positive for a part item"
		}
	default:
		return `kind must be "labor" or "part"`
	}
	if req.UnitPriceCents == nil || *req.UnitPriceCents <= 0 {
		return "unit_price_cents must be present and positive"
	}
	return ""
}

// writeItemError maps the items area's errors onto the uniform error body.
func writeItemError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrItemOrderNotFound):
		httpapi.WriteError(w, httpapi.CodeNotFound, "order not found")
	case errors.Is(err, store.ErrOrderItemNotFound):
		httpapi.WriteError(w, httpapi.CodeNotFound, "order item not found")
	case errors.Is(err, store.ErrOrderAbgeholt):
		httpapi.WriteError(w, httpapi.CodeInvalidTransition, "order is already picked up")
	default:
		httpapi.WriteError(w, httpapi.CodeInternalError, "internal server error")
	}
}
