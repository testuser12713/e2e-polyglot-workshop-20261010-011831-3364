package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// invoicesQueueKey is the Valkey list the worker reads completed orders from.
const invoicesQueueKey = "invoices:queue"

// completionMessage is the contract payload pushed for a completed order:
// {"order_id":int,"order_number":str}. The struct field tags emit exactly the
// shared JSON field names.
type completionMessage struct {
	OrderID     int    `json:"order_id"`
	OrderNumber string `json:"order_number"`
}

// Publisher writes messages onto the Valkey queue. The queue key is
// "invoices:queue" and the payload is {"order_id":int,"order_number":str}.
type Publisher struct {
	url    string
	client *redis.Client
	// err carries a configuration error (a malformed VALKEY_URL). New cannot
	// return an error without changing the shared constructor, so the failure
	// is reported by the first publish instead of being swallowed or panicking.
	err error
}

// New returns a Publisher for the given Valkey URL. The connection itself is
// established lazily by the client on the first command.
func New(url string) *Publisher {
	p := &Publisher{url: url}
	if url == "" {
		p.err = fmt.Errorf("VALKEY_URL is not set (see RUN.json)")
		return p
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		p.err = fmt.Errorf("parse valkey url: %w", err)
		return p
	}
	p.client = redis.NewClient(opts)
	return p
}

// URL returns the configured Valkey URL.
func (p *Publisher) URL() string {
	return p.url
}

// PublishOrderCompleted enqueues the message that makes the worker create the
// invoice for a completed order. The message is appended to the tail of the
// "invoices:queue" list; the consumer pops from the head.
func (p *Publisher) PublishOrderCompleted(ctx context.Context, orderID int, orderNumber string) error {
	if p.err != nil {
		return p.err
	}
	if p.client == nil {
		return fmt.Errorf("valkey client is not configured")
	}
	payload, err := json.Marshal(completionMessage{OrderID: orderID, OrderNumber: orderNumber})
	if err != nil {
		return fmt.Errorf("marshal order completion message: %w", err)
	}
	if err := p.client.RPush(ctx, invoicesQueueKey, payload).Err(); err != nil {
		return fmt.Errorf("push order completion message: %w", err)
	}
	return nil
}
