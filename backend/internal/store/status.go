package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"workshop/internal/domain"
)

// ErrOrderNotFound is returned when no order carries the given order number.
var ErrOrderNotFound = errors.New("order not found")

// ErrInvalidTransition is returned when the lifecycle does not allow the
// requested status change.
var ErrInvalidTransition = errors.New("invalid status transition")

// PublishOrderCompletedFunc is invoked inside ApplyStatusTransition's
// transaction with the id and number of the order that just changed. A returned
// error rolls the whole transaction back, so an order never ends up in 'fertig'
// without its queue message.
type PublishOrderCompletedFunc func(ctx context.Context, orderID int, orderNumber string) error

// queryer is the subset of pgx used to read an order detail; both pgx.Tx and
// *pgxpool.Pool satisfy it.
type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ApplyStatusTransition moves the order with orderNumber to the status to in a
// single transaction: it locks the row, verifies the transition against the
// lifecycle, updates the status, records a status event with the acting
// employee's name and the timestamp, optionally publishes the completion
// message and returns the fresh OrderDetail with its full history. Any error
// leaves the database unchanged.
func (s *Store) ApplyStatusTransition(
	ctx context.Context,
	orderNumber string,
	to domain.OrderStatus,
	changedBy string,
	publish PublishOrderCompletedFunc,
) (*domain.OrderDetail, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin status transition: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		orderID int
		current string
	)
	err = tx.QueryRow(ctx,
		`SELECT id, status FROM orders WHERE order_number = $1 FOR UPDATE`,
		orderNumber,
	).Scan(&orderID, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load order for status transition: %w", err)
	}

	if !domain.CanTransition(domain.OrderStatus(current), to) {
		return nil, ErrInvalidTransition
	}

	if _, err := tx.Exec(ctx,
		`UPDATE orders SET status = $1 WHERE id = $2`,
		string(to), orderID,
	); err != nil {
		return nil, fmt.Errorf("update order status: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO order_status_events (order_id, from_status, to_status, changed_by)
		 VALUES ($1, $2, $3, $4)`,
		orderID, current, string(to), changedBy,
	); err != nil {
		return nil, fmt.Errorf("insert status event: %w", err)
	}

	detail, err := loadOrderDetail(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}

	// Publish last, just before the commit: a failing push rolls the
	// transaction back, so an order never reaches 'fertig' without its
	// message.
	if publish != nil {
		if err := publish(ctx, orderID, orderNumber); err != nil {
			return nil, fmt.Errorf("publish order completion: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit status transition: %w", err)
	}
	return detail, nil
}

// loadOrderDetail reads the full OrderDetail of orderID using q, which may be
// the running transaction so the just-recorded status and event are included.
func loadOrderDetail(ctx context.Context, q queryer, orderID int) (*domain.OrderDetail, error) {
	var (
		detail        domain.OrderDetail
		status        string
		requestedDate string
		createdAt     time.Time
	)
	err := q.QueryRow(ctx, `
		SELECT o.order_number, o.status, to_char(o.requested_date, 'YYYY-MM-DD'),
		       o.problem_description, o.created_at,
		       c.name, c.email, c.phone,
		       v.license_plate, v.brand, v.model, v.mileage
		FROM orders o
		JOIN customers c ON c.id = o.customer_id
		JOIN vehicles v ON v.id = o.vehicle_id
		WHERE o.id = $1`,
		orderID,
	).Scan(
		&detail.OrderNumber, &status, &requestedDate,
		&detail.ProblemDescription, &createdAt,
		&detail.Customer.Name, &detail.Customer.Email, &detail.Customer.Phone,
		&detail.Vehicle.LicensePlate, &detail.Vehicle.Brand, &detail.Vehicle.Model, &detail.Vehicle.Mileage,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load order detail: %w", err)
	}
	detail.Status = domain.OrderStatus(status)
	detail.RequestedDate = requestedDate
	detail.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	detail.Items = []domain.OrderItem{}
	detail.History = []domain.StatusEvent{}

	itemRows, err := q.Query(ctx, `
		SELECT id, kind, description, hours::float8, quantity, unit_price_cents, total_cents
		FROM order_items
		WHERE order_id = $1
		ORDER BY id`,
		orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("load order items: %w", err)
	}
	for itemRows.Next() {
		var item domain.OrderItem
		if err := itemRows.Scan(
			&item.ID, &item.Kind, &item.Description,
			&item.Hours, &item.Quantity, &item.UnitPriceCents, &item.TotalCents,
		); err != nil {
			itemRows.Close()
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		detail.Items = append(detail.Items, item)
	}
	itemRows.Close()
	if err := itemRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order items: %w", err)
	}

	eventRows, err := q.Query(ctx, `
		SELECT from_status, to_status, changed_by, changed_at
		FROM order_status_events
		WHERE order_id = $1
		ORDER BY changed_at, id`,
		orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("load status history: %w", err)
	}
	for eventRows.Next() {
		var (
			event      domain.StatusEvent
			fromStatus *string
			toStatus   string
			changedAt  time.Time
		)
		if err := eventRows.Scan(&fromStatus, &toStatus, &event.ChangedBy, &changedAt); err != nil {
			eventRows.Close()
			return nil, fmt.Errorf("scan status event: %w", err)
		}
		if fromStatus != nil {
			from := domain.OrderStatus(*fromStatus)
			event.FromStatus = &from
		}
		event.ToStatus = domain.OrderStatus(toStatus)
		event.ChangedAt = changedAt.UTC().Format(time.RFC3339)
		detail.History = append(detail.History, event)
	}
	eventRows.Close()
	if err := eventRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate status history: %w", err)
	}

	return &detail, nil
}
