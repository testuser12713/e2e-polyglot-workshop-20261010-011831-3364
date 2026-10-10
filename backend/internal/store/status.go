package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"workshop/internal/domain"
)

// ErrInvalidTransition is returned when the order lifecycle does not allow the
// requested status change. It is kept separate from ErrNotFound (defined in
// workshop_orders.go) so the handler can answer 404 and 409 distinctly.
var ErrInvalidTransition = errors.New("invalid status transition")

// PublishOrderCompletedFunc enqueues the completion message for an order. The
// status area accepts it as an optional callback and invokes it inside the
// transition's transaction: when it fails, the whole transaction is rolled
// back, so an order never ends up in 'fertig' without its queue message.
type PublishOrderCompletedFunc func(ctx context.Context, orderID int, orderNumber string) error

// ApplyStatusTransition moves the order with orderNumber one step along the
// lifecycle to the status to, in a single parametrized transaction: it locks
// the order row, verifies the transition against domain.OrderStatusOrder,
// updates orders.status and inserts the order_status_events row with the
// acting employee's display name and the current timestamp. It then returns the
// fresh OrderDetail together with its history, reusing the customer order
// loaders.
//
// The named four-argument form (through changedBy) is the contract callers use;
// passing no callback never publishes. An optional publisher is invoked inside
// the transaction when the transition reaches 'fertig', so a failed enqueue
// rolls the transition back. It returns ErrNotFound for an unknown order and
// ErrInvalidTransition for a step the table forbids; both leave the database
// unchanged.
func (s *Store) ApplyStatusTransition(
	ctx context.Context,
	orderNumber string,
	to domain.OrderStatus,
	changedBy string,
	publish ...PublishOrderCompletedFunc,
) (*domain.OrderDetail, error) {
	var publisher PublishOrderCompletedFunc
	if len(publish) > 0 {
		publisher = publish[0]
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin status transition: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		orderID      int
		current      string
		licensePlate string
	)
	err = tx.QueryRow(ctx,
		`SELECT o.id, o.status, v.license_plate
		 FROM orders o
		 JOIN vehicles v ON v.id = o.vehicle_id
		 WHERE o.order_number = $1
		 FOR UPDATE OF o`,
		orderNumber,
	).Scan(&orderID, &current, &licensePlate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
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
		`INSERT INTO order_status_events (order_id, from_status, to_status, changed_by, changed_at)
		 VALUES ($1, $2, $3, $4, now())`,
		orderID, current, string(to), changedBy,
	); err != nil {
		return nil, fmt.Errorf("insert status event: %w", err)
	}

	// Publish before the commit: a failing enqueue rolls the transition back,
	// so no order sits in 'fertig' without its message.
	if publisher != nil {
		if err := publisher(ctx, orderID, orderNumber); err != nil {
			return nil, fmt.Errorf("publish order completion: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit status transition: %w", err)
	}

	// The transaction is committed, so the fresh state is visible through the
	// pool. Reuse the customer order loaders for the detail and the history.
	_, detail, err := s.OrderForCustomer(ctx, orderNumber, licensePlate)
	if err != nil {
		return nil, err
	}
	history, err := s.OrderHistory(ctx, orderID)
	if err != nil {
		return nil, err
	}
	detail.History = history
	return detail, nil
}
