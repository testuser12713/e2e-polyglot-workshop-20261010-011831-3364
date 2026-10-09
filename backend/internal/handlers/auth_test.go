package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"workshop/internal/auth"
	"workshop/internal/config"
	"workshop/internal/db"
	"workshop/internal/domain"
	"workshop/internal/handlers"
	"workshop/internal/queue"
	"workshop/internal/store"
	"workshop/internal/testsupport"
)

// newAuthStore gives the test a store on a schema it created itself and wires
// the shared handler dependencies. Only the employees table is reset, because
// that is the table this ticket owns.
func newAuthStore(t *testing.T) *store.Store {
	t.Helper()
	pool := testsupport.NewPool(t)
	ctx := context.Background()
	if err := db.Apply(ctx, pool); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM employees"); err != nil {
		t.Fatalf("reset employees: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM employees")
	})
	st := store.New(pool)
	handlers.Init(st, &config.Config{}, queue.New(""))
	return st
}

func uniqueEmail() string {
	return fmt.Sprintf("admin-%d@werkstatt.example", time.Now().UnixNano())
}

func postLogin(t *testing.T, email, password string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	req := httptest.NewRequest(http.MethodPost, "/api/workshop/login", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	handlers.WorkshopLogin(rr, req)
	return rr
}

func TestEnsureSeedEmployeeIsCreatedAndIdempotent(t *testing.T) {
	st := newAuthStore(t)
	ctx := context.Background()
	email := uniqueEmail()

	hash, err := auth.HashPassword("configured-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := st.EnsureSeedEmployee(ctx, email, hash); err != nil {
		t.Fatalf("EnsureSeedEmployee: %v", err)
	}
	// A second startup must not create a second employee.
	if err := st.EnsureSeedEmployee(ctx, email, hash); err != nil {
		t.Fatalf("EnsureSeedEmployee second call: %v", err)
	}

	employee, err := st.FindEmployeeByEmail(ctx, email)
	if err != nil {
		t.Fatalf("FindEmployeeByEmail: %v", err)
	}
	if employee == nil {
		t.Fatal("seeded employee not found after EnsureSeedEmployee")
	}
	if employee.Email != email {
		t.Fatalf("employee email = %q, want %q", employee.Email, email)
	}
	if !auth.CheckPassword(employee.PasswordHash, "configured-password") {
		t.Fatal("stored hash does not verify the configured password")
	}

	missing, err := st.FindEmployeeByEmail(ctx, uniqueEmail())
	if err != nil {
		t.Fatalf("FindEmployeeByEmail for unknown: %v", err)
	}
	if missing != nil {
		t.Fatal("FindEmployeeByEmail returned an employee for an unknown e-mail")
	}

	var count int
	if err := st.Pool().QueryRow(ctx, "SELECT count(*) FROM employees WHERE email = $1", email).Scan(&count); err != nil {
		t.Fatalf("count employees: %v", err)
	}
	if count != 1 {
		t.Fatalf("employee rows = %d, want 1", count)
	}
}

func TestWorkshopLoginWithCorrectCredentialsReturnsToken(t *testing.T) {
	st := newAuthStore(t)
	t.Setenv("AUTH_TOKEN_SECRET", "test-secret")
	ctx := context.Background()
	email := uniqueEmail()

	hash, err := auth.HashPassword("configured-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := st.EnsureSeedEmployee(ctx, email, hash); err != nil {
		t.Fatalf("EnsureSeedEmployee: %v", err)
	}

	rr := postLogin(t, email, "configured-password")
	if rr.Code != http.StatusOK {
		t.Fatalf("login = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
	}
	var resp domain.LoginResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("login body is not valid JSON: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("login returned an empty token")
	}
	if resp.Employee.Email != email {
		t.Fatalf("employee email = %q, want %q", resp.Employee.Email, email)
	}
	if _, err := auth.ParseToken(resp.Token); err != nil {
		t.Fatalf("issued token does not parse: %v", err)
	}
	// AC-25: the password never appears in a response body.
	if strings.Contains(rr.Body.String(), "configured-password") {
		t.Fatal("login response body contains the password")
	}
}

func TestWorkshopLoginRejectsWrongPasswordAndUnknownEmailAlike(t *testing.T) {
	st := newAuthStore(t)
	t.Setenv("AUTH_TOKEN_SECRET", "test-secret")
	ctx := context.Background()
	email := uniqueEmail()

	hash, err := auth.HashPassword("configured-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := st.EnsureSeedEmployee(ctx, email, hash); err != nil {
		t.Fatalf("EnsureSeedEmployee: %v", err)
	}

	wrong := postLogin(t, email, "wrong-password")
	unknown := postLogin(t, uniqueEmail(), "wrong-password")

	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password = %d, want 401 (body=%q)", wrong.Code, wrong.Body.String())
	}
	if unknown.Code != http.StatusUnauthorized {
		t.Fatalf("unknown email = %d, want 401 (body=%q)", unknown.Code, unknown.Body.String())
	}
	if wrong.Body.String() != unknown.Body.String() {
		t.Fatalf("wrong password and unknown email must answer the same body: %q vs %q",
			wrong.Body.String(), unknown.Body.String())
	}
	if !strings.Contains(wrong.Body.String(), `"unauthorized"`) {
		t.Fatalf("body = %q, want the uniform unauthorized error body", wrong.Body.String())
	}
}

func TestProtectedRouteWithoutTokenIsUnauthorizedAndChangesNothing(t *testing.T) {
	st := newAuthStore(t)
	t.Setenv("AUTH_TOKEN_SECRET", "test-secret")
	ctx := context.Background()

	email := uniqueEmail()
	hash, err := auth.HashPassword("configured-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := st.EnsureSeedEmployee(ctx, email, hash); err != nil {
		t.Fatalf("EnsureSeedEmployee: %v", err)
	}

	ran := false
	protected := auth.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ran = true
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/workshop/orders", nil)
	protected.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated protected route = %d, want 401", rr.Code)
	}
	if ran {
		t.Fatal("protected handler ran without a token")
	}

	var count int
	if err := st.Pool().QueryRow(ctx, "SELECT count(*) FROM employees WHERE email = $1", email).Scan(&count); err != nil {
		t.Fatalf("count employees: %v", err)
	}
	if count != 1 {
		t.Fatalf("employee rows = %d, want 1 (unauthenticated request changed data)", count)
	}
}
