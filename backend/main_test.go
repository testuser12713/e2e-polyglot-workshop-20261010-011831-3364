package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// serve sends one request through the real route table.
func serve(method, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	registerRoutes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(method, path, nil))
	return rr
}

func TestContractRoutesAreRegistered(t *testing.T) {
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/health"},
		{http.MethodPost, "/api/customer/appointments"},
		{http.MethodGet, "/api/customer/orders/AB-123"},
		{http.MethodGet, "/api/customer/orders/AB-123/invoice"},
		{http.MethodPost, "/api/workshop/login"},
		{http.MethodGet, "/api/workshop/orders"},
		{http.MethodGet, "/api/workshop/orders/AB-123"},
		{http.MethodPost, "/api/workshop/orders/AB-123/status"},
		{http.MethodPost, "/api/workshop/orders/AB-123/items"},
		{http.MethodPut, "/api/workshop/orders/AB-123/items/1"},
		{http.MethodGet, "/api/workshop/dashboard"},
	}
	for _, tc := range cases {
		rr := serve(tc.method, tc.path)
		if rr.Code == http.StatusNotFound {
			t.Errorf("%s %s should be routed, got 404", tc.method, tc.path)
		}
	}
}

func TestUnknownPathIsNotFound(t *testing.T) {
	rr := serve(http.MethodGet, "/api/nope")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown path = %d, want 404", rr.Code)
	}
}

func TestWrongMethodIsMethodNotAllowed(t *testing.T) {
	rr := serve(http.MethodPost, "/api/health")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method = %d, want 405", rr.Code)
	}
}
