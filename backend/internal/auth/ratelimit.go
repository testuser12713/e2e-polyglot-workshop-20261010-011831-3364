package auth

import (
	"net"
	"net/http"
	"sync"
	"time"

	"workshop/internal/httpapi"
)

const (
	// rateLimitMax is the number of attempts allowed per client within one
	// rolling window before further requests are rejected.
	rateLimitMax = 10
	// rateLimitWindow is the length of the rolling window.
	rateLimitWindow = time.Minute
)

// rateLimiter is an in-memory, per-client counter. It records the timestamps of
// recent attempts per client key and prunes those that fell out of the rolling
// window, so a client's budget is restored a minute after its oldest attempt.
type rateLimiter struct {
	mu      sync.Mutex
	clients map[string][]time.Time
	// now is the clock, injectable so tests can travel through time.
	now func() time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{
		clients: make(map[string][]time.Time),
		now:     time.Now,
	}
}

// allow reports whether a new attempt from key is permitted. It records the
// attempt when it is allowed. Expired timestamps are pruned on every call; a
// key whose whole window has expired is removed from the map again so the
// counters do not grow without bound.
func (rl *rateLimiter) allow(key string) bool {
	now := rl.now()
	cutoff := now.Add(-rateLimitWindow)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	recent := rl.clients[key]
	pruned := recent[:0]
	for _, at := range recent {
		if at.After(cutoff) {
			pruned = append(pruned, at)
		}
	}

	if len(pruned) >= rateLimitMax {
		rl.clients[key] = pruned
		return false
	}

	if len(pruned) == 0 {
		delete(rl.clients, key)
	} else {
		rl.clients[key] = pruned
	}
	rl.clients[key] = append(rl.clients[key], now)
	return true
}

// clientKey identifies the client of a request. It is the remote address
// without the port, falling back to the raw value when it cannot be split.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimit wraps the login handler with a per-client attempt limit. More than
// rateLimitMax requests from one client within rateLimitWindow are answered
// with the uniform 429 error body before the wrapped handler runs, so the
// password is never checked. It is transport-agnostic middleware and only the
// login route is wired through it.
func RateLimit(next http.Handler) http.Handler {
	limiter := newRateLimiter()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(clientKey(r)) {
			httpapi.WriteError(w, httpapi.CodeRateLimited, "too many login attempts, try again later")
			return
		}
		next.ServeHTTP(w, r)
	})
}
