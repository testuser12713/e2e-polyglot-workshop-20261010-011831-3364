package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"workshop/internal/domain"
	"workshop/internal/httpapi"
)

// dateLayout is the only accepted format for requested_date.
const dateLayout = "2006-01-02"

// CreateCustomerAppointment handles POST /api/customer/appointments.
//
// It validates the contract body, persists the customer and the vehicle (a
// known license plate reuses the existing vehicle), creates an order in status
// 'angefragt' and answers 201 with the created OrderDetail. A validation
// failure answers 400 with the uniform error body and one details entry per bad
// field.
func CreateCustomerAppointment(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAppointmentValidationError(w, nil, "invalid request body")
		return
	}

	if details := validateAppointment(req); len(details) > 0 {
		writeAppointmentValidationError(w, details, "validation failed")
		return
	}

	requestedDate, err := time.Parse(dateLayout, req.RequestedDate)
	if err != nil {
		// Reaching here is impossible after validation, but stay honest.
		writeAppointmentValidationError(w, []domain.FieldError{
			{Field: "requested_date", Message: "must be a date in YYYY-MM-DD format"},
		}, "validation failed")
		return
	}

	if Store == nil {
		writeAppointmentFailure(w, "store not initialized", "", nil)
		return
	}

	ctx := r.Context()

	customerID, err := Store.UpsertCustomer(ctx, req.Customer)
	if err != nil {
		writeAppointmentFailure(w, "persist customer", "", err)
		return
	}

	vehicleID, err := Store.GetOrCreateVehicleByPlate(ctx, req.Vehicle)
	if err != nil {
		writeAppointmentFailure(w, "persist vehicle", "", err)
		return
	}

	_, orderNumber, _, err := Store.CreateOrder(ctx, customerID, vehicleID, requestedDate, req.ProblemDescription)
	if err != nil {
		writeAppointmentFailure(w, "create order", "", err)
		return
	}

	detail, err := Store.ReadOrderDetail(ctx, orderNumber)
	if err != nil {
		writeAppointmentFailure(w, "read order", orderNumber, err)
		return
	}

	httpapi.WriteJSON(w, http.StatusCreated, detail)
}

// validateAppointment returns one error entry per invalid contract field. An
// empty result means the request is valid.
func validateAppointment(req domain.CreateAppointmentRequest) []domain.FieldError {
	var details []domain.FieldError

	if !validEmail(req.Customer.Email) {
		details = append(details, domain.FieldError{
			Field:   "customer.email",
			Message: "must be a valid e-mail address",
		})
	}
	if strings.TrimSpace(req.Vehicle.LicensePlate) == "" {
		details = append(details, domain.FieldError{
			Field:   "vehicle.license_plate",
			Message: "must not be empty",
		})
	}
	if _, err := time.Parse(dateLayout, req.RequestedDate); err != nil {
		details = append(details, domain.FieldError{
			Field:   "requested_date",
			Message: "must be a date in YYYY-MM-DD format",
		})
	}
	if req.Vehicle.Mileage < 0 {
		details = append(details, domain.FieldError{
			Field:   "vehicle.mileage",
			Message: "must be zero or greater",
		})
	}
	if strings.TrimSpace(req.ProblemDescription) == "" {
		details = append(details, domain.FieldError{
			Field:   "problem_description",
			Message: "must not be empty",
		})
	}

	return details
}

// validEmail reports whether value is a bare e-mail address (no display name).
func validEmail(value string) bool {
	addr, err := mail.ParseAddress(value)
	return err == nil && addr.Address == value
}

// writeAppointmentValidationError writes the uniform 400 body with the details
// entries for the rejected fields.
func writeAppointmentValidationError(w http.ResponseWriter, details []domain.FieldError, message string) {
	httpapi.WriteJSON(w, http.StatusBadRequest, domain.ErrorBody{
		Error: domain.ErrorDetail{
			Code:    httpapi.CodeValidationError,
			Message: message,
			Details: details,
		},
	})
}

// writeAppointmentFailure writes the uniform 500 body. It never logs a request
// value or a raw database error: a constraint violation can echo the license
// plate, so only the operation and the error's type are logged (AC-24). Once an
// order exists it is identified by its order number.
func writeAppointmentFailure(w http.ResponseWriter, operation, orderNumber string, err error) {
	if orderNumber != "" {
		log.Printf("create appointment: %s failed for order %s (error type %T)", operation, orderNumber, err)
	} else {
		log.Printf("create appointment: %s failed (error type %T)", operation, err)
	}
	httpapi.WriteError(w, httpapi.CodeInternalError, "internal server error")
}
