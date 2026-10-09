package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/db"
	"workshop/internal/domain"
	"workshop/internal/store"
)

// newTestPool returns a pool whose connections see only a fresh, dedicated
// schema. The schema is created and dropped by the test, so the dashboard
// counts are unaffected by rows another test package writes into the public
// schema. It skips cleanly when no test database is configured.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set — skipping database-backed test")
	}

	schema := fmt.Sprintf("dashboard_store_%d", time.Now().UnixNano())
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

// seedCustomerAndVehicle creates the single customer and vehicle every seeded
// order references and returns their ids.
func seedCustomerAndVehicle(t *testing.T, pool *pgxpool.Pool) (customerID, vehicleID int64) {
	t.Helper()
	ctx := context.Background()
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Test Kunde", "kunde@example.test", "0000",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO vehicles (license_plate, brand, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		"B-TS 100", "Test", "Modell", 1000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}
	return customerID, vehicleID
}

// seedOrder inserts an order and, when fertigAt is set, a status event that
// moved it to "fertig" at that instant.
func seedOrder(t *testing.T, pool *pgxpool.Pool, customerID, vehicleID int64, number string, status domain.OrderStatus, fertigAt *time.Time) {
	t.Helper()
	ctx := context.Background()
	var orderID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, requested_date, problem_description)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		number, customerID, vehicleID, string(status), "2026-03-15", "Testproblem",
	).Scan(&orderID); err != nil {
		t.Fatalf("insert order %s: %v", number, err)
	}
	if fertigAt == nil {
		return
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO order_status_events (order_id, from_status, to_status, changed_by, changed_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		orderID, string(domain.StatusInArbeit), string(domain.StatusFertig), "tester", *fertigAt,
	); err != nil {
		t.Fatalf("insert status event for %s: %v", number, err)
	}
}

// seedInvoice inserts an invoice with the given gross amount issued at issuedAt.
func seedInvoice(t *testing.T, pool *pgxpool.Pool, orderID int64, number string, issuedAt time.Time, grossCents int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO invoices (invoice_number, order_id, issued_at, labor_cents, parts_cents, net_cents, vat_cents, gross_cents)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		number, orderID, issuedAt, 0, 0, grossCents, 0, grossCents,
	); err != nil {
		t.Fatalf("insert invoice %s: %v", number, err)
	}
}

func TestDashboardStatsEmptyDatabaseReturnsZeros(t *testing.T) {
	pool := newTestPool(t)
	st := store.New(pool)

	stats, err := st.DashboardStats(context.Background(), time.Date(2026, 3, 15, 12, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("DashboardStats on empty database: %v", err)
	}
	if stats.OpenOrders != 0 || stats.CompletedToday != 0 || stats.RevenueMonthCents != 0 {
		t.Fatalf("empty database stats = %+v, want all zeros", stats)
	}
}

func TestDashboardStatsCountsOpenCompletedAndRevenue(t *testing.T) {
	pool := newTestPool(t)
	st := store.New(pool)
	ctx := context.Background()
	customerID, vehicleID := seedCustomerAndVehicle(t, pool)

	// The reference instant and every seeded timestamp share the local
	// location, so the day and month boundaries are the same anywhere.
	loc := time.Local
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, loc)
	yesterday := time.Date(2026, 3, 14, 23, 30, 0, 0, loc)

	// Three orders are still open (neither "fertig" nor "abgeholt").
	seedOrder(t, pool, customerID, vehicleID, "O-100", domain.StatusAngefragt, nil)
	seedOrder(t, pool, customerID, vehicleID, "O-101", domain.StatusBestaetigt, nil)
	seedOrder(t, pool, customerID, vehicleID, "O-102", domain.StatusInArbeit, nil)

	// Reached "fertig" today.
	today9 := time.Date(2026, 3, 15, 9, 30, 0, 0, loc)
	seedOrder(t, pool, customerID, vehicleID, "O-103", domain.StatusFertig, &today9)
	// Reached "fertig" earlier today and was picked up since — still counts,
	// because the dashboard counts the reach of "fertig", not the current
	// status.
	today7 := time.Date(2026, 3, 15, 7, 0, 0, 0, loc)
	seedOrder(t, pool, customerID, vehicleID, "O-104", domain.StatusAbgeholt, &today7)
	// Reached "fertig" yesterday and last month — counted by neither figure.
	seedOrder(t, pool, customerID, vehicleID, "O-105", domain.StatusFertig, &yesterday)

	// Invoices: two issued this month, one last month.
	seedInvoice(t, pool, orderIDOf(t, pool, "O-103"), "R-1", time.Date(2026, 3, 10, 8, 0, 0, 0, loc), 11900)
	seedInvoice(t, pool, orderIDOf(t, pool, "O-104"), "R-2", time.Date(2026, 3, 31, 23, 59, 0, 0, loc), 5000)
	seedInvoice(t, pool, orderIDOf(t, pool, "O-105"), "R-3", time.Date(2026, 2, 20, 8, 0, 0, 0, loc), 9900)

	stats, err := st.DashboardStats(ctx, now)
	if err != nil {
		t.Fatalf("DashboardStats: %v", err)
	}
	if stats.OpenOrders != 3 {
		t.Errorf("open_orders = %d, want 3", stats.OpenOrders)
	}
	if stats.CompletedToday != 2 {
		t.Errorf("completed_today = %d, want 2", stats.CompletedToday)
	}
	if stats.RevenueMonthCents != 16900 {
		t.Errorf("revenue_month_cents = %d, want 16900", stats.RevenueMonthCents)
	}
}

// orderIDOf looks up the id of an order by its number.
func orderIDOf(t *testing.T, pool *pgxpool.Pool, number string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(),
		`SELECT id FROM orders WHERE order_number = $1`, number,
	).Scan(&id); err != nil {
		t.Fatalf("look up order %s: %v", number, err)
	}
	return id
}
