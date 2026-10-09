package handlers_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	"workshop/internal/testsupport"
)

func workshopToken(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("read random bytes: %v", err)
	}
	var sb strings.Builder
	for _, x := range b {
		fmt.Fprintf(&sb, "%03d", int(x))
	}
	return sb.String()
}

func seedWorkshopOrder(t *testing.T, pool *pgxpool.Pool, token, suffix string, status domain.OrderStatus, plate string, createdAt time.Time) string {
	t.Helper()
	ctx := context.Background()
	orderNumber := "WS-" + token + "-" + suffix

	var customerID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Testkunde "+suffix, "kunde-"+token+"-"+suffix+"@example.test", "0123456789",
	).Scan(&customerID); err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	var vehicleID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO vehicles (license_plate, brand, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		plate, "VW", "Golf", 123456,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("seed vehicle: %v", err)
	}

	var orderID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO orders (order_number, customer_id, vehicle_id, status, requested_date, problem_description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		orderNumber, customerID, vehicleID, string(status),
		time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), "Bremsen quietschen", createdAt,
	).Scan(&orderID); err != nil {
		t.Fatalf("seed order: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO order_status_events (order_id, from_status, to_status, changed_by, changed_at)
		VALUES ($1, NULL, $2, $3, $4)`,
		orderID, string(status), "seed", createdAt,
	); err != nil {
		t.Fatalf("seed status event: %v", err)
	}
	return orderNumber
}

// schemaLockKey serializes schema creation across test binaries that share one
// test database. CREATE TABLE IF NOT EXISTS is not safe under concurrency.
const schemaLockKey = int64(832541120)

// applySchema creates the schema after taking a database-wide advisory lock, so
// two test packages running in parallel never race on the DDL.
func applySchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire schema lock connection: %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", schemaLockKey); err != nil {
		t.Fatalf("acquire schema lock: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", schemaLockKey)
	}()
	if err := db.Apply(ctx, pool); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
}

// initHandlers wires the shared dependencies against a fresh schema. It skips
// when no test database is available, like the rest of the suite.
func initHandlers(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testsupport.NewPool(t)
	applySchema(t, pool)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))
	return pool
}

func getAsUser(t *testing.T, path string, withUser bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if withUser {
		req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: 1, Name: "Tester", Email: "tester@example.test"}))
	}
	rr := httptest.NewRecorder()
	handlers.ListWorkshopOrders(rr, req)
	return rr
}

func decodeOrderList(t *testing.T, body []byte) []domain.OrderDetail {
	t.Helper()
	var parsed domain.OrderListResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body is not a valid order list: %v (body=%s)", err, string(body))
	}
	return parsed.Orders
}

func mineOnly(orders []domain.OrderDetail, token string) []string {
	var mine []string
	for _, order := range orders {
		if strings.Contains(order.OrderNumber, token) {
			mine = append(mine, order.OrderNumber)
		}
	}
	return mine
}

func TestListWorkshopOrdersFilters(t *testing.T) {
	pool := initHandlers(t)
	token := workshopToken(t)
	base := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)

	first := seedWorkshopOrder(t, pool, token, "1", domain.StatusBestaetigt, "B-"+token+"-AB1", base)
	second := seedWorkshopOrder(t, pool, token, "2", domain.StatusBestaetigt, "B-"+token+"-XY2", base.Add(time.Minute))
	third := seedWorkshopOrder(t, pool, token, "3", domain.StatusAngefragt, "B-"+token+"-AB3", base.Add(2*time.Minute))

	query := url.Values{}
	query.Set("status", string(domain.StatusBestaetigt))
	rr := getAsUser(t, "/api/workshop/orders?"+query.Encode(), true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status filter = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	if got := mineOnly(decodeOrderList(t, rr.Body.Bytes()), token); len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("status filter matched %v, want [%s %s]", got, first, second)
	}

	rr = getAsUser(t, "/api/workshop/orders?license_plate="+url.QueryEscape("AB"), true)
	if rr.Code != http.StatusOK {
		t.Fatalf("plate filter = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	if got := mineOnly(decodeOrderList(t, rr.Body.Bytes()), token); len(got) != 2 || got[0] != first || got[1] != third {
		t.Fatalf("plate filter matched %v, want [%s %s]", got, first, third)
	}

	query.Set("license_plate", "AB")
	rr = getAsUser(t, "/api/workshop/orders?"+query.Encode(), true)
	if rr.Code != http.StatusOK {
		t.Fatalf("combined filter = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	if got := mineOnly(decodeOrderList(t, rr.Body.Bytes()), token); len(got) != 1 || got[0] != first {
		t.Fatalf("combined filter matched %v, want [%s]", got, first)
	}
}

func TestListWorkshopOrdersRejectsUnknownStatus(t *testing.T) {
	initHandlers(t)
	rr := getAsUser(t, "/api/workshop/orders?status=voellig-unbekannt", true)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unknown status = %d, want 400 (body=%s)", rr.Code, rr.Body.String())
	}
	var body domain.ErrorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not valid JSON: %v", err)
	}
	if body.Error.Code != "validation_error" {
		t.Fatalf("error code = %q, want validation_error", body.Error.Code)
	}
}

func TestListWorkshopOrdersRequiresUser(t *testing.T) {
	initHandlers(t)
	rr := getAsUser(t, "/api/workshop/orders", false)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list = %d, want 401 (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestGetWorkshopOrderDetail(t *testing.T) {
	pool := initHandlers(t)
	token := workshopToken(t)
	orderNumber := seedWorkshopOrder(t, pool, token, "1", domain.StatusInArbeit, "B-"+token+"-AB1", time.Now().UTC())

	var orderID int64
	if err := pool.QueryRow(context.Background(), `SELECT id FROM orders WHERE order_number = $1`, orderNumber).Scan(&orderID); err != nil {
		t.Fatalf("read order id: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO order_items (order_id, kind, description, hours, quantity, unit_price_cents, total_cents)
		VALUES ($1, 'part', 'Bremsscheibe', NULL, 2, 500, 1000)`, orderID); err != nil {
		t.Fatalf("seed item: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/workshop/orders/"+orderNumber, nil)
	req.SetPathValue("order_number", orderNumber)
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: 1, Name: "Tester", Email: "tester@example.test"}))
	rr := httptest.NewRecorder()
	handlers.GetWorkshopOrder(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("detail = %d, want 200 (body=%s)", rr.Code, rr.Body.String())
	}
	var detail domain.OrderDetail
	if err := json.Unmarshal(rr.Body.Bytes(), &detail); err != nil {
		t.Fatalf("detail body is not valid JSON: %v", err)
	}
	if detail.OrderNumber != orderNumber {
		t.Fatalf("order_number = %q, want %q", detail.OrderNumber, orderNumber)
	}
	if detail.Status != domain.StatusInArbeit {
		t.Fatalf("status = %q, want %q", detail.Status, domain.StatusInArbeit)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(detail.Items))
	}
	if len(detail.History) != 1 {
		t.Fatalf("history = %d, want 1", len(detail.History))
	}
}

func TestGetWorkshopOrderUnknownNumberIsNotFound(t *testing.T) {
	initHandlers(t)
	missing := "WS-" + workshopToken(t) + "-missing"

	req := httptest.NewRequest(http.MethodGet, "/api/workshop/orders/"+missing, nil)
	req.SetPathValue("order_number", missing)
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: 1, Name: "Tester", Email: "tester@example.test"}))
	rr := httptest.NewRecorder()
	handlers.GetWorkshopOrder(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown order = %d, want 404 (body=%s)", rr.Code, rr.Body.String())
	}
	var body domain.ErrorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not valid JSON: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Fatalf("error code = %q, want not_found", body.Error.Code)
	}
}
