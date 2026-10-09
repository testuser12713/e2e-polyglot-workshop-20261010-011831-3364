package auth

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" || hash == "correct horse battery staple" {
		t.Fatalf("hash must be a real hash, got %q", hash)
	}
	if !CheckPassword(hash, "correct horse battery staple") {
		t.Fatal("CheckPassword rejected the correct password")
	}
	if CheckPassword(hash, "wrong password") {
		t.Fatal("CheckPassword accepted a wrong password")
	}
}

func TestIssueTokenRoundTrip(t *testing.T) {
	t.Setenv("AUTH_TOKEN_SECRET", "test-secret")
	want := User{ID: 7, Name: "Ada", Email: "ada@werkstatt.example"}
	token, err := IssueToken(want)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if token == "" {
		t.Fatal("IssueToken returned an empty token")
	}
	got, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken rejected a freshly issued token: %v", err)
	}
	if got != want {
		t.Fatalf("ParseToken = %+v, want %+v", got, want)
	}
}

func TestParseTokenRejectsTamperedAndExpired(t *testing.T) {
	t.Setenv("AUTH_TOKEN_SECRET", "test-secret")

	token, err := IssueToken(User{ID: 1, Name: "Ada", Email: "ada@werkstatt.example"})
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := ParseToken(token + "x"); err == nil {
		t.Fatal("ParseToken accepted a tampered token")
	}
	if _, err := ParseToken("not-a-token"); err == nil {
		t.Fatal("ParseToken accepted a malformed token")
	}

	expired, err := json.Marshal(tokenClaims{ID: 1, Exp: time.Now().Add(-time.Hour).Unix()})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(expired)
	expiredToken := encoded + "." + sign([]byte("test-secret"), encoded)
	if _, err := ParseToken(expiredToken); err == nil {
		t.Fatal("ParseToken accepted an expired token")
	}
}

func TestRequireUserWithoutOrInvalidTokenIsUnauthorized(t *testing.T) {
	t.Setenv("AUTH_TOKEN_SECRET", "test-secret")

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	protected := RequireUser(next)

	cases := []struct {
		name   string
		header string
	}{
		{"no header", ""},
		{"wrong scheme", "Basic abc"},
		{"invalid token", "Bearer not-a-token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/workshop/orders", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			protected.ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rr.Code)
			}
			if called {
				t.Fatal("wrapped handler ran for an unauthenticated request")
			}
			if !strings.Contains(rr.Body.String(), `"unauthorized"`) {
				t.Fatalf("body = %q, want the uniform unauthorized error body", rr.Body.String())
			}
		})
	}
}

func TestRequireUserPassesUserThroughContext(t *testing.T) {
	t.Setenv("AUTH_TOKEN_SECRET", "test-secret")

	want := User{ID: 42, Name: "Ada", Email: "ada@werkstatt.example"}
	token, err := IssueToken(want)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	var got User
	var ok bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = UserFromContext(r.Context())
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/workshop/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	RequireUser(next).ServeHTTP(rr, req)

	if !ok {
		t.Fatal("no user in context after a valid token")
	}
	if got != want {
		t.Fatalf("context user = %+v, want %+v", got, want)
	}
}
