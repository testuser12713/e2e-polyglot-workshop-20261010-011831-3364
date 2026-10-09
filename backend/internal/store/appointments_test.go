package store_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/db"
	"workshop/internal/domain"
	"workshop/internal/store"
	"workshop/internal/testsupport"
)

func newStore(t *testing.T) (*store.Store, *pgxpool.Pool) {
	t.Helper()
	pool := testsupport.NewPool(t)
	applySchema(t, pool)
	return store.New(pool), pool
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

func uniqueValue(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestUpsertCustomerReusesKnownEmail(t *testing.T) {
	st, _ := newStore(t)
	ctx := context.Background()
	email := uniqueValue("customer") + "@example.com"

	first, err := st.UpsertCustomer(ctx, domain.Customer{Name: "Erika", Email: email, Phone: "1"})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	second, err := st.UpsertCustomer(ctx, domain.Customer{Name: "Erika Neu", Email: email, Phone: "2"})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if first != second {
		t.Fatalf("customer id changed: %d -> %d, want the same row", first, second)
	}
}

func TestGetOrCreateVehicleByPlateCreatesOneRow(t *testing.T) {
	st, pool := newStore(t)
	ctx := context.Background()
	vehicle := domain.Vehicle{
		LicensePlate: uniqueValue("VV"),
		Brand:        "VW",
		Model:        "Golf",
		Mileage:      1000,
	}

	first, err := st.GetOrCreateVehicleByPlate(ctx, vehicle)
	if err != nil {
		t.Fatalf("first get-or-create: %v", err)
	}
	second, err := st.GetOrCreateVehicleByPlate(ctx, vehicle)
	if err != nil {
		t.Fatalf("second get-or-create: %v", err)
	}
	if first != second {
		t.Fatalf("vehicle id changed: %d -> %d, want the existing row", first, second)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM vehicles WHERE license_plate = $1`, vehicle.LicensePlate,
	).Scan(&count); err != nil {
		t.Fatalf("count vehicles: %v", err)
	}
	if count != 1 {
		t.Fatalf("vehicle rows for plate = %d, want exactly 1", count)
	}
}

func TestCreateOrderAndReadOrderDetail(t *testing.T) {
	st, _ := newStore(t)
	ctx := context.Background()

	customerID, err := st.UpsertCustomer(ctx, domain.Customer{
		Name: "Erika Mustermann", Email: uniqueValue("order") + "@example.com", Phone: "+49 30 1",
	})
	if err != nil {
		t.Fatalf("upsert customer: %v", err)
	}
	vehicleID, err := st.GetOrCreateVehicleByPlate(ctx, domain.Vehicle{
		LicensePlate: uniqueValue("OO"), Brand: "VW", Model: "Golf", Mileage: 120000,
	})
	if err != nil {
		t.Fatalf("get-or-create vehicle: %v", err)
	}

	requestedDate := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	id, orderNumber, createdAt, err := st.CreateOrder(ctx, customerID, vehicleID, requestedDate, "Bremsen quietschen.")
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if id == 0 {
		t.Fatalf("create order returned id 0")
	}
	if orderNumber == "" {
		t.Fatalf("create order returned an empty order_number")
	}
	if createdAt.IsZero() {
		t.Fatalf("create order returned a zero created_at")
	}

	detail, err := st.ReadOrderDetail(ctx, orderNumber)
	if err != nil {
		t.Fatalf("read order detail: %v", err)
	}
	if detail.OrderNumber != orderNumber {
		t.Fatalf("detail order_number = %q, want %q", detail.OrderNumber, orderNumber)
	}
	if detail.Status != domain.StatusAngefragt {
		t.Fatalf("detail status = %q, want %q", detail.Status, domain.StatusAngefragt)
	}
	if detail.RequestedDate != "2026-11-03" {
		t.Fatalf("detail requested_date = %q, want 2026-11-03", detail.RequestedDate)
	}
	if detail.ProblemDescription != "Bremsen quietschen." {
		t.Fatalf("detail problem_description = %q", detail.ProblemDescription)
	}
	if detail.Customer.Name != "Erika Mustermann" {
		t.Fatalf("detail customer = %+v", detail.Customer)
	}
	if detail.Vehicle.Mileage != 120000 {
		t.Fatalf("detail mileage = %d, want 120000", detail.Vehicle.Mileage)
	}
	if detail.Items == nil || detail.History == nil {
		t.Fatalf("items/history must be empty slices, not nil (items=%v history=%v)", detail.Items, detail.History)
	}
	if len(detail.Items) != 0 || len(detail.History) != 0 {
		t.Fatalf("new order must have no items/history, got %d/%d", len(detail.Items), len(detail.History))
	}
}

func TestReadOrderDetailUnknownOrder(t *testing.T) {
	st, _ := newStore(t)

	_, err := st.ReadOrderDetail(context.Background(), "AU-does-not-exist")
	if err == nil {
		t.Fatalf("reading an unknown order must return an error")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unknown order error = %v, want it to wrap pgx.ErrNoRows", err)
	}
}
