package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/db"
	"workshop/internal/domain"
	"workshop/internal/store"
)

// testPool returns a pool to the PostgreSQL server the suite runs with. It
// prefers TEST_DATABASE_URL and falls back to DATABASE_URL (the variable the
// pipeline sets). When neither is present the test is skipped cleanly.
func testPool(t *testing.T) *pgxpool.Pool {
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
	// Two test binaries (store and handlers) may apply the schema at the same
	// moment; CREATE TABLE IF NOT EXISTS can race, so retry briefly.
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

// seedOrderWithStatus inserts a customer, a vehicle and an order with the given
// status and returns the order's id and number. Every fixture uses a unique
// suffix so parallel tests never touch each other's rows.
func seedOrderWithStatus(t *testing.T, pool *pgxpool.Pool, status domain.OrderStatus) (int, string) {
	t.Helper()
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	var customerID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Test Customer", "test-"+suffix+"@example.com", "0123456789",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	var vehicleID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO vehicles (license_plate, brand, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		"TEST-"+suffix, "Test", "Model", 1000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}

	orderNumber := "TEST-" + suffix
	var orderID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, requested_date, problem_description)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		orderNumber, customerID, vehicleID, string(status), time.Now(), "Test problem",
	).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}
	return orderID, orderNumber
}

func countEvents(t *testing.T, pool *pgxpool.Pool, orderID int) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM order_status_events WHERE order_id = $1`, orderID,
	).Scan(&n); err != nil {
		t.Fatalf("count events: %v", err)
	}
	return n
}

func currentStatus(t *testing.T, pool *pgxpool.Pool, orderID int) domain.OrderStatus {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM orders WHERE id = $1`, orderID,
	).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return domain.OrderStatus(status)
}

func TestApplyStatusTransitionWalksTheFullChain(t *testing.T) {
	pool := testPool(t)
	st := store.New(pool)
	orderID, orderNumber := seedOrderWithStatus(t, pool, domain.StatusAngefragt)

	steps := []domain.OrderStatus{
		domain.StatusBestaetigt,
		domain.StatusInArbeit,
		domain.StatusFertig,
		domain.StatusAbgeholt,
	}
	var detail *domain.OrderDetail
	for i, step := range steps {
		fresh, err := st.ApplyStatusTransition(context.Background(), orderNumber, step, "Anna Meier", nil)
		if err != nil {
			t.Fatalf("step %d (%s): unexpected error: %v", i, step, err)
		}
		detail = fresh
		if detail.Status != step {
			t.Fatalf("step %d: returned status %q, want %q", i, detail.Status, step)
		}
		if len(detail.History) != i+1 {
			t.Fatalf("step %d: history length %d, want %d", i, len(detail.History), i+1)
		}
	}

	wantHistory := []domain.OrderStatus{
		domain.StatusBestaetigt,
		domain.StatusInArbeit,
		domain.StatusFertig,
		domain.StatusAbgeholt,
	}
	for i, event := range detail.History {
		if event.ToStatus != wantHistory[i] {
			t.Errorf("history[%d].to_status = %q, want %q", i, event.ToStatus, wantHistory[i])
		}
		if event.ChangedBy != "Anna Meier" {
			t.Errorf("history[%d].changed_by = %q, want employee name %q", i, event.ChangedBy, "Anna Meier")
		}
		if event.ChangedAt == "" {
			t.Errorf("history[%d].changed_at is empty", i)
		}
		if i > 0 && detail.History[i-1].ToStatus != *event.FromStatus {
			t.Errorf("history[%d].from_status = %v, want previous to_status", i, event.FromStatus)
		}
	}

	if got := currentStatus(t, pool, orderID); got != domain.StatusAbgeholt {
		t.Errorf("persisted status = %q, want abgeholt", got)
	}
	if n := countEvents(t, pool, orderID); n != 4 {
		t.Errorf("event count = %d, want 4", n)
	}
}

func TestApplyStatusTransitionRejectsJumpsAndLeavesDatabaseUnchanged(t *testing.T) {
	pool := testPool(t)
	st := store.New(pool)

	jumps := []struct {
		name string
		from domain.OrderStatus
		to   domain.OrderStatus
	}{
		{"skip one", domain.StatusAngefragt, domain.StatusInArbeit},
		{"jump to end", domain.StatusAngefragt, domain.StatusAbgeholt},
		{"repeat", domain.StatusAngefragt, domain.StatusAngefragt},
		{"backwards", domain.StatusBestaetigt, domain.StatusAngefragt},
	}
	for _, tc := range jumps {
		t.Run(tc.name, func(t *testing.T) {
			orderID, orderNumber := seedOrderWithStatus(t, pool, tc.from)
			_, err := st.ApplyStatusTransition(context.Background(), orderNumber, tc.to, "Anna Meier", nil)
			if !errors.Is(err, store.ErrInvalidTransition) {
				t.Fatalf("transition %s -> %s: err = %v, want ErrInvalidTransition", tc.from, tc.to, err)
			}
			if got := currentStatus(t, pool, orderID); got != tc.from {
				t.Errorf("status changed to %q, want unchanged %q", got, tc.from)
			}
			if n := countEvents(t, pool, orderID); n != 0 {
				t.Errorf("recorded %d events for a rejected transition, want 0", n)
			}
		})
	}
}

func TestApplyStatusTransitionUnknownOrder(t *testing.T) {
	pool := testPool(t)
	st := store.New(pool)

	_, err := st.ApplyStatusTransition(context.Background(), "DOES-NOT-EXIST-"+fmt.Sprintf("%d", time.Now().UnixNano()), domain.StatusBestaetigt, "Anna Meier", nil)
	if !errors.Is(err, store.ErrOrderNotFound) {
		t.Fatalf("err = %v, want ErrOrderNotFound", err)
	}
}

func TestApplyStatusTransitionRollsBackWhenPublishFails(t *testing.T) {
	pool := testPool(t)
	st := store.New(pool)
	orderID, orderNumber := seedOrderWithStatus(t, pool, domain.StatusInArbeit)

	publish := func(ctx context.Context, orderID int, orderNumber string) error {
		return errors.New("queue unavailable")
	}
	_, err := st.ApplyStatusTransition(context.Background(), orderNumber, domain.StatusFertig, "Anna Meier", publish)
	if err == nil {
		t.Fatal("expected an error when the publisher fails")
	}
	if got := currentStatus(t, pool, orderID); got != domain.StatusInArbeit {
		t.Errorf("status = %q after failed publish, want in Arbeit (rolled back)", got)
	}
	if n := countEvents(t, pool, orderID); n != 0 {
		t.Errorf("recorded %d events despite rollback, want 0", n)
	}
}

func TestCanTransitionOnlyAllowsSingleSteps(t *testing.T) {
	allowed := [][2]domain.OrderStatus{
		{domain.StatusAngefragt, domain.StatusBestaetigt},
		{domain.StatusBestaetigt, domain.StatusInArbeit},
		{domain.StatusInArbeit, domain.StatusFertig},
		{domain.StatusFertig, domain.StatusAbgeholt},
	}
	for _, pair := range allowed {
		if !domain.CanTransition(pair[0], pair[1]) {
			t.Errorf("CanTransition(%q, %q) = false, want true", pair[0], pair[1])
		}
	}
	for _, from := range domain.OrderStatusOrder {
		for _, to := range domain.OrderStatusOrder {
			if from == to {
				continue
			}
			isAllowed := false
			for _, pair := range allowed {
				if pair[0] == from && pair[1] == to {
					isAllowed = true
					break
				}
			}
			if got := domain.CanTransition(from, to); got != isAllowed {
				t.Errorf("CanTransition(%q, %q) = %v, want %v", from, to, got, isAllowed)
			}
		}
	}
	if domain.CanTransition("kaputt", domain.StatusBestaetigt) {
		t.Error("CanTransition from an unknown status must be false")
	}
}
