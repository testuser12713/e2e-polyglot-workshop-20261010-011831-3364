package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"workshop/internal/domain"
)

// ErrOrderNotFound is returned when an order cannot be found by its number.
var ErrOrderNotFound = errors.New("order not found")

// OrderFilter narrows the workshop order list. A nil Status and an empty
// LicensePlate each mean "do not filter on this field".
type OrderFilter struct {
	Status       *domain.OrderStatus
	LicensePlate string
}

// orderSelect is the projection shared by the list and the detail query. It
// reads one order together with its customer and vehicle.
const orderSelect = `
	SELECT
		o.id,
		o.order_number,
		o.status,
		o.requested_date,
		o.problem_description,
		o.created_at,
		c.name,
		c.email,
		c.phone,
		v.license_plate,
		v.brand,
		v.model,
		v.mileage
	FROM orders o
	JOIN customers c ON c.id = o.customer_id
	JOIN vehicles v ON v.id = o.vehicle_id`

// ListOrders returns the orders matching filter, each as a full OrderDetail
// with its items and status history. The status is compared exactly; the
// license plate is matched as a case-insensitive substring. The result is
// ordered by created_at (and id as a stable tiebreaker). Every value is passed
// to PostgreSQL as a bound parameter.
func (s *Store) ListOrders(ctx context.Context, filter OrderFilter) ([]domain.OrderDetail, error) {
	var status *string
	if filter.Status != nil {
		value := string(*filter.Status)
		status = &value
	}

	query := orderSelect + `
	WHERE ($1::text IS NULL OR o.status = $1::text)
	  AND ($2::text = '' OR v.license_plate ILIKE '%' || $2::text || '%')
	ORDER BY o.created_at, o.id`

	rows, err := s.pool.Query(ctx, query, status, filter.LicensePlate)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	type scannedOrder struct {
		id     int64
		detail domain.OrderDetail
	}
	var scanned []scannedOrder
	for rows.Next() {
		id, detail, err := scanOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		scanned = append(scanned, scannedOrder{id: id, detail: detail})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}

	orders := make([]domain.OrderDetail, 0, len(scanned))
	for _, entry := range scanned {
		if err := s.loadDetail(ctx, entry.id, &entry.detail); err != nil {
			return nil, err
		}
		orders = append(orders, entry.detail)
	}
	return orders, nil
}

// OrderForWorkshop returns the full OrderDetail for orderNumber, including its
// items and status history. It returns ErrOrderNotFound when no such order
// exists.
func (s *Store) OrderForWorkshop(ctx context.Context, orderNumber string) (domain.OrderDetail, error) {
	row := s.pool.QueryRow(ctx, orderSelect+`
	WHERE o.order_number = $1::text`, orderNumber)

	id, detail, err := scanOrder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.OrderDetail{}, ErrOrderNotFound
	}
	if err != nil {
		return domain.OrderDetail{}, fmt.Errorf("get order: %w", err)
	}
	if err := s.loadDetail(ctx, id, &detail); err != nil {
		return domain.OrderDetail{}, err
	}
	return detail, nil
}

// loadDetail fills the items and status history of an order in place.
func (s *Store) loadDetail(ctx context.Context, orderID int64, detail *domain.OrderDetail) error {
	items, err := s.loadItems(ctx, orderID)
	if err != nil {
		return err
	}
	history, err := s.loadHistory(ctx, orderID)
	if err != nil {
		return err
	}
	detail.Items = items
	detail.History = history
	return nil
}

// loadItems returns the items of an order in insertion order.
func (s *Store) loadItems(ctx context.Context, orderID int64) ([]domain.OrderItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, kind, description, hours::float8, quantity, unit_price_cents, total_cents
		FROM order_items
		WHERE order_id = $1
		ORDER BY id`, orderID)
	if err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}
	defer rows.Close()

	items := []domain.OrderItem{}
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(
			&item.ID,
			&item.Kind,
			&item.Description,
			&item.Hours,
			&item.Quantity,
			&item.UnitPriceCents,
			&item.TotalCents,
		); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list order items: %w", err)
	}
	return items, nil
}

// loadHistory returns the status events of an order in chronological order.
func (s *Store) loadHistory(ctx context.Context, orderID int64) ([]domain.StatusEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT from_status, to_status, changed_by, changed_at
		FROM order_status_events
		WHERE order_id = $1
		ORDER BY changed_at, id`, orderID)
	if err != nil {
		return nil, fmt.Errorf("list order history: %w", err)
	}
	defer rows.Close()

	history := []domain.StatusEvent{}
	for rows.Next() {
		var (
			from      *string
			to        domain.OrderStatus
			changedBy string
			changedAt time.Time
		)
		if err := rows.Scan(&from, &to, &changedBy, &changedAt); err != nil {
			return nil, fmt.Errorf("scan status event: %w", err)
		}
		event := domain.StatusEvent{
			ToStatus:  to,
			ChangedBy: changedBy,
			ChangedAt: formatTime(changedAt),
		}
		if from != nil {
			value := domain.OrderStatus(*from)
			event.FromStatus = &value
		}
		history = append(history, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list order history: %w", err)
	}
	return history, nil
}

// rowScanner is implemented by both pgx.Row and pgx.Rows, so scanOrder serves
// the list and the single-order query alike.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanOrder reads one joined order row into an id and an OrderDetail (without
// items and history).
func scanOrder(scanner rowScanner) (int64, domain.OrderDetail, error) {
	var (
		id            int64
		detail        domain.OrderDetail
		requestedDate time.Time
		createdAt     time.Time
	)
	err := scanner.Scan(
		&id,
		&detail.OrderNumber,
		&detail.Status,
		&requestedDate,
		&detail.ProblemDescription,
		&createdAt,
		&detail.Customer.Name,
		&detail.Customer.Email,
		&detail.Customer.Phone,
		&detail.Vehicle.LicensePlate,
		&detail.Vehicle.Brand,
		&detail.Vehicle.Model,
		&detail.Vehicle.Mileage,
	)
	if err != nil {
		return 0, domain.OrderDetail{}, err
	}
	detail.RequestedDate = requestedDate.Format("2006-01-02")
	detail.CreatedAt = formatTime(createdAt)
	return id, detail, nil
}

// formatTime renders a timestamp as RFC3339 in UTC.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
