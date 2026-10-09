package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"workshop/internal/config"
	"workshop/internal/db"
	"workshop/internal/domain"
	"workshop/internal/handlers"
	"workshop/internal/queue"
	"workshop/internal/store"
	"workshop/internal/testsupport"
)

// applyOnce ensures the schema is created once per test process.
var applyOnce sync.Once

// applySchema creates the api schema under a PostgreSQL advisory lock, so the
// CREATE TABLE IF NOT EXISTS statements cannot race when the test packages run
// in parallel against the same database.
func applySchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	applyOnce.Do(func() {
		ctx := context.Background()
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire schema connection: %v", err)
		}
		defer conn.Release()
		if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(773311224)`); err != nil {
			t.Fatalf("lock schema: %v", err)
		}
		defer func() {
			_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock(773311224)`)
		}()
		if err := db.Apply(ctx, pool); err != nil {
			t.Fatalf("apply schema: %v", err)
		}
	})
}

type handlerSeed struct {
	orderID     int
	orderNumber string
	plate       string
}

// seedHandlerOrder creates the rows one customer order endpoint test needs and
// removes exactly those rows again when the test ends.
func seedHandlerOrder(t *testing.T, pool *pgxpool.Pool) handlerSeed {
	t.Helper()
	ctx := context.Background()

	applySchema(t, pool)

	unique := time.Now().UnixNano()
	orderNumber := fmt.Sprintf("HT-%d", unique)
	plate := fmt.Sprintf("HT-%d", unique%100000000)

	var customerID, vehicleID, orderID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Ben Kunde", "ben@example.test", "0170 0000002",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO vehicles (license_plate, brand, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		plate, "Opel", "Astra", 88000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, requested_date, problem_description)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		orderNumber, customerID, vehicleID, "in Arbeit", "2026-07-02", "Motor ruckelt",
	).Scan(&orderID); err != nil {
		t.Fatalf("insert order: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO order_items (order_id, kind, description, hours, unit_price_cents, total_cents)
		 VALUES ($1, 'labor', $2, $3, $4, $5)`,
		orderID, "Motordiagnose", 1.0, 9000, 9000,
	); err != nil {
		t.Fatalf("insert item: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO order_status_events (order_id, from_status, to_status, changed_by)
		 VALUES ($1, NULL, $2, $3)`,
		orderID, "angefragt", "web",
	); err != nil {
		t.Fatalf("insert status event: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO order_status_events (order_id, from_status, to_status, changed_by)
		 VALUES ($1, $2, $3, $4)`,
		orderID, "angefragt", "bestätigt", "admin",
	); err != nil {
		t.Fatalf("insert status event: %v", err)
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

	return handlerSeed{orderID: orderID, orderNumber: orderNumber, plate: plate}
}

func addHandlerInvoice(t *testing.T, pool *pgxpool.Pool, orderID int) {
	t.Helper()
	ctx := context.Background()
	invoiceNumber := fmt.Sprintf("RE-HT-%d", time.Now().UnixNano())

	var invoiceID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO invoices (invoice_number, order_id, labor_cents, parts_cents, net_cents, vat_cents, gross_cents)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		invoiceNumber, orderID, 9000, 0, 9000, 1710, 10710,
	).Scan(&invoiceID); err != nil {
		t.Fatalf("insert invoice: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO invoice_lines (invoice_id, description, quantity, unit_price_cents, total_cents)
		 VALUES ($1, $2, $3, $4, $5)`,
		invoiceID, "Motordiagnose", 1, 9000, 9000,
	); err != nil {
		t.Fatalf("insert invoice line: %v", err)
	}
}

func orderRequest(orderNumber, plate string) *http.Request {
	req := httptest.NewRequest(http.MethodGet,
		"/api/customer/orders/"+orderNumber+"?license_plate="+url.QueryEscape(plate), nil)
	req.SetPathValue("order_number", orderNumber)
	return req
}

func decodeErrorBody(t *testing.T, rr *httptest.ResponseRecorder) domain.ErrorBody {
	t.Helper()
	var body domain.ErrorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not valid JSON: %v (body=%q)", err, rr.Body.String())
	}
	return body
}

func TestGetCustomerOrderReturnsDetailWithItemsAndHistory(t *testing.T) {
	pool := testsupport.NewPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))
	seeded := seedHandlerOrder(t, pool)

	rr := httptest.NewRecorder()
	handlers.GetCustomerOrder(rr, orderRequest(seeded.orderNumber, seeded.plate))

	if rr.Code != http.StatusOK {
		t.Fatalf("GET customer order = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
	}
	var detail domain.OrderDetail
	if err := json.Unmarshal(rr.Body.Bytes(), &detail); err != nil {
		t.Fatalf("order body is not valid JSON: %v", err)
	}
	if detail.OrderNumber != seeded.orderNumber {
		t.Fatalf("order_number = %q, want %q", detail.OrderNumber, seeded.orderNumber)
	}
	if detail.Status != domain.StatusInArbeit {
		t.Fatalf("status = %q, want in Arbeit", detail.Status)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(detail.Items))
	}
	if len(detail.History) != 2 {
		t.Fatalf("history = %d entries, want 2", len(detail.History))
	}
}

func TestGetCustomerOrderWrongPlateIsNotFound(t *testing.T) {
	pool := testsupport.NewPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))
	seeded := seedHandlerOrder(t, pool)

	rr := httptest.NewRecorder()
	handlers.GetCustomerOrder(rr, orderRequest(seeded.orderNumber, "FALSCH-9"))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("wrong plate = %d, want 404 (body=%q)", rr.Code, rr.Body.String())
	}
	if got := decodeErrorBody(t, rr).Error.Code; got != "not_found" {
		t.Fatalf("error code = %q, want not_found", got)
	}
}

func TestGetCustomerOrderUnknownNumberIsNotFound(t *testing.T) {
	pool := testsupport.NewPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))
	seeded := seedHandlerOrder(t, pool)

	rr := httptest.NewRecorder()
	handlers.GetCustomerOrder(rr, orderRequest("GIBT-ES-NICHT", seeded.plate))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown number = %d, want 404 (body=%q)", rr.Code, rr.Body.String())
	}
	if got := decodeErrorBody(t, rr).Error.Code; got != "not_found" {
		t.Fatalf("error code = %q, want not_found", got)
	}
}

func TestGetCustomerInvoiceNotFoundBeforeInvoicing(t *testing.T) {
	pool := testsupport.NewPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))
	seeded := seedHandlerOrder(t, pool)

	rr := httptest.NewRecorder()
	handlers.GetCustomerInvoice(rr, orderRequest(seeded.orderNumber, seeded.plate))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("invoice before invoicing = %d, want 404 (body=%q)", rr.Code, rr.Body.String())
	}
	if got := decodeErrorBody(t, rr).Error.Code; got != "not_found" {
		t.Fatalf("error code = %q, want not_found", got)
	}
}

func TestGetCustomerInvoiceWrongPlateIsNotFound(t *testing.T) {
	pool := testsupport.NewPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))
	seeded := seedHandlerOrder(t, pool)
	addHandlerInvoice(t, pool, seeded.orderID)

	rr := httptest.NewRecorder()
	handlers.GetCustomerInvoice(rr, orderRequest(seeded.orderNumber, "FALSCH-9"))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("wrong plate invoice = %d, want 404 (body=%q)", rr.Code, rr.Body.String())
	}
	if got := decodeErrorBody(t, rr).Error.Code; got != "not_found" {
		t.Fatalf("error code = %q, want not_found", got)
	}
}

func TestGetCustomerInvoiceReturnsLinesAndCents(t *testing.T) {
	pool := testsupport.NewPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))
	seeded := seedHandlerOrder(t, pool)
	addHandlerInvoice(t, pool, seeded.orderID)

	rr := httptest.NewRecorder()
	handlers.GetCustomerInvoice(rr, orderRequest(seeded.orderNumber, seeded.plate))

	if rr.Code != http.StatusOK {
		t.Fatalf("GET customer invoice = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
	}
	var invoice domain.Invoice
	if err := json.Unmarshal(rr.Body.Bytes(), &invoice); err != nil {
		t.Fatalf("invoice body is not valid JSON: %v", err)
	}
	if invoice.OrderNumber != seeded.orderNumber {
		t.Fatalf("order_number = %q, want %q", invoice.OrderNumber, seeded.orderNumber)
	}
	if invoice.LaborCents != 9000 || invoice.PartsCents != 0 {
		t.Fatalf("labor/parts = %d/%d, want 9000/0", invoice.LaborCents, invoice.PartsCents)
	}
	if invoice.NetCents != 9000 || invoice.VatCents != 1710 || invoice.GrossCents != 10710 {
		t.Fatalf("net/vat/gross = %d/%d/%d, want 9000/1710/10710", invoice.NetCents, invoice.VatCents, invoice.GrossCents)
	}
	if len(invoice.Lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(invoice.Lines))
	}
}
