package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"workshop/internal/auth"
	"workshop/internal/domain"
	"workshop/internal/httpapi"
)

// WorkshopLogin handles POST /api/workshop/login. It looks up the employee by
// e-mail and compares the stored password hash. An unknown e-mail and a wrong
// password both answer the SAME 401 body, so the response never reveals which
// part was wrong. A successful login returns a signed token and the employee.
func WorkshopLogin(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, httpapi.CodeValidationError, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		httpapi.WriteError(w, httpapi.CodeUnauthorized, "invalid credentials")
		return
	}

	employee, err := Store.FindEmployeeByEmail(r.Context(), req.Email)
	if err != nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "internal server error")
		return
	}
	if employee == nil || !auth.CheckPassword(employee.PasswordHash, req.Password) {
		httpapi.WriteError(w, httpapi.CodeUnauthorized, "invalid credentials")
		return
	}

	token, err := auth.IssueToken(auth.User{
		ID:    employee.ID,
		Name:  employee.Name,
		Email: employee.Email,
	})
	if err != nil {
		httpapi.WriteError(w, httpapi.CodeInternalError, "internal server error")
		return
	}

	// Log line carries the employee id only — never the e-mail or password.
	log.Printf("workshop login: employee %d authenticated", employee.ID)

	httpapi.WriteJSON(w, http.StatusOK, domain.LoginResponse{
		Token: token,
		Employee: domain.Employee{
			ID:    employee.ID,
			Name:  employee.Name,
			Email: employee.Email,
		},
	})
}
