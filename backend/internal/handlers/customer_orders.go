package handlers

import (
	"errors"
	"net/http"

	"workshop/internal/httpapi"
	"workshop/internal/store"
)

// GetCustomerOrder handles GET /api/customer/orders/{order_number}?license_plate=.
// It is public: the order number together with the vehicle's license plate is
// the customer's access proof. A wrong plate and an unknown number both answer
// the same 404 so the endpoint never confirms which order numbers exist.
func GetCustomerOrder(w http.ResponseWriter, r *http.Request) {
	if Store == nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "service not initialized")
		return
	}

	orderNumber := r.PathValue("order_number")
	licensePlate := r.URL.Query().Get("license_plate")
	ctx := r.Context()

	orderID, detail, err := Store.OrderForCustomer(ctx, orderNumber, licensePlate)
	if errors.Is(err, store.ErrNotFound) {
		httpapi.WriteError(w, httpapi.CodeNotFound, "order not found")
		return
	}
	if err != nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "internal server error")
		return
	}

	history, err := Store.OrderHistory(ctx, orderID)
	if err != nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "internal server error")
		return
	}
	detail.History = history

	httpapi.WriteJSON(w, http.StatusOK, detail)
}

// GetCustomerInvoice handles
// GET /api/customer/orders/{order_number}/invoice?license_plate=. It is public
// on the same proof as the order lookup. The order is resolved first, so a
// wrong plate answers 404 exactly as it does for the order; an order without an
// invoice yet answers 404 as well.
func GetCustomerInvoice(w http.ResponseWriter, r *http.Request) {
	if Store == nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "service not initialized")
		return
	}

	orderNumber := r.PathValue("order_number")
	licensePlate := r.URL.Query().Get("license_plate")
	ctx := r.Context()

	orderID, _, err := Store.OrderForCustomer(ctx, orderNumber, licensePlate)
	if errors.Is(err, store.ErrNotFound) {
		httpapi.WriteError(w, httpapi.CodeNotFound, "invoice not found")
		return
	}
	if err != nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "internal server error")
		return
	}

	invoice, err := Store.InvoiceForOrder(ctx, orderID)
	if err != nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "internal server error")
		return
	}
	if invoice == nil {
		httpapi.WriteError(w, httpapi.CodeNotFound, "invoice not found")
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, invoice)
}
