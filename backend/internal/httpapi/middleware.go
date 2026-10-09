package httpapi

import (
	"log"
	"net/http"
	"os"
)

// CORS allows the configured frontend origin to call the api. The origin comes
// from WEB_ORIGIN; when it is unset there is simply no cross-origin access.
// CORS is the outermost middleware, so error responses (including the panic
// recovery below) keep their cross-origin headers.
func CORS(next http.Handler) http.Handler {
	origin := os.Getenv("WEB_ORIGIN")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin != "" {
			if r.Header.Get("Origin") == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				if w.Header().Get("Access-Control-Allow-Origin") != "" {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					w.Header().Set("Access-Control-Max-Age", "600")
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Recover turns a panic in a handler into the uniform internal_error 500 body
// instead of letting the connection die with a stacktrace.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic handling %s %s: %v", r.Method, r.URL.Path, rec)
				WriteError(w, CodeInternalError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// NotFound is the catch-all handler for an unknown path.
func NotFound(w http.ResponseWriter, r *http.Request) {
	WriteError(w, CodeNotFound, "not found")
}

// Method guards a route to a single HTTP method. A request with any other
// method answers the uniform error body with 405 and an Allow header.
func Method(method string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			WriteError(w, CodeMethodNotAllowed, "method not allowed")
			return
		}
		h.ServeHTTP(w, r)
	})
}
