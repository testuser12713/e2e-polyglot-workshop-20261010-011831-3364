package httpapi

import (
	"encoding/json"
	"net/http"

	"workshop/internal/domain"
)

// Error codes of the uniform error body. Every failure of the api is reported
// through one of these; the code decides the HTTP status.
const (
	CodeValidationError   = "validation_error"
	CodeUnauthorized      = "unauthorized"
	CodeNotFound          = "not_found"
	CodeInvalidTransition = "invalid_transition"
	CodeRateLimited       = "rate_limited"
	CodeNotImplemented    = "not_implemented"
	CodeInternalError     = "internal_error"
	CodeMethodNotAllowed  = "method_not_allowed"
)

// statusByCode maps an error code to its HTTP status.
var statusByCode = map[string]int{
	CodeValidationError:   http.StatusBadRequest,
	CodeUnauthorized:      http.StatusUnauthorized,
	CodeNotFound:          http.StatusNotFound,
	CodeInvalidTransition: http.StatusConflict,
	CodeRateLimited:       http.StatusTooManyRequests,
	CodeNotImplemented:    http.StatusNotImplemented,
	CodeInternalError:     http.StatusInternalServerError,
	CodeMethodNotAllowed:  http.StatusMethodNotAllowed,
}

// StatusForCode returns the HTTP status for an error code. An unknown code is
// an internal defect and is reported as 500.
func StatusForCode(code string) int {
	if status, ok := statusByCode[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// WriteJSON writes v with the given status as JSON.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes the uniform error body for the given code. The status
// comes from the code map, so every failure is shaped the same way and carries
// no internals.
func WriteError(w http.ResponseWriter, code, message string) {
	WriteJSON(w, StatusForCode(code), domain.ErrorBody{
		Error: domain.ErrorDetail{Code: code, Message: message},
	})
}
