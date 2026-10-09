package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/auth"
	"workshop/internal/config"
	"workshop/internal/db"
	"workshop/internal/domain"
	"workshop/internal/handlers"
	"workshop/internal/queue"
	"workshop/internal/store"
)

// itemsHandlerPool returns a pool to the PostgreSQL server the suite runs with
// and applies the schema. It prefers TEST_DATABASE_URL and falls back to
// DATABASE_URL; without either the test is skipped cleanly.
func itemsHandlerPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("no TEST_DATABASE_URL/DATABASE_URL — skipping database-backed test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	t.Cleanup(pool.Close)
	var applyErr error
	for attempt := 0; attempt < 5; attempt++ {
		if applyErr = db.Apply(context.Background(), pool); applyErr == nil {
			break
		}
		time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
	}
	if applyErr != nil {
		t.Fatalf("apply schema: %v", applyErr)
	}
	return pool
}

// itemsHandlerOrder seeds one order for the handler tests and returns its
// number. A unique suffix keeps parallel tests off each other's rows.
func itemsHandlerOrder(t *testing.T, pool *pgxpool.Pool, status domain.OrderStatus) string {
	t.Helper()
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	var customerID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Handler Test Customer", "hitems-"+suffix+"@example.com", "0123456789",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	var vehicleID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO vehicles (license_plate, brand, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		"HITEMS-"+suffix, "Test", "Model", 1000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}
	orderNumber := "HITEMS-" + suffix
	if _, err := pool.Exec(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, requested_date, problem_description)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		orderNumber, customerID, vehicleID, string(status), time.Now(), "Test problem",
	); err != nil {
		t.Fatalf("insert order: %v", err)
	}
	return orderNumber
}

// authedItemRequest builds a request carrying the workshop employee, so the
// handler is exercised without going through a login.
func authedItemRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	return req.WithContext(auth.WithUser(req.Context(), auth.User{ID: 1, Name: "Anna Meier", Email: "anna@example.com"}))
}

func decodeOrderDetail(t *testing.T, rr *httptest.ResponseRecorder) domain.OrderDetail {
	t.Helper()
	var detail domain.OrderDetail
	if err := json.Unmarshal(rr.Body.Bytes(), &detail); err != nil {
		t.Fatalf("response is not a valid OrderDetail: %v (body=%q)", err, rr.Body.String())
	}
	return detail
}

func decodeErrorCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not valid JSON: %v (body=%q)", err, rr.Body.String())
	}
	return body.Error.Code
}

func TestCreateOrderItemCreatesLaborAndPart(t *testing.T) {
	pool := itemsHandlerPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))

	orderNumber := itemsHandlerOrder(t, pool, domain.StatusBestaetigt)

	rr := httptest.NewRecorder()
	req := authedItemRequest(http.MethodPost, "/api/workshop/orders/"+orderNumber+"/items",
		`{"kind":"labor","description":"Bremsen wechseln","hours":1.5,"unit_price_cents":8000}`)
	req.SetPathValue("order_number", orderNumber)
	handlers.CreateOrderItem(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("POST labor item = %d, want 201 (body=%q)", rr.Code, rr.Body.String())
	}
	detail := decodeOrderDetail(t, rr)
	if len(detail.Items) != 1 || detail.Items[0].Kind != "labor" || detail.Items[0].TotalCents != 12000 {
		t.Fatalf("returned detail = %+v, want one labor item with total 12000", detail.Items)
	}

	rr = httptest.NewRecorder()
	req = authedItemRequest(http.MethodPost, "/api/workshop/orders/"+orderNumber+"/items",
		`{"kind":"part","description":"Bremsbelag","quantity":3,"unit_price_cents":1500}`)
	req.SetPathValue("order_number", orderNumber)
	handlers.CreateOrderItem(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("POST part item = %d, want 201 (body=%q)", rr.Code, rr.Body.String())
	}
	detail = decodeOrderDetail(t, rr)
	if len(detail.Items) != 2 {
		t.Fatalf("returned detail has %d items, want 2", len(detail.Items))
	}
	if detail.Items[1].Kind != "part" || detail.Items[1].TotalCents != 4500 {
		t.Fatalf("part item = %+v, want kind part total 4500", detail.Items[1])
	}
}

func TestUpdateOrderItemChangesQuantityAndTotal(t *testing.T) {
	pool := itemsHandlerPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))

	orderNumber := itemsHandlerOrder(t, pool, domain.StatusInArbeit)

	rr := httptest.NewRecorder()
	req := authedItemRequest(http.MethodPost, "/api/workshop/orders/"+orderNumber+"/items",
		`{"kind":"part","description":"Ölfilter","quantity":2,"unit_price_cents":1000}`)
	req.SetPathValue("order_number", orderNumber)
	handlers.CreateOrderItem(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST item = %d, want 201 (body=%q)", rr.Code, rr.Body.String())
	}
	created := decodeOrderDetail(t, rr)
	itemID := created.Items[0].ID
	itemIDText := fmt.Sprintf("%d", itemID)

	rr = httptest.NewRecorder()
	req = authedItemRequest(http.MethodPut, "/api/workshop/orders/"+orderNumber+"/items/"+itemIDText,
		`{"kind":"part","description":"Ölfilter","quantity":5,"unit_price_cents":1000}`)
	req.SetPathValue("order_number", orderNumber)
	req.SetPathValue("item_id", itemIDText)
	handlers.UpdateOrderItem(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("PUT item = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
	}
	updated := decodeOrderDetail(t, rr)
	if len(updated.Items) != 1 {
		t.Fatalf("after update: %d items, want 1", len(updated.Items))
	}
	if updated.Items[0].ID != itemID || updated.Items[0].TotalCents != 5000 {
		t.Fatalf("updated item = %+v, want id %d total 5000", updated.Items[0], itemID)
	}
}

func TestCreateOrderItemValidationErrors(t *testing.T) {
	pool := itemsHandlerPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))
	orderNumber := itemsHandlerOrder(t, pool, domain.StatusBestaetigt)

	cases := []struct {
		name string
		body string
	}{
		{"unknown kind", `{"kind":"material","description":"x","quantity":1,"unit_price_cents":100}`},
		{"labor missing hours", `{"kind":"labor","description":"x","unit_price_cents":100}`},
		{"labor zero hours", `{"kind":"labor","description":"x","hours":0,"unit_price_cents":100}`},
		{"part missing quantity", `{"kind":"part","description":"x","unit_price_cents":100}`},
		{"part negative quantity", `{"kind":"part","description":"x","quantity":-1,"unit_price_cents":100}`},
		{"missing unit price", `{"kind":"part","description":"x","quantity":1}`},
		{"negative unit price", `{"kind":"part","description":"x","quantity":1,"unit_price_cents":-5}`},
		{"malformed body", `{`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := authedItemRequest(http.MethodPost, "/api/workshop/orders/"+orderNumber+"/items", tc.body)
			req.SetPathValue("order_number", orderNumber)
			handlers.CreateOrderItem(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%q)", rr.Code, rr.Body.String())
			}
			if code := decodeErrorCode(t, rr); code != "validation_error" {
				t.Fatalf("error code = %q, want validation_error", code)
			}
		})
	}
}

func TestOrderItemHandlersRequireUser(t *testing.T) {
	pool := itemsHandlerPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))
	orderNumber := itemsHandlerOrder(t, pool, domain.StatusBestaetigt)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/workshop/orders/"+orderNumber+"/items",
		strings.NewReader(`{"kind":"part","description":"x","quantity":1,"unit_price_cents":100}`))
	req.SetPathValue("order_number", orderNumber)
	handlers.CreateOrderItem(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("POST without user = %d, want 401", rr.Code)
	}
	if code := decodeErrorCode(t, rr); code != "unauthorized" {
		t.Fatalf("error code = %q, want unauthorized", code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/workshop/orders/"+orderNumber+"/items/1",
		strings.NewReader(`{"kind":"part","description":"x","quantity":1,"unit_price_cents":100}`))
	req.SetPathValue("order_number", orderNumber)
	req.SetPathValue("item_id", "1")
	handlers.UpdateOrderItem(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("PUT without user = %d, want 401", rr.Code)
	}
}

func TestOrderItemHandlersNotFound(t *testing.T) {
	pool := itemsHandlerPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))

	rr := httptest.NewRecorder()
	req := authedItemRequest(http.MethodPost, "/api/workshop/orders/DOES-NOT-EXIST/items",
		`{"kind":"part","description":"x","quantity":1,"unit_price_cents":100}`)
	req.SetPathValue("order_number", "DOES-NOT-EXIST")
	handlers.CreateOrderItem(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("POST unknown order = %d, want 404 (body=%q)", rr.Code, rr.Body.String())
	}

	orderNumber := itemsHandlerOrder(t, pool, domain.StatusBestaetigt)
	rr = httptest.NewRecorder()
	req = authedItemRequest(http.MethodPut, "/api/workshop/orders/"+orderNumber+"/items/999999999",
		`{"kind":"part","description":"x","quantity":1,"unit_price_cents":100}`)
	req.SetPathValue("order_number", orderNumber)
	req.SetPathValue("item_id", "999999999")
	handlers.UpdateOrderItem(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("PUT unknown item = %d, want 404 (body=%q)", rr.Code, rr.Body.String())
	}
}
