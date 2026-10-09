package store_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/domain"
	"workshop/internal/store"
	"workshop/internal/testsupport"
)

// uniqueToken returns a run of digits that is unique enough to keep this
// test's rows apart from any other row in a shared test database. It contains
// no letters, so a plate search for "AB" cannot match it by accident.
func uniqueToken(t *testing.T) string {
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

// seedOrder inserts a customer, a vehicle and an order and returns the order
// number together with its license plate and status.
func seedOrder(t *testing.T, pool *pgxpool.Pool, token, suffix string, status domain.OrderStatus, plate string, createdAt time.Time) (string, string) {
	t.Helper()
	ctx := context.Background()
	orderNumber := "WS-" + token + "-" + suffix

	var customerID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Testkunde "+suffix,
		"kunde-"+token+"-"+suffix+"@example.test",
		"0123456789",
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
	return orderNumber, plate
}

// assertOnlyThese fails when the orders number set, restricted to this test's
// token, differs from want (same members and same order).
func assertOnlyThese(t *testing.T, got []domain.OrderDetail, token string, want []string) {
	t.Helper()
	var mine []string
	for _, order := range got {
		if strings.Contains(order.OrderNumber, token) {
			mine = append(mine, order.OrderNumber)
		}
	}
	if len(mine) != len(want) {
		t.Fatalf("matched order numbers = %v, want %v", mine, want)
	}
	for i := range want {
		if mine[i] != want[i] {
			t.Fatalf("matched order numbers = %v, want %v", mine, want)
		}
	}
}

func TestListOrdersFiltersByStatusPlateAndCombination(t *testing.T) {
	pool := testsupport.NewPool(t)
	ctx := context.Background()
	applySchema(t, pool)
	st := store.New(pool)
	token := uniqueToken(t)
	base := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)

	first, _ := seedOrder(t, pool, token, "1", domain.StatusBestaetigt, "B-"+token+"-AB1", base)
	second, _ := seedOrder(t, pool, token, "2", domain.StatusBestaetigt, "B-"+token+"-XY2", base.Add(time.Minute))
	third, _ := seedOrder(t, pool, token, "3", domain.StatusAngefragt, "B-"+token+"-AB3", base.Add(2*time.Minute))

	confirmed := domain.StatusBestaetigt

	byStatus, err := st.ListOrders(ctx, store.OrderFilter{Status: &confirmed})
	if err != nil {
		t.Fatalf("ListOrders by status: %v", err)
	}
	assertOnlyThese(t, byStatus, token, []string{first, second})

	byPlate, err := st.ListOrders(ctx, store.OrderFilter{LicensePlate: "AB"})
	if err != nil {
		t.Fatalf("ListOrders by plate: %v", err)
	}
	assertOnlyThese(t, byPlate, token, []string{first, third})

	both, err := st.ListOrders(ctx, store.OrderFilter{Status: &confirmed, LicensePlate: "AB"})
	if err != nil {
		t.Fatalf("ListOrders by status and plate: %v", err)
	}
	assertOnlyThese(t, both, token, []string{first})
}

func TestOrderForWorkshopReturnsItemsAndHistory(t *testing.T) {
	pool := testsupport.NewPool(t)
	ctx := context.Background()
	applySchema(t, pool)
	st := store.New(pool)
	token := uniqueToken(t)
	orderNumber, plate := seedOrder(t, pool, token, "1", domain.StatusInArbeit, "B-"+token+"-AB1", time.Now().UTC())

	var orderID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM orders WHERE order_number = $1`, orderNumber).Scan(&orderID); err != nil {
		t.Fatalf("read order id: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO order_items (order_id, kind, description, hours, quantity, unit_price_cents, total_cents)
		VALUES ($1, 'labor', 'Bremsbeläge wechseln', 1.5, NULL, 9900, 14850),
		       ($1, 'part', 'Bremsscheibe', NULL, 2, 500, 1000)`, orderID); err != nil {
		t.Fatalf("seed items: %v", err)
	}

	detail, err := st.OrderForWorkshop(ctx, orderNumber)
	if err != nil {
		t.Fatalf("OrderForWorkshop: %v", err)
	}
	if detail.OrderNumber != orderNumber {
		t.Fatalf("order number = %q, want %q", detail.OrderNumber, orderNumber)
	}
	if detail.Status != domain.StatusInArbeit {
		t.Fatalf("status = %q, want %q", detail.Status, domain.StatusInArbeit)
	}
	if detail.Vehicle.LicensePlate != plate {
		t.Fatalf("plate = %q, want %q", detail.Vehicle.LicensePlate, plate)
	}
	if detail.Customer.Name == "" {
		t.Fatalf("customer name is empty")
	}
	if len(detail.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(detail.Items))
	}
	if detail.Items[0].Hours == nil || *detail.Items[0].Hours != 1.5 {
		t.Fatalf("labor hours = %v, want 1.5", detail.Items[0].Hours)
	}
	if detail.Items[1].Quantity == nil || *detail.Items[1].Quantity != 2 {
		t.Fatalf("part quantity = %v, want 2", detail.Items[1].Quantity)
	}
	if len(detail.History) != 1 {
		t.Fatalf("history = %d, want 1", len(detail.History))
	}
	if detail.History[0].FromStatus != nil {
		t.Fatalf("first history from_status = %v, want nil", detail.History[0].FromStatus)
	}
	if detail.History[0].ToStatus != domain.StatusInArbeit {
		t.Fatalf("history to_status = %q, want %q", detail.History[0].ToStatus, domain.StatusInArbeit)
	}
	if _, err := time.Parse(time.RFC3339, detail.History[0].ChangedAt); err != nil {
		t.Fatalf("changed_at %q is not RFC3339: %v", detail.History[0].ChangedAt, err)
	}
}

func TestOrderForWorkshopUnknownNumber(t *testing.T) {
	pool := testsupport.NewPool(t)
	ctx := context.Background()
	applySchema(t, pool)
	st := store.New(pool)

	_, err := st.OrderForWorkshop(ctx, "WS-"+uniqueToken(t)+"-nope")
	if !errors.Is(err, store.ErrOrderNotFound) {
		t.Fatalf("OrderForWorkshop unknown = %v, want ErrOrderNotFound", err)
	}
}
