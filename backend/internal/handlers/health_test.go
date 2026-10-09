package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"workshop/internal/config"
	"workshop/internal/handlers"
	"workshop/internal/queue"
	"workshop/internal/store"
	"workshop/internal/testsupport"
)

func TestHealthReportsOKAfterDatabasePing(t *testing.T) {
	pool := testsupport.NewPool(t)
	handlers.Init(store.New(pool), &config.Config{}, queue.New(""))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	handlers.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/health = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("health body is not valid JSON: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("health status = %q, want %q", body.Status, "ok")
	}
}
