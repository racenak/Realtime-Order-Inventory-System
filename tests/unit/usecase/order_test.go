package usecase_test

import (
	"context"
	"testing"

	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/usecase"
	"github.com/racenak/Realtime-Order-Inventory-System/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOrder_EmptyCart(t *testing.T) {
	uc := usecase.NewCreateOrderUseCase(
		&mocks.MockOrderRepository{},
		&mocks.MockOrderItemRepository{},
		&mocks.MockOutboxRepository{},
	)

	_, err := uc.CreateOrder(context.Background(), usecase.CreateOrderRequest{
		CustomerID: "cust-1",
		Items:      []usecase.OrderItemRequest{},
	})

	assert.ErrorIs(t, err, domain.ErrEmptyCart)
}

func TestCreateOrder_InvalidQuantity(t *testing.T) {
	uc := usecase.NewCreateOrderUseCase(
		&mocks.MockOrderRepository{},
		&mocks.MockOrderItemRepository{},
		&mocks.MockOutboxRepository{},
	)

	tests := []struct {
		name     string
		quantity int
	}{
		{"zero quantity", 0},
		{"negative quantity", -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.CreateOrder(context.Background(), usecase.CreateOrderRequest{
				CustomerID: "cust-1",
				Items: []usecase.OrderItemRequest{
					{ProductID: "p1", SKU: "SKU-1", ProductName: "Widget", Quantity: tt.quantity, UnitPrice: 10.00},
				},
			})
			assert.ErrorIs(t, err, domain.ErrInvalidQuantity)
		})
	}
}

func TestCreateOrder_DuplicateIdempotencyKey(t *testing.T) {
	existingOrder := &domain.Order{
		ID:              "existing-order",
		CustomerID:      "cust-1",
		Status:          domain.StatusPendingPayment,
		IdempotencyKey:  "key-123",
	}

	orderRepo := &mocks.MockOrderRepository{
		GetByIDempotencyKeyFn: func(ctx context.Context, key string) (*domain.Order, error) {
			if key == "key-123" {
				return existingOrder, nil
			}
			return nil, domain.ErrOrderNotFound
		},
	}

	uc := usecase.NewCreateOrderUseCase(
		orderRepo,
		&mocks.MockOrderItemRepository{},
		&mocks.MockOutboxRepository{},
	)

	result, err := uc.CreateOrder(context.Background(), usecase.CreateOrderRequest{
		CustomerID: "cust-1",
		Items: []usecase.OrderItemRequest{
			{ProductID: "p1", SKU: "SKU-1", ProductName: "Widget", Quantity: 1, UnitPrice: 10.00},
		},
		IdempotencyKey: "key-123",
	})

	assert.ErrorIs(t, err, domain.ErrDuplicateIdempotencyKey)
	assert.Equal(t, existingOrder, result)
}

func TestGetOrder_NotFound(t *testing.T) {
	orderRepo := &mocks.MockOrderRepository{
		GetByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return nil, domain.ErrOrderNotFound
		},
	}

	uc := usecase.NewCreateOrderUseCase(
		orderRepo,
		&mocks.MockOrderItemRepository{},
		&mocks.MockOutboxRepository{},
	)

	_, err := uc.GetOrder(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, domain.ErrOrderNotFound)
}

func TestGetOrder_Success(t *testing.T) {
	expected := &domain.Order{ID: "order-1", CustomerID: "cust-1"}
	items := []domain.OrderItem{{ID: "item-1", OrderID: "order-1"}}

	orderRepo := &mocks.MockOrderRepository{
		GetByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return expected, nil
		},
	}
	itemRepo := &mocks.MockOrderItemRepository{
		GetByOrderIDFn: func(ctx context.Context, orderID string) ([]domain.OrderItem, error) {
			return items, nil
		},
	}

	uc := usecase.NewCreateOrderUseCase(orderRepo, itemRepo, &mocks.MockOutboxRepository{})

	result, err := uc.GetOrder(context.Background(), "order-1")
	require.NoError(t, err)
	assert.Equal(t, "order-1", result.ID)
	assert.Len(t, result.Items, 1)
}

func TestCancelOrder_NotCancellable(t *testing.T) {
	order := &domain.Order{
		ID:     "order-1",
		Status: domain.StatusShipped,
	}

	orderRepo := &mocks.MockOrderRepository{
		GetByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return order, nil
		},
	}

	uc := usecase.NewCreateOrderUseCase(orderRepo, &mocks.MockOrderItemRepository{}, &mocks.MockOutboxRepository{})

	err := uc.CancelOrder(context.Background(), "order-1", "too late")
	assert.ErrorIs(t, err, domain.ErrOrderNotCancellable)
}

func TestConfirmOrder_AlreadyProcessing(t *testing.T) {
	order := &domain.Order{
		ID:     "order-1",
		Status: domain.StatusProcessing,
	}

	orderRepo := &mocks.MockOrderRepository{
		GetByIDFn: func(ctx context.Context, id string) (*domain.Order, error) {
			return order, nil
		},
	}

	uc := usecase.NewCreateOrderUseCase(orderRepo, &mocks.MockOrderItemRepository{}, &mocks.MockOutboxRepository{})

	err := uc.ConfirmOrder(context.Background(), "order-1")
	assert.NoError(t, err) // Should be no-op, not an error
}

func TestListOrders_Success(t *testing.T) {
	orders := []*domain.Order{{ID: "o1"}, {ID: "o2"}}

	orderRepo := &mocks.MockOrderRepository{
		ListFn: func(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, int, error) {
			return orders, 2, nil
		},
	}

	uc := usecase.NewCreateOrderUseCase(orderRepo, &mocks.MockOrderItemRepository{}, &mocks.MockOutboxRepository{})

	result, total, err := uc.ListOrders(context.Background(), "cust-1", 10, 0)
	assert.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, result, 2)
}
