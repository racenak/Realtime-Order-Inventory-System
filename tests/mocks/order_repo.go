package mocks

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
)

type MockOrderRepository struct {
	CreateFn            func(ctx context.Context, order *domain.Order) error
	CreateInTxFn        func(ctx context.Context, tx *sqlx.Tx, order *domain.Order) error
	GetByIDFn           func(ctx context.Context, id string) (*domain.Order, error)
	GetByIDempotencyKeyFn func(ctx context.Context, key string) (*domain.Order, error)
	ListFn              func(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, int, error)
	UpdateStatusFn      func(ctx context.Context, id string, status domain.OrderStatus) error
	UpdateStatusInTxFn  func(ctx context.Context, tx *sqlx.Tx, id string, status domain.OrderStatus) error
	DBFn                func() *sqlx.DB
}

func (m *MockOrderRepository) Create(ctx context.Context, order *domain.Order) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, order)
	}
	return nil
}

func (m *MockOrderRepository) CreateInTx(ctx context.Context, tx *sqlx.Tx, order *domain.Order) error {
	if m.CreateInTxFn != nil {
		return m.CreateInTxFn(ctx, tx, order)
	}
	return nil
}

func (m *MockOrderRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, domain.ErrOrderNotFound
}

func (m *MockOrderRepository) GetByIDempotencyKey(ctx context.Context, key string) (*domain.Order, error) {
	if m.GetByIDempotencyKeyFn != nil {
		return m.GetByIDempotencyKeyFn(ctx, key)
	}
	return nil, domain.ErrOrderNotFound
}

func (m *MockOrderRepository) List(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, int, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, customerID, limit, offset)
	}
	return nil, 0, nil
}

func (m *MockOrderRepository) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus) error {
	if m.UpdateStatusFn != nil {
		return m.UpdateStatusFn(ctx, id, status)
	}
	return nil
}

func (m *MockOrderRepository) UpdateStatusInTx(ctx context.Context, tx *sqlx.Tx, id string, status domain.OrderStatus) error {
	if m.UpdateStatusInTxFn != nil {
		return m.UpdateStatusInTxFn(ctx, tx, id, status)
	}
	return nil
}

func (m *MockOrderRepository) DB() *sqlx.DB {
	if m.DBFn != nil {
		return m.DBFn()
	}
	return nil
}
