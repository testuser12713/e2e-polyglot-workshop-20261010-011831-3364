package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"workshop/internal/domain"
)

// countingHandler is a stub inner handler that counts how often it is invoked,
// standing in for the login handler the limiter wraps.
func countingHandler(calls *int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		w.WriteHeader(http.StatusOK)
	})
}

// request sends one login request from remoteAddr through h.
func request(h http.Handler, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/workshop/login", nil)
	req.RemoteAddr = remoteAddr
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestRateLimitBlocksEleventhAttemptAndSkipsHandler(t *testing.T) {
	var calls int
	h := RateLimit(countingHandler(&calls))

	for i := 0; i < rateLimitMax; i++ {
		rr := request(h, "203.0.113.7:12345")
		if rr.Code != http.StatusOK {
			t.Fatalf("attempt %d: got status %d, want 200", i+1, rr.Code)
		}
	}
	if calls != rateLimitMax {
		t.Fatalf("inner handler calls = %d, want %d", calls, rateLimitMax)
	}

	rr := request(h, "203.0.113.7:12345")
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("eleventh attempt: got status %d, want 429", rr.Code)
	}
	if calls != rateLimitMax {
		t.Fatalf("inner handler ran for a blocked request: calls = %d, want %d", calls, rateLimitMax)
	}

	var body domain.ErrorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not valid JSON: %v (body=%q)", err, rr.Body.String())
	}
	if body.Error.Code != "rate_limited" {
		t.Fatalf("error code = %q, want rate_limited", body.Error.Code)
	}
}

func TestRateLimitIsPerClient(t *testing.T) {
	var calls int
	h := RateLimit(countingHandler(&calls))

	for i := 0; i < rateLimitMax; i++ {
		if rr := request(h, "203.0.113.7:1111"); rr.Code != http.StatusOK {
			t.Fatalf("first client attempt %d: got %d, want 200", i+1, rr.Code)
		}
	}

	if rr := request(h, "198.51.100.9:2222"); rr.Code != http.StatusOK {
		t.Fatalf("second client blocked: got %d, want 200", rr.Code)
	}

	if rr := request(h, "203.0.113.7:3333"); rr.Code != http.StatusTooManyRequests {
		t.Fatalf("first client eleventh attempt: got %d, want 429", rr.Code)
	}
}

func TestRateLimiterRestoresBudgetAfterWindow(t *testing.T) {
	rl := newRateLimiter()
	base := time.Now()
	rl.now = func() time.Time { return base }

	for i := 0; i < rateLimitMax; i++ {
		if !rl.allow("client") {
			t.Fatalf("attempt %d unexpectedly blocked", i+1)
		}
	}
	if rl.allow("client") {
		t.Fatal("attempt beyond the limit was allowed")
	}

	rl.now = func() time.Time { return base.Add(rateLimitWindow + time.Second) }
	if !rl.allow("client") {
		t.Fatal("attempt after the window expired was blocked")
	}
}
