package store_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/domain"
	"workshop/internal/store"
	"workshop/internal/testsupport"
)

type seededOrder struct {
	id          int
	orderNumber string
	plate       string
}

// seedCustomerOrder creates one customer, vehicle, order, two items and two status
// events. It only inserts rows it owns and removes exactly those again.
func seedCustomerOrder(t *testing.T, pool *pgxpool.Pool) seededOrder {
	t.Helper()
	ctx := context.Background()

	applySchema(t, pool)

	unique := time.Now().UnixNano()
	orderNumber := fmt.Sprintf("ST-%d", unique)
	plate := fmt.Sprintf("ST-%d", unique%100000000)

	var customerID, vehicleID, orderID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Anna Kunde", "anna@example.test", "0170 0000001",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO vehicles (license_plate, brand, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		plate, "VW", "Golf", 123456,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, requested_date, problem_description)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		orderNumber, customerID, vehicleID, "in Arbeit", "2026-07-01", "Bremsen quietschen",
	).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO order_items (order_id, kind, description, hours, unit_price_cents, total_cents)
		 VALUES ($1, 'labor', $2, $3, $4, $5)`,
		orderID, "Bremsbeläge erneuern", 1.5, 8000, 12000,
	); err != nil {
		t.Fatalf("insert labor item: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO order_items (order_id, kind, description, quantity, unit_price_cents, total_cents)
		 VALUES ($1, 'part', $2, $3, $4, $5)`,
		orderID, "Bremsbelag", 2, 4500, 9000,
	); err != nil {
		t.Fatalf("insert part item: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO order_status_events (order_id, from_status, to_status, changed_by)
		 VALUES ($1, NULL, $2, $3)`,
		orderID, "angefragt", "web",
	); err != nil {
		t.Fatalf("insert first status event: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO order_status_events (order_id, from_status, to_status, changed_by)
		 VALUES ($1, $2, $3, $4)`,
		orderID, "angefragt", "bestätigt", "admin",
	); err != nil {
		t.Fatalf("insert second status event: %v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, `DELETE FROM invoice_lines WHERE invoice_id IN (SELECT id FROM invoices WHERE order_id = $1)`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM invoices WHERE order_id = $1`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM order_status_events WHERE order_id = $1`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM order_items WHERE order_id = $1`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM orders WHERE id = $1`, orderID)
		_, _ = pool.Exec(ctx, `DELETE FROM customers WHERE id = $1`, customerID)
		_, _ = pool.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, vehicleID)
	})

	return seededOrder{id: orderID, orderNumber: orderNumber, plate: plate}
}

func addInvoice(t *testing.T, pool *pgxpool.Pool, orderID int) string {
	t.Helper()
	ctx := context.Background()
	invoiceNumber := fmt.Sprintf("RE-%d", time.Now().UnixNano())

	var invoiceID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO invoices (invoice_number, order_id, labor_cents, parts_cents, net_cents, vat_cents, gross_cents)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		invoiceNumber, orderID, 12000, 9000, 21000, 3990, 24990,
	).Scan(&invoiceID); err != nil {
		t.Fatalf("insert invoice: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO invoice_lines (invoice_id, description, quantity, unit_price_cents, total_cents)
		 VALUES ($1, $2, $3, $4, $5)`,
		invoiceID, "Bremsbeläge erneuern", 1, 12000, 12000,
	); err != nil {
		t.Fatalf("insert invoice line: %v", err)
	}
	return invoiceNumber
}

func TestOrderForCustomerReturnsDetail(t *testing.T) {
	pool := testsupport.NewPool(t)
	st := store.New(pool)
	seeded := seedCustomerOrder(t, pool)

	orderID, detail, err := st.OrderForCustomer(context.Background(), seeded.orderNumber, seeded.plate)
	if err != nil {
		t.Fatalf("OrderForCustomer: %v", err)
	}
	if orderID != seeded.id {
		t.Fatalf("orderID = %d, want %d", orderID, seeded.id)
	}
	if detail.OrderNumber != seeded.orderNumber {
		t.Fatalf("OrderNumber = %q, want %q", detail.OrderNumber, seeded.orderNumber)
	}
	if detail.Status != domain.StatusInArbeit {
		t.Fatalf("Status = %q, want %q", detail.Status, domain.StatusInArbeit)
	}
	if detail.RequestedDate != "2026-07-01" {
		t.Fatalf("RequestedDate = %q, want 2026-07-01", detail.RequestedDate)
	}
	if detail.Customer.Name != "Anna Kunde" || detail.Vehicle.LicensePlate != seeded.plate {
		t.Fatalf("customer/vehicle not loaded: %+v %+v", detail.Customer, detail.Vehicle)
	}
	if len(detail.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(detail.Items))
	}
	if detail.Items[0].Kind != "labor" || detail.Items[0].Hours == nil {
		t.Fatalf("labor item not shaped: %+v", detail.Items[0])
	}
	if got := *detail.Items[0].Hours; got < 1.49 || got > 1.51 {
		t.Fatalf("labor hours = %v, want ~1.5", got)
	}
	if detail.Items[1].Kind != "part" || detail.Items[1].Quantity == nil || *detail.Items[1].Quantity != 2 {
		t.Fatalf("part item not shaped: %+v", detail.Items[1])
	}
}

func TestOrderForCustomerWrongPlateIsNotFound(t *testing.T) {
	pool := testsupport.NewPool(t)
	st := store.New(pool)
	seeded := seedCustomerOrder(t, pool)

	_, _, err := st.OrderForCustomer(context.Background(), seeded.orderNumber, "FALSCH-1")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("wrong plate error = %v, want ErrNotFound", err)
	}
}

func TestOrderForCustomerUnknownNumberIsNotFound(t *testing.T) {
	pool := testsupport.NewPool(t)
	st := store.New(pool)
	seeded := seedCustomerOrder(t, pool)

	_, _, err := st.OrderForCustomer(context.Background(), "GIBT-ES-NICHT", seeded.plate)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown number error = %v, want ErrNotFound", err)
	}
}

func TestOrderHistoryReturnsEvents(t *testing.T) {
	pool := testsupport.NewPool(t)
	st := store.New(pool)
	seeded := seedCustomerOrder(t, pool)

	events, err := st.OrderHistory(context.Background(), seeded.id)
	if err != nil {
		t.Fatalf("OrderHistory: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("history = %d events, want 2", len(events))
	}
	if events[0].FromStatus != nil {
		t.Fatalf("first event FromStatus = %v, want nil", *events[0].FromStatus)
	}
	if events[0].ToStatus != domain.StatusAngefragt {
		t.Fatalf("first ToStatus = %q, want angefragt", events[0].ToStatus)
	}
	if events[1].FromStatus == nil || *events[1].FromStatus != domain.StatusAngefragt {
		t.Fatalf("second FromStatus not loaded: %+v", events[1])
	}
	if events[1].ToStatus != domain.StatusBestaetigt {
		t.Fatalf("second ToStatus = %q, want bestätigt", events[1].ToStatus)
	}
}

func TestInvoiceForOrderNilBeforeInvoicing(t *testing.T) {
	pool := testsupport.NewPool(t)
	st := store.New(pool)
	seeded := seedCustomerOrder(t, pool)

	invoice, err := st.InvoiceForOrder(context.Background(), seeded.id)
	if err != nil {
		t.Fatalf("InvoiceForOrder: %v", err)
	}
	if invoice != nil {
		t.Fatalf("invoice = %+v, want nil before invoicing", invoice)
	}
}

func TestInvoiceForOrderReturnsLinesAndCents(t *testing.T) {
	pool := testsupport.NewPool(t)
	st := store.New(pool)
	seeded := seedCustomerOrder(t, pool)
	invoiceNumber := addInvoice(t, pool, seeded.id)

	invoice, err := st.InvoiceForOrder(context.Background(), seeded.id)
	if err != nil {
		t.Fatalf("InvoiceForOrder: %v", err)
	}
	if invoice == nil {
		t.Fatal("invoice = nil, want a loaded invoice")
	}
	if invoice.InvoiceNumber != invoiceNumber {
		t.Fatalf("InvoiceNumber = %q, want %q", invoice.InvoiceNumber, invoiceNumber)
	}
	if invoice.OrderNumber != seeded.orderNumber {
		t.Fatalf("OrderNumber = %q, want %q", invoice.OrderNumber, seeded.orderNumber)
	}
	if invoice.LaborCents != 12000 || invoice.PartsCents != 9000 {
		t.Fatalf("labor/parts = %d/%d, want 12000/9000", invoice.LaborCents, invoice.PartsCents)
	}
	if invoice.NetCents != 21000 || invoice.VatCents != 3990 || invoice.GrossCents != 24990 {
		t.Fatalf("net/vat/gross = %d/%d/%d, want 21000/3990/24990", invoice.NetCents, invoice.VatCents, invoice.GrossCents)
	}
	if len(invoice.Lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(invoice.Lines))
	}
	if invoice.Lines[0].TotalCents != 12000 {
		t.Fatalf("line total = %d, want 12000", invoice.Lines[0].TotalCents)
	}
}
