package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"workshop/internal/domain"
)

// Errors of the order items area. They are kept separate from the status
// area's sentinels so both areas can live side by side.
var (
	// ErrItemOrderNotFound is returned when no order carries the given number.
	ErrItemOrderNotFound = errors.New("order not found")

	// ErrOrderItemNotFound is returned when the addressed item does not exist
	// on that order.
	ErrOrderItemNotFound = errors.New("order item not found")

	// ErrOrderAbgeholt is returned when an item is added to or changed on an
	// order that has already been picked up.
	ErrOrderAbgeholt = errors.New("order already picked up")
)

// itemQueryer is the subset of pgx used to read an order detail inside a
// transaction. pgx.Tx satisfies it.
type itemQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// AddOrderItem inserts a new position into orderNumber and returns the fresh
// OrderDetail including it. The line total is hours * hourly rate for a
// "labor" item and quantity * unit_price_cents for a "part" item, in whole
// cents. An order that is already 'abgeholt' rejects the change.
func (s *Store) AddOrderItem(ctx context.Context, orderNumber string, req domain.ItemRequest) (*domain.OrderDetail, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin add order item: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	orderID, err := lockOrderForItem(ctx, tx, orderNumber)
	if err != nil {
		return nil, err
	}

	total := ItemTotalCents(req)
	if _, err := tx.Exec(ctx, `
		INSERT INTO order_items (order_id, kind, description, hours, quantity, unit_price_cents, total_cents)
		VALUES ($1, $2, $3, $4::float8, $5, $6, $7)`,
		orderID, req.Kind, req.Description, req.Hours, req.Quantity, req.UnitPriceCents, total,
	); err != nil {
		return nil, fmt.Errorf("insert order item: %w", err)
	}

	detail, err := loadItemOrderDetail(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit add order item: %w", err)
	}
	return detail, nil
}

// UpdateOrderItem replaces the addressed position of orderNumber and returns
// the fresh OrderDetail. The total is recomputed the same way as on creation.
// An unknown order or item, or an order that is already 'abgeholt', rejects
// the change.
func (s *Store) UpdateOrderItem(ctx context.Context, orderNumber string, itemID int, req domain.ItemRequest) (*domain.OrderDetail, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin update order item: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	orderID, err := lockOrderForItem(ctx, tx, orderNumber)
	if err != nil {
		return nil, err
	}

	total := ItemTotalCents(req)
	tag, err := tx.Exec(ctx, `
		UPDATE order_items
		SET kind = $1, description = $2, hours = $3::float8, quantity = $4,
		    unit_price_cents = $5, total_cents = $6
		WHERE id = $7 AND order_id = $8`,
		req.Kind, req.Description, req.Hours, req.Quantity, req.UnitPriceCents, total, itemID, orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("update order item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrOrderItemNotFound
	}

	detail, err := loadItemOrderDetail(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit update order item: %w", err)
	}
	return detail, nil
}

// ItemTotalCents computes a position's line total in whole cents: hours times
// the hourly rate for a "labor" item, quantity times the unit price for a
// "part" item. A request with missing fields yields 0; handlers reject those
// before they reach the store.
func ItemTotalCents(req domain.ItemRequest) int {
	switch req.Kind {
	case "labor":
		if req.Hours == nil || req.UnitPriceCents == nil {
			return 0
		}
		return int(math.Round(*req.Hours * float64(*req.UnitPriceCents)))
	case "part":
		if req.Quantity == nil || req.UnitPriceCents == nil {
			return 0
		}
		return *req.Quantity * *req.UnitPriceCents
	default:
		return 0
	}
}

// lockOrderForItem locks the order row and returns its id. It rejects an order
// that is already 'abgeholt', so no item is ever added to a finished job.
func lockOrderForItem(ctx context.Context, tx pgx.Tx, orderNumber string) (int, error) {
	var (
		orderID int
		status  string
	)
	err := tx.QueryRow(ctx,
		`SELECT id, status FROM orders WHERE order_number = $1 FOR UPDATE`,
		orderNumber,
	).Scan(&orderID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrItemOrderNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("load order for item change: %w", err)
	}
	if domain.OrderStatus(status) == domain.StatusAbgeholt {
		return 0, ErrOrderAbgeholt
	}
	return orderID, nil
}

// loadItemOrderDetail reads the full OrderDetail of orderID using q. It is the
// items area's own reader so this file is self-contained; the status area
// carries an equivalent helper.
func loadItemOrderDetail(ctx context.Context, q itemQueryer, orderID int) (*domain.OrderDetail, error) {
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
		return nil, ErrItemOrderNotFound
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
