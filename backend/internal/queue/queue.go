package queue

import "context"

// Publisher writes messages onto the Valkey queue. The queue key is
// "invoices:queue" and the payload is {"order_id":int,"order_number":str}.
type Publisher struct {
	url string
}

// New returns a Publisher for the given Valkey URL.
func New(url string) *Publisher {
	return &Publisher{url: url}
}

// URL returns the configured Valkey URL.
func (p *Publisher) URL() string {
	return p.url
}

// PublishOrderCompleted enqueues the message that makes the worker create the
// invoice for a completed order.
//
// Skeleton stub: the order status lifecycle ticket implements this.
func (p *Publisher) PublishOrderCompleted(ctx context.Context, orderID int, orderNumber string) error {
	return nil
}
