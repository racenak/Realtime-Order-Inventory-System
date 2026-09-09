package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
)

type orderRepository struct {
	db *sqlx.DB
}

func NewOrderRepository(db *sqlx.DB) domain.OrderRepository {
	return &orderRepository{db: db}
}

type orderRow struct {
	ID              string  `db:"id"`
	CustomerID      string  `db:"customer_id"`
	Status          string  `db:"status"`
	Currency        string  `db:"currency"`
	Subtotal        float64 `db:"subtotal"`
	DiscountAmount  float64 `db:"discount_amount"`
	ShippingFee     float64 `db:"shipping_fee"`
	TaxAmount       float64 `db:"tax_amount"`
	TotalAmount     float64 `db:"total_amount"`
	ShippingAddress string  `db:"shipping_address"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

func (r *orderRepository) Create(ctx context.Context, order *domain.Order) error {
	shippingAddrJSON, err := json.Marshal(order.ShippingAddress)
	if err != nil {
		return fmt.Errorf("failed to marshal shipping address: %w", err)
	}

	query := `
		INSERT INTO orders (id, customer_id, status, currency, subtotal, discount_amount, shipping_fee, tax_amount, total_amount, shipping_address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err = r.db.ExecContext(ctx, query,
		order.ID,
		order.CustomerID,
		order.Status,
		order.Currency,
		order.Subtotal,
		order.DiscountAmount,
		order.ShippingFee,
		order.TaxAmount,
		order.TotalAmount,
		shippingAddrJSON,
		order.CreatedAt,
		order.UpdatedAt,
	)

	return err
}

func (r *orderRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	var row orderRow
	query := `SELECT * FROM orders WHERE id = $1`

	err := r.db.GetContext(ctx, &row, query, id)
	if err == sql.ErrNoRows {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}

	return r.rowToOrder(&row), nil
}

func (r *orderRepository) List(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, int, error) {
	var count int
	countQuery := `SELECT COUNT(*) FROM orders WHERE customer_id = $1`
	err := r.db.GetContext(ctx, &count, countQuery, customerID)
	if err != nil {
		return nil, 0, err
	}

	var rows []orderRow
	query := `SELECT * FROM orders WHERE customer_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	err = r.db.SelectContext(ctx, &rows, query, customerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	orders := make([]*domain.Order, len(rows))
	for i, row := range rows {
		orders[i] = r.rowToOrder(&row)
	}

	return orders, count, nil
}

func (r *orderRepository) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus) error {
	query := `UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	return err
}

func (r *orderRepository) rowToOrder(row *orderRow) *domain.Order {
	var shippingAddr domain.Address
	json.Unmarshal([]byte(row.ShippingAddress), &shippingAddr)

	return &domain.Order{
		ID:              row.ID,
		CustomerID:      row.CustomerID,
		Status:          domain.OrderStatus(row.Status),
		Currency:        row.Currency,
		Subtotal:        row.Subtotal,
		DiscountAmount:  row.DiscountAmount,
		ShippingFee:     row.ShippingFee,
		TaxAmount:       row.TaxAmount,
		TotalAmount:     row.TotalAmount,
		ShippingAddress: shippingAddr,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

type orderItemRepository struct {
	db *sqlx.DB
}

func NewOrderItemRepository(db *sqlx.DB) domain.OrderItemRepository {
	return &orderItemRepository{db: db}
}

func (r *orderItemRepository) Create(ctx context.Context, items []domain.OrderItem) error {
	query := `
		INSERT INTO order_items (id, order_id, product_id, sku, product_name, quantity, unit_price, discount_amount, total_amount, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	for _, item := range items {
		_, err := r.db.ExecContext(ctx, query,
			item.ID,
			item.OrderID,
			item.ProductID,
			item.SKU,
			item.ProductName,
			item.Quantity,
			item.UnitPrice,
			0,
			item.TotalPrice,
			time.Now(),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *orderItemRepository) GetByOrderID(ctx context.Context, orderID string) ([]domain.OrderItem, error) {
	var items []domain.OrderItem
	query := `SELECT * FROM order_items WHERE order_id = $1`
	err := r.db.SelectContext(ctx, &items, query, orderID)
	return items, err
}

type outboxRepository struct {
	db *sqlx.DB
}

func NewOutboxRepository(db *sqlx.DB) domain.OutboxRepository {
	return &outboxRepository{db: db}
}

func (r *outboxRepository) Create(ctx context.Context, event domain.OutboxEvent) error {
	query := `
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.ExecContext(ctx, query,
		event.ID,
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.Payload,
		event.Status,
		event.CreatedAt,
	)

	return err
}

func (r *outboxRepository) GetPending(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	var events []domain.OutboxEvent
	query := `SELECT * FROM outbox_events WHERE status = 'PENDING' ORDER BY created_at ASC LIMIT $1`
	err := r.db.SelectContext(ctx, &events, query, limit)
	return events, err
}

func (r *outboxRepository) MarkPublished(ctx context.Context, id string) error {
	query := `UPDATE outbox_events SET status = 'PUBLISHED', published_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now(), id)
	return err
}
