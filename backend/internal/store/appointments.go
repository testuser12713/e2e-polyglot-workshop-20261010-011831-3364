package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"workshop/internal/domain"
)

// UpsertCustomer returns the id of the customer identified by e-mail. A known
// e-mail reuses the existing row (name and phone are refreshed); an unknown one
// inserts a new row. Every statement is parametrized.
func (s *Store) UpsertCustomer(ctx context.Context, c domain.Customer) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin customer upsert: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	err = tx.QueryRow(ctx,
		`SELECT id FROM customers WHERE email = $1 ORDER BY id LIMIT 1 FOR UPDATE`,
		c.Email,
	).Scan(&id)
	switch {
	case err == nil:
		if _, err := tx.Exec(ctx,
			`UPDATE customers SET name = $2, phone = $3 WHERE id = $1`,
			id, c.Name, c.Phone,
		); err != nil {
			return 0, fmt.Errorf("update customer: %w", err)
		}
	case errors.Is(err, pgx.ErrNoRows):
		if err := tx.QueryRow(ctx,
			`INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
			c.Name, c.Email, c.Phone,
		).Scan(&id); err != nil {
			return 0, fmt.Errorf("insert customer: %w", err)
		}
	default:
		return 0, fmt.Errorf("select customer: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit customer upsert: %w", err)
	}
	return id, nil
}

// GetOrCreateVehicleByPlate returns the id of the vehicle with the given license
// plate. A known plate returns the existing row instead of inserting a second
// one; a new plate inserts a row. The read locks the row (SELECT ... FOR UPDATE)
// so concurrent requests do not create duplicates, and the unique constraint on
// license_plate is the final guard when two requests insert at once.
func (s *Store) GetOrCreateVehicleByPlate(ctx context.Context, v domain.Vehicle) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin vehicle upsert: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	err = tx.QueryRow(ctx,
		`SELECT id FROM vehicles WHERE license_plate = $1 FOR UPDATE`,
		v.LicensePlate,
	).Scan(&id)
	switch {
	case err == nil:
		// Known plate: reuse the existing vehicle row, never a second one.
	case errors.Is(err, pgx.ErrNoRows):
		err = tx.QueryRow(ctx,
			`INSERT INTO vehicles (license_plate, brand, model, mileage)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (license_plate) DO NOTHING
			 RETURNING id`,
			v.LicensePlate, v.Brand, v.Model, v.Mileage,
		).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			// A concurrent request inserted the same plate first; read its row.
			if err := tx.QueryRow(ctx,
				`SELECT id FROM vehicles WHERE license_plate = $1`,
				v.LicensePlate,
			).Scan(&id); err != nil {
				return 0, fmt.Errorf("select vehicle after conflict: %w", err)
			}
		} else if err != nil {
			return 0, fmt.Errorf("insert vehicle: %w", err)
		}
	default:
		return 0, fmt.Errorf("select vehicle: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit vehicle upsert: %w", err)
	}
	return id, nil
}

// CreateOrder inserts a new order in status 'angefragt' and returns its id, its
// generated unique order number and its creation timestamp. The status is bound
// as a parameter; no request value is ever concatenated into SQL.
func (s *Store) CreateOrder(
	ctx context.Context,
	customerID, vehicleID int64,
	requestedDate time.Time,
	problemDescription string,
) (int64, string, time.Time, error) {
	orderNumber, err := newOrderNumber()
	if err != nil {
		return 0, "", time.Time{}, fmt.Errorf("generate order number: %w", err)
	}

	var id int64
	var createdAt time.Time
	err = s.pool.QueryRow(ctx,
		`INSERT INTO orders (order_number, customer_id, vehicle_id, status, requested_date, problem_description)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		orderNumber, customerID, vehicleID, string(domain.StatusAngefragt), requestedDate, problemDescription,
	).Scan(&id, &createdAt)
	if err != nil {
		return 0, "", time.Time{}, fmt.Errorf("insert order: %w", err)
	}
	return id, orderNumber, createdAt, nil
}

// ReadOrderDetail loads the shared OrderDetail representation of the order with
// the given order number. A newly created order has no items and no history, so
// both slices are returned empty (never null) rather than omitted.
func (s *Store) ReadOrderDetail(ctx context.Context, orderNumber string) (domain.OrderDetail, error) {
	var (
		detail        domain.OrderDetail
		status        string
		requestedDate time.Time
		createdAt     time.Time
		customer      domain.Customer
		vehicle       domain.Vehicle
	)
	err := s.pool.QueryRow(ctx,
		`SELECT o.order_number, o.status, o.requested_date, o.problem_description, o.created_at,
		        c.name, c.email, c.phone,
		        v.license_plate, v.brand, v.model, v.mileage
		   FROM orders o
		   JOIN customers c ON c.id = o.customer_id
		   JOIN vehicles  v ON v.id = o.vehicle_id
		  WHERE o.order_number = $1`,
		orderNumber,
	).Scan(
		&detail.OrderNumber, &status, &requestedDate, &detail.ProblemDescription, &createdAt,
		&customer.Name, &customer.Email, &customer.Phone,
		&vehicle.LicensePlate, &vehicle.Brand, &vehicle.Model, &vehicle.Mileage,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.OrderDetail{}, fmt.Errorf("order %s: %w", orderNumber, pgx.ErrNoRows)
		}
		return domain.OrderDetail{}, fmt.Errorf("select order: %w", err)
	}

	detail.Status = domain.OrderStatus(status)
	detail.RequestedDate = requestedDate.Format("2006-01-02")
	detail.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	detail.Customer = customer
	detail.Vehicle = vehicle
	detail.Items = []domain.OrderItem{}
	detail.History = []domain.StatusEvent{}
	return detail, nil
}

// newOrderNumber generates an order number that is unique in practice: the
// "AU-" prefix, the current UTC date and 8 random bytes rendered as hex.
func newOrderNumber() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "AU-" + time.Now().UTC().Format("20060102") + "-" + strings.ToUpper(hex.EncodeToString(buf)), nil
}
