package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"workshop/internal/config"
	"workshop/internal/db"
	"workshop/internal/domain"
	"workshop/internal/handlers"
	"workshop/internal/queue"
	"workshop/internal/store"
	"workshop/internal/testsupport"

	"github.com/jackc/pgx/v5/pgxpool"
)

// newAppointmentStack wires the handler dependencies against the real Postgres
// pool and applies the schema, so the handler exercises the same path as the
// running api.
func newAppointmentStack(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testsupport.NewPool(t)
	applySchema(t, pool)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))
	return pool
}

// applySchema creates the schema under a Postgres advisory lock so concurrent
// test packages cannot race over the catalog while creating the same tables.
func applySchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection for schema lock: %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", schemaLockKey); err != nil {
		t.Fatalf("acquire schema lock: %v", err)
	}
	defer func() { _, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", schemaLockKey) }()
	if err := db.Apply(ctx, pool); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
}

const schemaLockKey = 7711301

func uniquePlate() string {
	return fmt.Sprintf("TT-%d", time.Now().UnixNano())
}

func validAppointmentBody(plate string) domain.CreateAppointmentRequest {
	return domain.CreateAppointmentRequest{
		Customer: domain.Customer{
			Name:  "Erika Mustermann",
			Email: "erika@example.com",
			Phone: "+49 30 123456",
		},
		Vehicle: domain.Vehicle{
			LicensePlate: plate,
			Brand:        "VW",
			Model:        "Golf",
			Mileage:      120000,
		},
		RequestedDate:      "2026-11-03",
		ProblemDescription: "Bremsen quietschen beim Anhalten.",
	}
}

func postAppointment(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/customer/appointments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handlers.CreateCustomerAppointment(rr, req)
	return rr
}

func TestCreateCustomerAppointmentReturnsCreatedOrder(t *testing.T) {
	newAppointmentStack(t)
	plate := uniquePlate()

	raw, err := json.Marshal(validAppointmentBody(plate))
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	rr := postAppointment(t, string(raw))
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/customer/appointments = %d, want 201 (body=%q)", rr.Code, rr.Body.String())
	}

	var detail domain.OrderDetail
	if err := json.Unmarshal(rr.Body.Bytes(), &detail); err != nil {
		t.Fatalf("created body is not valid OrderDetail JSON: %v (body=%q)", err, rr.Body.String())
	}
	if detail.OrderNumber == "" {
		t.Fatalf("created order has no order_number")
	}
	if detail.Status != domain.StatusAngefragt {
		t.Fatalf("created status = %q, want %q", detail.Status, domain.StatusAngefragt)
	}
	if detail.Vehicle.LicensePlate != plate {
		t.Fatalf("created vehicle plate = %q, want %q", detail.Vehicle.LicensePlate, plate)
	}
	if detail.RequestedDate != "2026-11-03" {
		t.Fatalf("requested_date = %q, want 2026-11-03", detail.RequestedDate)
	}
	if detail.Customer.Email != "erika@example.com" {
		t.Fatalf("customer email = %q, want erika@example.com", detail.Customer.Email)
	}
}

func TestCreateCustomerAppointmentReusesKnownVehicle(t *testing.T) {
	pool := newAppointmentStack(t)
	plate := uniquePlate()

	first, _ := json.Marshal(validAppointmentBody(plate))
	if rr := postAppointment(t, string(first)); rr.Code != http.StatusCreated {
		t.Fatalf("first POST = %d, want 201 (body=%q)", rr.Code, rr.Body.String())
	}

	second := validAppointmentBody(plate)
	second.Customer.Email = "max@example.com"
	second.Customer.Name = "Max Beispiel"
	raw, _ := json.Marshal(second)
	if rr := postAppointment(t, string(raw)); rr.Code != http.StatusCreated {
		t.Fatalf("second POST = %d, want 201 (body=%q)", rr.Code, rr.Body.String())
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM vehicles WHERE license_plate = $1`, plate,
	).Scan(&count); err != nil {
		t.Fatalf("count vehicles: %v", err)
	}
	if count != 1 {
		t.Fatalf("vehicles with plate %q = %d, want exactly 1", plate, count)
	}
}

func TestCreateCustomerAppointmentValidation(t *testing.T) {
	newAppointmentStack(t)
	plate := uniquePlate()

	cases := []struct {
		name          string
		mutate        func(*domain.CreateAppointmentRequest)
		expectedField string
	}{
		{
			name:          "invalid e-mail",
			mutate:        func(r *domain.CreateAppointmentRequest) { r.Customer.Email = "not-an-email" },
			expectedField: "customer.email",
		},
		{
			name:          "empty license plate",
			mutate:        func(r *domain.CreateAppointmentRequest) { r.Vehicle.LicensePlate = "  " },
			expectedField: "vehicle.license_plate",
		},
		{
			name:          "bad requested date",
			mutate:        func(r *domain.CreateAppointmentRequest) { r.RequestedDate = "03.11.2026" },
			expectedField: "requested_date",
		},
		{
			name:          "negative mileage",
			mutate:        func(r *domain.CreateAppointmentRequest) { r.Vehicle.Mileage = -1 },
			expectedField: "vehicle.mileage",
		},
		{
			name:          "empty problem description",
			mutate:        func(r *domain.CreateAppointmentRequest) { r.ProblemDescription = "   " },
			expectedField: "problem_description",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validAppointmentBody(plate)
			tc.mutate(&req)
			raw, _ := json.Marshal(req)

			rr := postAppointment(t, string(raw))
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("POST = %d, want 400 (body=%q)", rr.Code, rr.Body.String())
			}

			var body domain.ErrorBody
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatalf("error body is not valid JSON: %v (body=%q)", err, rr.Body.String())
			}
			if body.Error.Code != "validation_error" {
				t.Fatalf("error code = %q, want validation_error", body.Error.Code)
			}
			if !containsField(body.Error.Details, tc.expectedField) {
				t.Fatalf("details %+v do not name field %q", body.Error.Details, tc.expectedField)
			}
		})
	}
}

func TestCreateCustomerAppointmentRejectsMalformedJSON(t *testing.T) {
	newAppointmentStack(t)

	rr := postAppointment(t, "{not json")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("malformed body = %d, want 400", rr.Code)
	}
	var body domain.ErrorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not valid JSON: %v", err)
	}
	if body.Error.Code != "validation_error" {
		t.Fatalf("error code = %q, want validation_error", body.Error.Code)
	}
}

func containsField(details []domain.FieldError, field string) bool {
	for _, d := range details {
		if d.Field == field {
			return true
		}
	}
	return false
}
