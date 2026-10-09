package handlers

import (
	"net/http"

	"workshop/internal/httpapi"
)

// CreateCustomerAppointment handles POST /api/customer/appointments.
//
// Skeleton stub: the customer appointment ticket implements this.
func CreateCustomerAppointment(w http.ResponseWriter, r *http.Request) {
	httpapi.WriteError(w, httpapi.CodeNotImplemented, "customer appointment #13 implements this")
}
