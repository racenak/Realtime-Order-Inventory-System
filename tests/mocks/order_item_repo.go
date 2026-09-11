package mocks

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
)

type MockOrderItemRepository struct {
	CreateFn       func(ctx context.Context, items []domain.OrderItem) error
	CreateInTxFn   func(ctx context.Context, tx *sqlx.Tx, items []domain.OrderItem) error
	GetByOrderIDFn func(ctx context.Context, orderID string) ([]domain.OrderItem, error)
}

func (m *MockOrderItemRepository) Create(ctx context.Context, items []domain.OrderItem) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, items)
	}
	return nil
}

func (m *MockOrderItemRepository) CreateInTx(ctx context.Context, tx *sqlx.Tx, items []domain.OrderItem) error {
	if m.CreateInTxFn != nil {
		return m.CreateInTxFn(ctx, tx, items)
	}
	return nil
}

func (m *MockOrderItemRepository) GetByOrderID(ctx context.Context, orderID string) ([]domain.OrderItem, error) {
	if m.GetByOrderIDFn != nil {
		return m.GetByOrderIDFn(ctx, orderID)
	}
	return nil, nil
}
