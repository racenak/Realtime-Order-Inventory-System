package mocks

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/domain"
)

type MockMovementRepository struct {
	CreateFn       func(ctx context.Context, movement *domain.InventoryMovement) error
	CreateInTxFn   func(ctx context.Context, tx *sqlx.Tx, movement *domain.InventoryMovement) error
	GetByProductIDFn func(ctx context.Context, productID string) ([]*domain.InventoryMovement, error)
}

func (m *MockMovementRepository) Create(ctx context.Context, movement *domain.InventoryMovement) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, movement)
	}
	return nil
}

func (m *MockMovementRepository) CreateInTx(ctx context.Context, tx *sqlx.Tx, movement *domain.InventoryMovement) error {
	if m.CreateInTxFn != nil {
		return m.CreateInTxFn(ctx, tx, movement)
	}
	return nil
}

func (m *MockMovementRepository) GetByProductID(ctx context.Context, productID string) ([]*domain.InventoryMovement, error) {
	if m.GetByProductIDFn != nil {
		return m.GetByProductIDFn(ctx, productID)
	}
	return nil, nil
}
