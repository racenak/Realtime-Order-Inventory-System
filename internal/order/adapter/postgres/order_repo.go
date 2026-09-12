package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/database"
)

type orderRepository struct {
	db *sqlx.DB
}

func NewOrderRepository(db *sqlx.DB) domain.OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) DB() *sqlx.DB {
	return r.db
}

type orderRow struct {
	ID              string    `db:"id"`
	CustomerID      string    `db:"customer_id"`
	Status          string    `db:"status"`
	Currency        string    `db:"currency"`
	Subtotal        float64   `db:"subtotal"`
	DiscountAmount  float64   `db:"discount_amount"`
	ShippingFee     float64   `db:"shipping_fee"`
	TaxAmount       float64   `db:"tax_amount"`
	TotalAmount     float64   `db:"total_amount"`
	ShippingAddress string    `db:"shipping_address"`
	IdempotencyKey  *string   `db:"idempotency_key"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

func (r *orderRepository) Create(ctx context.Context, order *domain.Order) error {
	return r.CreateInTx(ctx, nil, order)
}

func (r *orderRepository) CreateInTx(ctx context.Context, tx *sqlx.Tx, order *domain.Order) error {
	shippingAddrJSON, err := json.Marshal(order.ShippingAddress)
	if err != nil {
		return fmt.Errorf("failed to marshal shipping address: %w", err)
	}

	query := `
		INSERT INTO orders (id, customer_id, status, currency, subtotal, discount_amount, shipping_fee, tax_amount, total_amount, shipping_address, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	var executor database.DBExecutor = r.db
	if tx != nil {
		executor = tx
	}

	_, err = executor.ExecContext(ctx, query,
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
		order.IdempotencyKey,
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

func (r *orderRepository) GetByIDempotencyKey(ctx context.Context, key string) (*domain.Order, error) {
	var row orderRow
	query := `SELECT * FROM orders WHERE idempotency_key = $1`

	err := r.db.GetContext(ctx, &row, query, key)
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
	var countQuery string
	var countArgs []interface{}

	if customerID != "" {
		countQuery = `SELECT COUNT(*) FROM orders WHERE customer_id = $1`
		countArgs = []interface{}{customerID}
	} else {
		countQuery = `SELECT COUNT(*) FROM orders`
		countArgs = nil
	}

	err := r.db.GetContext(ctx, &count, countQuery, countArgs...)
	if err != nil {
		return nil, 0, err
	}

	var rows []orderRow
	var query string
	var args []interface{}

	if customerID != "" {
		query = `SELECT * FROM orders WHERE customer_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		args = []interface{}{customerID, limit, offset}
	} else {
		query = `SELECT * FROM orders ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		args = []interface{}{limit, offset}
	}

	err = r.db.SelectContext(ctx, &rows, query, args...)
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
	return r.UpdateStatusInTx(ctx, nil, id, status)
}

func (r *orderRepository) UpdateStatusInTx(ctx context.Context, tx *sqlx.Tx, id string, status domain.OrderStatus) error {
	query := `UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3`

	var executor database.DBExecutor = r.db
	if tx != nil {
		executor = tx
	}

	_, err := executor.ExecContext(ctx, query, status, time.Now(), id)
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

type orderItemRow struct {
	ID             string    `db:"id"`
	OrderID        string    `db:"order_id"`
	ProductID      string    `db:"product_id"`
	SKU            string    `db:"sku"`
	ProductName    string    `db:"product_name"`
	Quantity       int       `db:"quantity"`
	UnitPrice      float64   `db:"unit_price"`
	DiscountAmount float64   `db:"discount_amount"`
	TotalAmount    float64   `db:"total_amount"`
	CreatedAt      time.Time `db:"created_at"`
}

func (r *orderItemRepository) Create(ctx context.Context, items []domain.OrderItem) error {
	return r.CreateInTx(ctx, nil, items)
}

func (r *orderItemRepository) CreateInTx(ctx context.Context, tx *sqlx.Tx, items []domain.OrderItem) error {
	query := `
		INSERT INTO order_items (id, order_id, product_id, sku, product_name, quantity, unit_price, discount_amount, total_amount, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	var executor database.DBExecutor = r.db
	if tx != nil {
		executor = tx
	}

	for _, item := range items {
		_, err := executor.ExecContext(ctx, query,
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
	var rows []orderItemRow
	query := `SELECT * FROM order_items WHERE order_id = $1`
	err := r.db.SelectContext(ctx, &rows, query, orderID)
	if err != nil {
		return nil, err
	}

	items := make([]domain.OrderItem, len(rows))
	for i, row := range rows {
		items[i] = domain.OrderItem{
			ID:          row.ID,
			OrderID:     row.OrderID,
			ProductID:   row.ProductID,
			SKU:         row.SKU,
			ProductName: row.ProductName,
			Quantity:    row.Quantity,
			UnitPrice:   row.UnitPrice,
			TotalPrice:  row.TotalAmount,
		}
	}

	return items, nil
}

type outboxEventRow struct {
	ID            string         `db:"id"`
	AggregateType string         `db:"aggregate_type"`
	AggregateID   string         `db:"aggregate_id"`
	EventType     string         `db:"event_type"`
	Payload       string         `db:"payload"`
	Status        string         `db:"status"`
	CreatedAt     string         `db:"created_at"`
	PublishedAt   sql.NullString `db:"published_at"`
}

type outboxRepository struct {
	db *sqlx.DB
}

func NewOutboxRepository(db *sqlx.DB) domain.OutboxRepository {
	return &outboxRepository{db: db}
}

func (r *outboxRepository) Create(ctx context.Context, event domain.OutboxEvent) error {
	return r.CreateInTx(ctx, nil, event)
}

func (r *outboxRepository) CreateInTx(ctx context.Context, tx *sqlx.Tx, event domain.OutboxEvent) error {
	query := `
		INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	var executor database.DBExecutor = r.db
	if tx != nil {
		executor = tx
	}

	_, err := executor.ExecContext(ctx, query,
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
	var rows []outboxEventRow
	query := `SELECT * FROM outbox_events WHERE status = 'PENDING' ORDER BY created_at ASC LIMIT $1`
	err := r.db.SelectContext(ctx, &rows, query, limit)
	if err != nil {
		return nil, err
	}

	events := make([]domain.OutboxEvent, len(rows))
	for i, row := range rows {
		events[i] = domain.OutboxEvent{
			ID:            row.ID,
			AggregateType: row.AggregateType,
			AggregateID:   row.AggregateID,
			EventType:     row.EventType,
			Payload:       row.Payload,
			Status:        row.Status,
			CreatedAt:     row.CreatedAt,
		}
	}

	return events, nil
}

func (r *outboxRepository) ClaimBatch(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, status, created_at::text
		FROM outbox_events
		WHERE status = 'PENDING'
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`

	var rows []outboxEventRow
	if err := tx.SelectContext(ctx, &rows, query, limit); err != nil {
		return nil, fmt.Errorf("failed to claim pending events: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil
	}

	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}

	updateQuery := `UPDATE outbox_events SET status = 'CLAIMED' WHERE id = ANY($1)`
	if _, err := tx.ExecContext(ctx, updateQuery, pq.Array(ids)); err != nil {
		return nil, fmt.Errorf("failed to mark events as claimed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit claim transaction: %w", err)
	}

	events := make([]domain.OutboxEvent, len(rows))
	for i, row := range rows {
		events[i] = domain.OutboxEvent{
			ID:            row.ID,
			AggregateType: row.AggregateType,
			AggregateID:   row.AggregateID,
			EventType:     row.EventType,
			Payload:       row.Payload,
			Status:        row.Status,
			CreatedAt:     row.CreatedAt,
		}
	}

	return events, nil
}

func (r *outboxRepository) MarkPublished(ctx context.Context, id string) error {
	query := `UPDATE outbox_events SET status = 'PUBLISHED', published_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now().Format(time.RFC3339), id)
	return err
}

func (r *outboxRepository) MarkFailed(ctx context.Context, id string) error {
	query := `UPDATE outbox_events SET status = 'FAILED', published_at = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, time.Now().Format(time.RFC3339), id)
	return err
}
