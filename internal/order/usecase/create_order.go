package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
)

type createOrderUseCase struct {
	orderRepo     domain.OrderRepository
	orderItemRepo domain.OrderItemRepository
	outboxRepo    domain.OutboxRepository
}

func NewCreateOrderUseCase(
	orderRepo domain.OrderRepository,
	orderItemRepo domain.OrderItemRepository,
	outboxRepo domain.OutboxRepository,
) OrderUseCase {
	return &createOrderUseCase{
		orderRepo:     orderRepo,
		orderItemRepo: orderItemRepo,
		outboxRepo:    outboxRepo,
	}
}

func (uc *createOrderUseCase) CreateOrder(ctx context.Context, req CreateOrderRequest) (*domain.Order, error) {
	if len(req.Items) == 0 {
		return nil, domain.ErrEmptyCart
	}

	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, domain.ErrInvalidQuantity
		}
	}

	if req.IdempotencyKey != "" {
		existing, err := uc.orderRepo.GetByIDempotencyKey(ctx, req.IdempotencyKey)
		if err == nil && existing != nil {
			return existing, domain.ErrDuplicateIdempotencyKey
		}
	}

	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}

	order := &domain.Order{
		ID:         uuid.New().String(),
		CustomerID: req.CustomerID,
		Status:     domain.StatusPendingPayment,
		Currency:   currency,
		ShippingAddress: domain.Address{
			Street:  req.ShippingAddress.Street,
			City:    req.ShippingAddress.City,
			State:   req.ShippingAddress.State,
			Zip:     req.ShippingAddress.Zip,
			Country: req.ShippingAddress.Country,
		},
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	var orderItems []domain.OrderItem
	for _, item := range req.Items {
		orderItem := domain.OrderItem{
			ID:          uuid.New().String(),
			OrderID:     order.ID,
			ProductID:   item.ProductID,
			SKU:         item.SKU,
			ProductName: item.ProductName,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			TotalPrice:  float64(item.Quantity) * item.UnitPrice,
		}
		orderItems = append(orderItems, orderItem)
	}

	order.Items = orderItems
	order.CalculateTotal()

	eventPayload, _ := json.Marshal(map[string]interface{}{
		"order_id":    order.ID,
		"customer_id": order.CustomerID,
		"total":       order.TotalAmount,
		"items":       len(order.Items),
	})

	outboxEvent := domain.OutboxEvent{
		ID:            uuid.New().String(),
		AggregateType: "order",
		AggregateID:   order.ID,
		EventType:     "order.created",
		Payload:       string(eventPayload),
		Status:        "PENDING",
		CreatedAt:     time.Now().Format(time.RFC3339),
	}

	db := uc.orderRepo.DB()
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := uc.orderRepo.CreateInTx(ctx, tx, order); err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	if err := uc.orderItemRepo.CreateInTx(ctx, tx, orderItems); err != nil {
		return nil, fmt.Errorf("failed to create order items: %w", err)
	}

	if err := uc.outboxRepo.CreateInTx(ctx, tx, outboxEvent); err != nil {
		return nil, fmt.Errorf("failed to create outbox event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return order, nil
}

func (uc *createOrderUseCase) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	order, err := uc.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	items, err := uc.orderItemRepo.GetByOrderID(ctx, id)
	if err != nil {
		return nil, err
	}
	order.Items = items

	return order, nil
}

func (uc *createOrderUseCase) ListOrders(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, int, error) {
	return uc.orderRepo.List(ctx, customerID, limit, offset)
}

func (uc *createOrderUseCase) CancelOrder(ctx context.Context, id string, reason string) error {
	order, err := uc.orderRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !order.CanCancel() {
		return domain.ErrOrderNotCancellable
	}

	eventPayload, _ := json.Marshal(map[string]interface{}{
		"order_id":   order.ID,
		"old_status": order.Status,
		"new_status": domain.StatusCancelled,
		"reason":     reason,
	})

	outboxEvent := domain.OutboxEvent{
		ID:            uuid.New().String(),
		AggregateType: "order",
		AggregateID:   order.ID,
		EventType:     "order.cancelled",
		Payload:       string(eventPayload),
		Status:        "PENDING",
		CreatedAt:     time.Now().Format(time.RFC3339),
	}

	db := uc.orderRepo.DB()
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := uc.orderRepo.UpdateStatusInTx(ctx, tx, id, domain.StatusCancelled); err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	if err := uc.outboxRepo.CreateInTx(ctx, tx, outboxEvent); err != nil {
		return fmt.Errorf("failed to create outbox event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (uc *createOrderUseCase) ConfirmOrder(ctx context.Context, id string) error {
	order, err := uc.orderRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if order.Status != domain.StatusPendingPayment {
		return nil
	}

	eventPayload, _ := json.Marshal(map[string]interface{}{
		"order_id":   order.ID,
		"old_status": order.Status,
		"new_status": domain.StatusProcessing,
	})

	outboxEvent := domain.OutboxEvent{
		ID:            uuid.New().String(),
		AggregateType: "order",
		AggregateID:   order.ID,
		EventType:     "order.confirmed",
		Payload:       string(eventPayload),
		Status:        "PENDING",
		CreatedAt:     time.Now().Format(time.RFC3339),
	}

	db := uc.orderRepo.DB()
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := uc.orderRepo.UpdateStatusInTx(ctx, tx, id, domain.StatusProcessing); err != nil {
		return fmt.Errorf("failed to confirm order: %w", err)
	}

	if err := uc.outboxRepo.CreateInTx(ctx, tx, outboxEvent); err != nil {
		return fmt.Errorf("failed to create outbox event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
