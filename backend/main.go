package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"workshop/internal/auth"
	"workshop/internal/config"
	"workshop/internal/db"
	"workshop/internal/handlers"
	"workshop/internal/httpapi"
	"workshop/internal/queue"
	"workshop/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Apply(ctx, pool); err != nil {
		return err
	}

	st := store.New(pool)
	passwordHash, err := auth.HashPassword(cfg.WorkshopAdminPassword)
	if err != nil {
		return fmt.Errorf("hash workshop admin password: %w", err)
	}
	if err := st.EnsureSeedEmployee(ctx, cfg.WorkshopAdminEmail, passwordHash); err != nil {
		return fmt.Errorf("seed workshop admin: %w", err)
	}

	handlers.Init(st, cfg, queue.New(cfg.ValkeyURL))

	mux := http.NewServeMux()
	registerRoutes(mux)
	handler := httpapi.CORS(httpapi.Recover(mux))

	addr := ":" + cfg.Port
	return http.ListenAndServe(addr, handler)
}

// registerRoutes declares every contract route on the single ServeMux. A route
// is guarded to its method; workshop routes require a session and the login
// route is rate limited.
func registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", httpapi.NotFound)

	route(mux, http.MethodGet, "/api/health", handlers.Health)

	route(mux, http.MethodPost, "/api/customer/appointments", handlers.CreateCustomerAppointment)
	route(mux, http.MethodGet, "/api/customer/orders/{order_number}", handlers.GetCustomerOrder)
	route(mux, http.MethodGet, "/api/customer/orders/{order_number}/invoice", handlers.GetCustomerInvoice)

	login := auth.RateLimit(http.HandlerFunc(handlers.WorkshopLogin))
	mux.Handle("/api/workshop/login", httpapi.Method(http.MethodPost, login))

	routeProtected(mux, http.MethodGet, "/api/workshop/orders", handlers.ListWorkshopOrders)
	routeProtected(mux, http.MethodGet, "/api/workshop/orders/{order_number}", handlers.GetWorkshopOrder)
	routeProtected(mux, http.MethodPost, "/api/workshop/orders/{order_number}/status", handlers.UpdateOrderStatus)
	routeProtected(mux, http.MethodPost, "/api/workshop/orders/{order_number}/items", handlers.CreateOrderItem)
	routeProtected(mux, http.MethodPut, "/api/workshop/orders/{order_number}/items/{item_id}", handlers.UpdateOrderItem)
	routeProtected(mux, http.MethodGet, "/api/workshop/dashboard", handlers.GetWorkshopDashboard)
}

func route(mux *http.ServeMux, method, path string, h http.HandlerFunc) {
	mux.Handle(path, httpapi.Method(method, h))
}

func routeProtected(mux *http.ServeMux, method, path string, h http.HandlerFunc) {
	mux.Handle(path, httpapi.Method(method, auth.RequireUser(h)))
}
