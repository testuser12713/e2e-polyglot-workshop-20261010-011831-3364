package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"workshop/internal/domain"
)

// ErrNotFound is returned by the store when the requested record does not
// exist. For the customer order lookup it also covers a license plate that
// does not match the order's vehicle.
var ErrNotFound = errors.New("store: not found")

// OrderForCustomer loads the order identified by orderNumber whose vehicle's
// license plate equals licensePlate. It joins the order with its customer and
// vehicle and loads the order items. The internal order ID is returned so the
// caller can request the order's history and invoice.
//
// It returns ErrNotFound when no order carries the number or when the number
// exists but its vehicle's plate differs from licensePlate. Both cases look
// identical to a customer, so the handler answers 404 for either.
func (s *Store) OrderForCustomer(ctx context.Context, orderNumber, licensePlate string) (int, *domain.OrderDetail, error) {
	const orderQuery = `
		SELECT o.id, o.order_number, o.status, o.requested_date, o.problem_description, o.created_at,
		       c.name, c.email, c.phone,
		       v.license_plate, v.brand, v.model, v.mileage
		FROM orders o
		JOIN customers c ON c.id = o.customer_id
		JOIN vehicles v ON v.id = o.vehicle_id
		WHERE o.order_number = $1 AND v.license_plate = $2`

	var (
		orderID     int
		detail      domain.OrderDetail
		requestedAt time.Time
		createdAt   time.Time
	)
	err := s.pool.QueryRow(ctx, orderQuery, orderNumber, licensePlate).Scan(
		&orderID,
		&detail.OrderNumber,
		&detail.Status,
		&requestedAt,
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
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, ErrNotFound
	}
	if err != nil {
		return 0, nil, fmt.Errorf("load order %s: %w", orderNumber, err)
	}

	detail.RequestedDate = requestedAt.Format("2006-01-02")
	detail.CreatedAt = createdAt.Format(time.RFC3339)
	detail.Items = []domain.OrderItem{}

	items, err := s.orderItems(ctx, orderID)
	if err != nil {
		return 0, nil, err
	}
	detail.Items = items

	return orderID, &detail, nil
}

// orderItems loads the positions of one order in insertion order.
func (s *Store) orderItems(ctx context.Context, orderID int) ([]domain.OrderItem, error) {
	const query = `
		SELECT id, kind, description, hours, quantity, unit_price_cents, total_cents
		FROM order_items
		WHERE order_id = $1
		ORDER BY id`

	rows, err := s.pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("load order items for order %d: %w", orderID, err)
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
			return nil, fmt.Errorf("scan order item for order %d: %w", orderID, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order items for order %d: %w", orderID, err)
	}
	return items, nil
}

// OrderHistory returns the status events of one order, oldest first. A known
// order with no recorded event yields an empty slice, never nil.
func (s *Store) OrderHistory(ctx context.Context, orderID int) ([]domain.StatusEvent, error) {
	const query = `
		SELECT from_status, to_status, changed_by, changed_at
		FROM order_status_events
		WHERE order_id = $1
		ORDER BY changed_at, id`

	rows, err := s.pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("load order history for order %d: %w", orderID, err)
	}
	defer rows.Close()

	events := []domain.StatusEvent{}
	for rows.Next() {
		var (
			fromStatus *string
			toStatus   string
			changedBy  string
			changedAt  time.Time
		)
		if err := rows.Scan(&fromStatus, &toStatus, &changedBy, &changedAt); err != nil {
			return nil, fmt.Errorf("scan status event for order %d: %w", orderID, err)
		}
		event := domain.StatusEvent{
			ToStatus:  domain.OrderStatus(toStatus),
			ChangedBy: changedBy,
			ChangedAt: changedAt.Format(time.RFC3339),
		}
		if fromStatus != nil {
			from := domain.OrderStatus(*fromStatus)
			event.FromStatus = &from
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate status events for order %d: %w", orderID, err)
	}
	return events, nil
}

// InvoiceForOrder loads the invoice of one order together with its lines. When
// the order has not been invoiced yet it returns (nil, nil): no invoice and no
// error, because "not invoiced yet" is an expected state and not a failure.
func (s *Store) InvoiceForOrder(ctx context.Context, orderID int) (*domain.Invoice, error) {
	const query = `
		SELECT i.id, i.invoice_number, o.order_number, i.issued_at,
		       i.labor_cents, i.parts_cents, i.net_cents, i.vat_cents, i.gross_cents
		FROM invoices i
		JOIN orders o ON o.id = i.order_id
		WHERE i.order_id = $1`

	var (
		invoiceID int
		invoice   domain.Invoice
		issuedAt  time.Time
	)
	err := s.pool.QueryRow(ctx, query, orderID).Scan(
		&invoiceID,
		&invoice.InvoiceNumber,
		&invoice.OrderNumber,
		&issuedAt,
		&invoice.LaborCents,
		&invoice.PartsCents,
		&invoice.NetCents,
		&invoice.VatCents,
		&invoice.GrossCents,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load invoice for order %d: %w", orderID, err)
	}
	invoice.IssuedAt = issuedAt.Format(time.RFC3339)

	lines, err := s.invoiceLines(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	invoice.Lines = lines
	return &invoice, nil
}

// invoiceLines loads the lines of one invoice in insertion order.
func (s *Store) invoiceLines(ctx context.Context, invoiceID int) ([]domain.InvoiceLine, error) {
	const query = `
		SELECT description, quantity, unit_price_cents, total_cents
		FROM invoice_lines
		WHERE invoice_id = $1
		ORDER BY id`

	rows, err := s.pool.Query(ctx, query, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("load invoice lines for invoice %d: %w", invoiceID, err)
	}
	defer rows.Close()

	lines := []domain.InvoiceLine{}
	for rows.Next() {
		var line domain.InvoiceLine
		if err := rows.Scan(&line.Description, &line.Quantity, &line.UnitPriceCents, &line.TotalCents); err != nil {
			return nil, fmt.Errorf("scan invoice line for invoice %d: %w", invoiceID, err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invoice lines for invoice %d: %w", invoiceID, err)
	}
	return lines, nil
}
