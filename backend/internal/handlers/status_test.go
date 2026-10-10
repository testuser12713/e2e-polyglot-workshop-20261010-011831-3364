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
	"github.com/redis/go-redis/v9"

	"workshop/internal/auth"
	"workshop/internal/config"
	"workshop/internal/db"
	"workshop/internal/domain"
	"workshop/internal/handlers"
	"workshop/internal/queue"
	"workshop/internal/store"
)

const statusInvoicesQueueKey = "invoices:queue"

// statusHandlerPool connects to the suite's PostgreSQL and applies the schema.
// It prefers TEST_DATABASE_URL and falls back to DATABASE_URL (the variable the
// pipeline sets); without either the test is skipped cleanly. It is never
// SQLite.
func statusHandlerPool(t *testing.T) *pgxpool.Pool {
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

// statusHandlerRedis connects to the suite's Valkey; without a URL the test is
// skipped.
func statusHandlerRedis(t *testing.T) *redis.Client {
	t.Helper()
	url := os.Getenv("TEST_VALKEY_URL")
	if url == "" {
		url = os.Getenv("VALKEY_URL")
	}
	if url == "" {
		t.Skip("no TEST_VALKEY_URL/VALKEY_URL — skipping queue-backed test")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatalf("parse valkey url: %v", err)
	}
	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// statusEnv wires the handler dependencies against the real database and queue
// and clears the completion queue.
func statusEnv(t *testing.T) (*pgxpool.Pool, *redis.Client) {
	t.Helper()
	pool := statusHandlerPool(t)
	client := statusHandlerRedis(t)
	valkeyURL := os.Getenv("TEST_VALKEY_URL")
	if valkeyURL == "" {
		valkeyURL = os.Getenv("VALKEY_URL")
	}
	handlers.Init(store.New(pool), &config.Config{}, queue.New(valkeyURL))
	if err := client.Del(context.Background(), statusInvoicesQueueKey).Err(); err != nil {
		t.Fatalf("clear queue: %v", err)
	}
	return pool, client
}

func statusSeedOrder(t *testing.T, pool *pgxpool.Pool, status domain.OrderStatus) (int, string) {
	t.Helper()
	ctx := context.Background()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	var customerID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
		"Status Handler Customer", "status-handler-"+suffix+"@example.com", "0123456789",
	).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	var vehicleID int
	if err := pool.QueryRow(ctx,
		`INSERT INTO vehicles (license_plate, brand, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id`,
		"HSTATUS-"+suffix, "Test", "Model", 1000,
	).Scan(&vehicleID); err != nil {
		t.Fatalf("insert vehicle: %v", err)
	}

	orderNumber := "HSTATUS-" + suffix
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

func statusPost(t *testing.T, orderNumber, status string, withUser bool) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"status":%q}`, status)
	req := httptest.NewRequest(http.MethodPost, "/api/workshop/orders/"+orderNumber+"/status", strings.NewReader(body))
	req.SetPathValue("order_number", orderNumber)
	if withUser {
		req = req.WithContext(auth.WithUser(req.Context(), auth.User{ID: 11, Name: "Tester", Email: "tester@example.com"}))
	}
	rr := httptest.NewRecorder()
	handlers.UpdateOrderStatus(rr, req)
	return rr
}

func statusDecodeDetail(t *testing.T, rr *httptest.ResponseRecorder) domain.OrderDetail {
	t.Helper()
	var detail domain.OrderDetail
	if err := json.Unmarshal(rr.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode order detail: %v (body=%q)", err, rr.Body.String())
	}
	return detail
}

func statusDecodeErrorCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var body domain.ErrorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (body=%q)", err, rr.Body.String())
	}
	return body.Error.Code
}

func statusCurrentStatus(t *testing.T, pool *pgxpool.Pool, orderID int) domain.OrderStatus {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(),
		`SELECT status FROM orders WHERE id = $1`, orderID).Scan(&status); err != nil {
		t.Fatalf("read status: %v", err)
	}
	return domain.OrderStatus(status)
}

func statusEventCount(t *testing.T, pool *pgxpool.Pool, orderID int) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM order_status_events WHERE order_id = $1`, orderID).Scan(&n); err != nil {
		t.Fatalf("count events: %v", err)
	}
	return n
}

func statusQueueLen(t *testing.T, client *redis.Client) int64 {
	t.Helper()
	n, err := client.LLen(context.Background(), statusInvoicesQueueKey).Result()
	if err != nil {
		t.Fatalf("queue length: %v", err)
	}
	return n
}

func TestUpdateOrderStatusWalksChainAndPublishesOnce(t *testing.T) {
	pool, client := statusEnv(t)
	orderID, orderNumber := statusSeedOrder(t, pool, domain.StatusAngefragt)

	steps := []domain.OrderStatus{
		domain.StatusBestaetigt,
		domain.StatusInArbeit,
		domain.StatusFertig,
		domain.StatusAbgeholt,
	}
	for i, step := range steps {
		rr := statusPost(t, orderNumber, string(step), true)
		if rr.Code != http.StatusOK {
			t.Fatalf("step %d (%s): status = %d, body = %s", i, step, rr.Code, rr.Body.String())
		}
		detail := statusDecodeDetail(t, rr)
		if detail.Status != step {
			t.Fatalf("step %d: returned status %q, want %q", i, step, detail.Status)
		}
		if len(detail.History) != i+1 {
			t.Fatalf("step %d: history length = %d, want %d", i, len(detail.History), i+1)
		}
		if last := detail.History[len(detail.History)-1]; last.ChangedBy != "Tester" {
			t.Fatalf("step %d: changed_by = %q, want the employee name %q", i, last.ChangedBy, "Tester")
		}
	}

	if got := statusCurrentStatus(t, pool, orderID); got != domain.StatusAbgeholt {
		t.Fatalf("persisted status = %q, want abgeholt", got)
	}

	if n := statusQueueLen(t, client); n != 1 {
		t.Fatalf("queue length after reaching fertig = %d, want exactly 1", n)
	}
	raw, err := client.LRange(context.Background(), statusInvoicesQueueKey, 0, -1).Result()
	if err != nil {
		t.Fatalf("read queue: %v", err)
	}
	var payload struct {
		OrderID     int    `json:"order_id"`
		OrderNumber string `json:"order_number"`
	}
	if err := json.Unmarshal([]byte(raw[0]), &payload); err != nil {
		t.Fatalf("decode queue payload: %v (raw=%q)", err, raw[0])
	}
	if payload.OrderID != orderID || payload.OrderNumber != orderNumber {
		t.Fatalf("queue payload = %+v, want order_id %d order_number %q", payload, orderID, orderNumber)
	}
}

func TestUpdateOrderStatusRejectsJumpsWithoutChangingDatabase(t *testing.T) {
	pool, client := statusEnv(t)
	orderID, orderNumber := statusSeedOrder(t, pool, domain.StatusAngefragt)

	jumps := []string{
		string(domain.StatusInArbeit),
		string(domain.StatusFertig),
		string(domain.StatusAbgeholt),
	}
	for _, target := range jumps {
		rr := statusPost(t, orderNumber, target, true)
		if rr.Code != http.StatusConflict {
			t.Fatalf("jump angefragt -> %s: status = %d, want 409 (body=%s)", target, rr.Code, rr.Body.String())
		}
		if code := statusDecodeErrorCode(t, rr); code != "invalid_transition" {
			t.Fatalf("jump -> %s: body code = %q, want invalid_transition", target, code)
		}
	}

	if got := statusCurrentStatus(t, pool, orderID); got != domain.StatusAngefragt {
		t.Fatalf("status = %q after rejected transitions, want angefragt", got)
	}
	if n := statusEventCount(t, pool, orderID); n != 0 {
		t.Fatalf("recorded %d events for rejected transitions, want 0", n)
	}
	if n := statusQueueLen(t, client); n != 0 {
		t.Fatalf("queue length = %d after rejected transitions, want 0", n)
	}
}

func TestUpdateOrderStatusUnknownValueIsValidationError(t *testing.T) {
	pool, _ := statusEnv(t)
	orderID, orderNumber := statusSeedOrder(t, pool, domain.StatusAngefragt)

	rr := statusPost(t, orderNumber, "kaputt", true)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unknown status: status = %d, want 400 (body=%s)", rr.Code, rr.Body.String())
	}
	if code := statusDecodeErrorCode(t, rr); code != "validation_error" {
		t.Fatalf("unknown status: body code = %q, want validation_error", code)
	}
	if got := statusCurrentStatus(t, pool, orderID); got != domain.StatusAngefragt {
		t.Fatalf("status = %q, want unchanged angefragt", got)
	}
}

func TestUpdateOrderStatusUnknownOrderIsNotFound(t *testing.T) {
	statusEnv(t)
	rr := statusPost(t, "DOES-NOT-EXIST-"+fmt.Sprintf("%d", time.Now().UnixNano()), string(domain.StatusBestaetigt), true)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown order: status = %d, want 404 (body=%s)", rr.Code, rr.Body.String())
	}
	if code := statusDecodeErrorCode(t, rr); code != "not_found" {
		t.Fatalf("unknown order: body code = %q, want not_found", code)
	}
}

func TestUpdateOrderStatusWithoutSessionIsUnauthorized(t *testing.T) {
	pool, client := statusEnv(t)
	orderID, orderNumber := statusSeedOrder(t, pool, domain.StatusAngefragt)

	rr := statusPost(t, orderNumber, string(domain.StatusBestaetigt), false)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("without session: status = %d, want 401 (body=%s)", rr.Code, rr.Body.String())
	}
	if code := statusDecodeErrorCode(t, rr); code != "unauthorized" {
		t.Fatalf("without session: body code = %q, want unauthorized", code)
	}
	if got := statusCurrentStatus(t, pool, orderID); got != domain.StatusAngefragt {
		t.Fatalf("status = %q, want unchanged angefragt", got)
	}
	if n := statusQueueLen(t, client); n != 0 {
		t.Fatalf("queue length = %d, want 0", n)
	}
}
