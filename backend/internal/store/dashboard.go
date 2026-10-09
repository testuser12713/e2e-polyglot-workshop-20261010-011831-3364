package store

import (
	"context"
	"fmt"
	"time"

	"workshop/internal/domain"
)

// DashboardStats returns the three figures of the workshop dashboard:
//
//   - OpenOrders: orders whose status is neither "fertig" nor "abgeholt".
//   - CompletedToday: orders that reached "fertig" during the local day of now,
//     read from order_status_events.changed_at.
//   - RevenueMonthCents: the summed gross_cents of invoices issued in the local
//     month of now.
//
// now is the reference instant, so the caller (and the tests) control the day
// and month boundaries. Every statement is parametrized.
func (s *Store) DashboardStats(ctx context.Context, now time.Time) (domain.DashboardResponse, error) {
	local := now.Local()
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)
	monthStart := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, local.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)

	var stats domain.DashboardResponse

	var openOrders int64
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM orders WHERE status NOT IN ($1, $2)`,
		string(domain.StatusFertig), string(domain.StatusAbgeholt),
	).Scan(&openOrders); err != nil {
		return domain.DashboardResponse{}, fmt.Errorf("count open orders: %w", err)
	}

	var completedToday int64
	if err := s.pool.QueryRow(ctx,
		`SELECT count(DISTINCT order_id) FROM order_status_events
		 WHERE to_status = $1 AND changed_at >= $2 AND changed_at < $3`,
		string(domain.StatusFertig), dayStart, dayEnd,
	).Scan(&completedToday); err != nil {
		return domain.DashboardResponse{}, fmt.Errorf("count orders completed today: %w", err)
	}

	var revenueMonth int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(sum(gross_cents), 0) FROM invoices
		 WHERE issued_at >= $1 AND issued_at < $2`,
		monthStart, monthEnd,
	).Scan(&revenueMonth); err != nil {
		return domain.DashboardResponse{}, fmt.Errorf("sum revenue of current month: %w", err)
	}

	stats.OpenOrders = int(openOrders)
	stats.CompletedToday = int(completedToday)
	stats.RevenueMonthCents = int(revenueMonth)
	return stats, nil
}
