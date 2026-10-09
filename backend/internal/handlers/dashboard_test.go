package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/auth"
	"workshop/internal/config"
	"workshop/internal/db"
	"workshop/internal/domain"
	"workshop/internal/handlers"
	"workshop/internal/queue"
	"workshop/internal/store"
)

// newIsolatedPool returns a pool scoped to a fresh, dedicated schema so the
// seeded orders and invoices are the only rows the dashboard handler can see.
func newIsolatedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set — skipping database-backed test")
	}

	schema := fmt.Sprintf("dashboard_handler_%d", time.Now().UnixNano())
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("parse test database url: %v", err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET search_path TO "+schema)
		return err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	if _, err := pool.Exec(context.Background(), "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create test schema: %v", err)
	}
	if err := db.Apply(context.Background(), pool); err != nil {
		pool.Close()
		t.Fatalf("apply schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		pool.Close()
	})
	return pool
}

func TestGetWorkshopDashboardReturnsFigures(t *testing.T) {
	pool := newIsolatedPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))
	ctx := context.Background()

	var customerID, vehicleID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Test Kunde", "kunde@example.test", "0000",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO vehicles (license_plate, brand, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		"B-HW 42", "Test", "Modell", 1000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}

	insertOrder := func(number string, status domain.OrderStatus) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(ctx,
			`INSERT INTO orders (order_number, customer_id, vehicle_id, status, requested_date, problem_description)
			 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			number, customerID, vehicleID, string(status), "2026-03-15", "Testproblem",
		).Scan(&id); err != nil {
			t.Fatalf("insert order %s: %v", number, err)
		}
		return id
	}

	insertOrder("O-200", domain.StatusAngefragt)
	insertOrder("O-201", domain.StatusInArbeit)
	finishedID := insertOrder("O-202", domain.StatusFertig)

	now := time.Now()
	if _, err := pool.Exec(ctx,
		`INSERT INTO order_status_events (order_id, from_status, to_status, changed_by, changed_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		finishedID, string(domain.StatusInArbeit), string(domain.StatusFertig), "tester", now,
	); err != nil {
		t.Fatalf("insert status event: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO invoices (invoice_number, order_id, issued_at, labor_cents, parts_cents, net_cents, vat_cents, gross_cents)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		"R-200", finishedID, now, 10000, 1900, 11900, 2261, 14161,
	); err != nil {
		t.Fatalf("insert invoice: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/workshop/dashboard", nil).
		WithContext(auth.WithUser(ctx, auth.User{ID: 1, Name: "Tester", Email: "tester@example.test"}))
	rr := httptest.NewRecorder()
	handlers.GetWorkshopDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/workshop/dashboard = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
	}
	var body domain.DashboardResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("dashboard body is not valid JSON: %v", err)
	}
	if body.OpenOrders != 2 {
		t.Errorf("open_orders = %d, want 2", body.OpenOrders)
	}
	if body.CompletedToday != 1 {
		t.Errorf("completed_today = %d, want 1", body.CompletedToday)
	}
	if body.RevenueMonthCents != 14161 {
		t.Errorf("revenue_month_cents = %d, want 14161", body.RevenueMonthCents)
	}
}
