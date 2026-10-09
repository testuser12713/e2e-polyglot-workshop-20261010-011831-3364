package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"workshop/internal/domain"
)

func decodeError(t *testing.T, body []byte) domain.ErrorBody {
	t.Helper()
	var parsed domain.ErrorBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("error body is not valid JSON: %v (body=%q)", err, body)
	}
	return parsed
}

func TestWriteErrorUsesUniformBodyAndStatus(t *testing.T) {
	cases := []struct {
		code string
		want int
	}{
		{CodeValidationError, http.StatusBadRequest},
		{CodeUnauthorized, http.StatusUnauthorized},
		{CodeNotFound, http.StatusNotFound},
		{CodeInvalidTransition, http.StatusConflict},
		{CodeRateLimited, http.StatusTooManyRequests},
		{CodeInternalError, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		rr := httptest.NewRecorder()
		WriteError(rr, tc.code, "some message")
		if rr.Code != tc.want {
			t.Errorf("code %s: got status %d, want %d", tc.code, rr.Code, tc.want)
		}
		got := decodeError(t, rr.Body.Bytes())
		if got.Error.Code != tc.code {
			t.Errorf("code %s: body code %q", tc.code, got.Error.Code)
		}
		if got.Error.Message == "" {
			t.Errorf("code %s: body message is empty", tc.code)
		}
	}
}

func TestStatusForCodeUnknownIsInternalError(t *testing.T) {
	if got := StatusForCode("no_such_code"); got != http.StatusInternalServerError {
		t.Fatalf("StatusForCode(unknown) = %d, want 500", got)
	}
}

func TestWriteJSONSetsContentType(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteJSON(rr, http.StatusOK, domain.HealthResponse{Status: "ok"})
	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func newTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", NotFound)
	mux.Handle("/api/health", Method(http.MethodGet, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, domain.HealthResponse{Status: "ok"})
	})))
	return mux
}

func TestUnknownPathReturnsUniformNotFound(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil)
	newTestMux().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want 404", rr.Code)
	}
	if got := decodeError(t, rr.Body.Bytes()); got.Error.Code != CodeNotFound {
		t.Fatalf("body code = %q, want %q", got.Error.Code, CodeNotFound)
	}
}

func TestUnknownMethodReturnsUniformMethodNotAllowed(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	newTestMux().ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got status %d, want 405", rr.Code)
	}
	if allow := rr.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("Allow = %q, want GET", allow)
	}
	if got := decodeError(t, rr.Body.Bytes()); got.Error.Code != CodeMethodNotAllowed {
		t.Fatalf("body code = %q, want %q", got.Error.Code, CodeMethodNotAllowed)
	}
}

func TestRecoverTurnsPanicIntoInternalError(t *testing.T) {
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	Recover(panicking).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d, want 500", rr.Code)
	}
	body := rr.Body.String()
	if got := decodeError(t, []byte(body)); got.Error.Code != CodeInternalError {
		t.Fatalf("body code = %q, want %q", got.Error.Code, CodeInternalError)
	}
}
