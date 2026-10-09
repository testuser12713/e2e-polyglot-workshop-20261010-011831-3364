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

// itemsPool returns a pool to the PostgreSQL server the suite runs with. It
// prefers TEST_DATABASE_URL and falls back to DATABASE_URL (the variable the
// pipeline sets). When neither is present the test is skipped cleanly.
func itemsPool(t *testing.T) *pgxpool.Pool {
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

// seedOrderForItems inserts a customer, a vehicle and an order with the given
// status and returns the order's id and number. A unique suffix keeps parallel
// tests off each other's rows.
func seedOrderForItems(t *testing.T, pool *pgxpool.Pool, status domain.OrderStatus) (int, string) {
	t.Helper()
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	var customerID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Item Test Customer", "items-"+suffix+"@example.com", "0123456789",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	var vehicleID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO vehicles (license_plate, brand, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		"ITEMS-"+suffix, "Test", "Model", 1000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}

	orderNumber := "ITEMS-" + suffix
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

func floatPtr(v float64) *float64 { return &v }
func intPtr(v int) *int           { return &v }

func TestAddOrderItemComputesLaborAndPartTotals(t *testing.T) {
	pool := itemsPool(t)
	st := store.New(pool)
	_, orderNumber := seedOrderForItems(t, pool, domain.StatusBestaetigt)
	ctx := context.Background()

	labor := domain.ItemRequest{
		Kind:           "labor",
		Description:    "Bremsen wechseln",
		Hours:          floatPtr(1.5),
		UnitPriceCents: intPtr(8000),
	}
	detail, err := st.AddOrderItem(ctx, orderNumber, labor)
	if err != nil {
		t.Fatalf("add labor item: %v", err)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("after labor item: %d items, want 1", len(detail.Items))
	}
	got := detail.Items[0]
	if got.Kind != "labor" || got.TotalCents != 12000 {
		t.Fatalf("labor item = %+v, want kind labor total 12000", got)
	}
	if got.Hours == nil || *got.Hours != 1.5 {
		t.Fatalf("labor item hours = %v, want 1.5", got.Hours)
	}
	if got.Quantity != nil {
		t.Fatalf("labor item quantity = %v, want nil", *got.Quantity)
	}

	part := domain.ItemRequest{
		Kind:           "part",
		Description:    "Bremsbelag",
		Quantity:       intPtr(3),
		UnitPriceCents: intPtr(1500),
	}
	detail, err = st.AddOrderItem(ctx, orderNumber, part)
	if err != nil {
		t.Fatalf("add part item: %v", err)
	}
	if len(detail.Items) != 2 {
		t.Fatalf("after part item: %d items, want 2", len(detail.Items))
	}
	gotPart := detail.Items[1]
	if gotPart.Kind != "part" || gotPart.TotalCents != 4500 {
		t.Fatalf("part item = %+v, want kind part total 4500", gotPart)
	}
	if gotPart.Quantity == nil || *gotPart.Quantity != 3 {
		t.Fatalf("part item quantity = %v, want 3", gotPart.Quantity)
	}
}

func TestUpdateOrderItemRecomputesTotal(t *testing.T) {
	pool := itemsPool(t)
	st := store.New(pool)
	_, orderNumber := seedOrderForItems(t, pool, domain.StatusInArbeit)
	ctx := context.Background()

	created, err := st.AddOrderItem(ctx, orderNumber, domain.ItemRequest{
		Kind:           "part",
		Description:    "Ölfilter",
		Quantity:       intPtr(2),
		UnitPriceCents: intPtr(1000),
	})
	if err != nil {
		t.Fatalf("add part item: %v", err)
	}
	itemID := created.Items[0].ID
	if created.Items[0].TotalCents != 2000 {
		t.Fatalf("created total = %d, want 2000", created.Items[0].TotalCents)
	}

	updated, err := st.UpdateOrderItem(ctx, orderNumber, itemID, domain.ItemRequest{
		Kind:           "part",
		Description:    "Ölfilter",
		Quantity:       intPtr(5),
		UnitPriceCents: intPtr(1000),
	})
	if err != nil {
		t.Fatalf("update part item: %v", err)
	}
	if len(updated.Items) != 1 {
		t.Fatalf("after update: %d items, want 1", len(updated.Items))
	}
	if updated.Items[0].ID != itemID {
		t.Fatalf("updated item id = %d, want %d", updated.Items[0].ID, itemID)
	}
	if updated.Items[0].TotalCents != 5000 {
		t.Fatalf("updated total = %d, want 5000", updated.Items[0].TotalCents)
	}
	if updated.Items[0].Quantity == nil || *updated.Items[0].Quantity != 5 {
		t.Fatalf("updated quantity = %v, want 5", updated.Items[0].Quantity)
	}
}

func TestAddAndUpdateRejectPickedUpOrder(t *testing.T) {
	pool := itemsPool(t)
	st := store.New(pool)
	_, orderNumber := seedOrderForItems(t, pool, domain.StatusAbgeholt)
	ctx := context.Background()

	_, err := st.AddOrderItem(ctx, orderNumber, domain.ItemRequest{
		Kind:           "labor",
		Description:    "Nacharbeit",
		Hours:          floatPtr(1),
		UnitPriceCents: intPtr(8000),
	})
	if !errors.Is(err, store.ErrOrderAbgeholt) {
		t.Fatalf("add to abgeholt order: err = %v, want ErrOrderAbgeholt", err)
	}

	_, err = st.UpdateOrderItem(ctx, orderNumber, 1, domain.ItemRequest{
		Kind:           "labor",
		Description:    "Nacharbeit",
		Hours:          floatPtr(1),
		UnitPriceCents: intPtr(8000),
	})
	if !errors.Is(err, store.ErrOrderAbgeholt) {
		t.Fatalf("update on abgeholt order: err = %v, want ErrOrderAbgeholt", err)
	}
}

func TestAddOrderItemUnknownOrder(t *testing.T) {
	pool := itemsPool(t)
	st := store.New(pool)

	_, err := st.AddOrderItem(context.Background(), "DOES-NOT-EXIST", domain.ItemRequest{
		Kind:           "part",
		Description:    "x",
		Quantity:       intPtr(1),
		UnitPriceCents: intPtr(100),
	})
	if !errors.Is(err, store.ErrItemOrderNotFound) {
		t.Fatalf("err = %v, want ErrItemOrderNotFound", err)
	}
}

func TestUpdateOrderItemUnknownItem(t *testing.T) {
	pool := itemsPool(t)
	st := store.New(pool)
	_, orderNumber := seedOrderForItems(t, pool, domain.StatusBestaetigt)

	_, err := st.UpdateOrderItem(context.Background(), orderNumber, 999999999, domain.ItemRequest{
		Kind:           "part",
		Description:    "x",
		Quantity:       intPtr(1),
		UnitPriceCents: intPtr(100),
	})
	if !errors.Is(err, store.ErrOrderItemNotFound) {
		t.Fatalf("err = %v, want ErrOrderItemNotFound", err)
	}
}
