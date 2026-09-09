package domain

import (
	"context"
)

type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, id string) (*Order, error)
	List(ctx context.Context, customerID string, limit, offset int) ([]*Order, int, error)
	UpdateStatus(ctx context.Context, id string, status OrderStatus) error
}

type OrderItemRepository interface {
	Create(ctx context.Context, items []OrderItem) error
	GetByOrderID(ctx context.Context, orderID string) ([]OrderItem, error)
}

type OutboxRepository interface {
	Create(ctx context.Context, event OutboxEvent) error
	GetPending(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkPublished(ctx context.Context, id string) error
}

type OutboxEvent struct {
	ID            string `json:"id" db:"id"`
	AggregateType string `json:"aggregate_type" db:"aggregate_type"`
	AggregateID   string `json:"aggregate_id" db:"aggregate_id"`
	EventType     string `json:"event_type" db:"event_type"`
	Payload       string `json:"payload" db:"payload"`
	Status        string `json:"status" db:"status"`
	CreatedAt     string `json:"created_at" db:"created_at"`
	PublishedAt   string `json:"published_at" db:"published_at"`
}
