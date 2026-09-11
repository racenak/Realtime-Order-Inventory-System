package mocks

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/domain"
)

type MockInventoryRepository struct {
	CreateFn                     func(ctx context.Context, inventory *domain.Inventory) error
	CreateInTxFn                 func(ctx context.Context, tx *sqlx.Tx, inventory *domain.Inventory) error
	GetByProductAndWarehouseFn   func(ctx context.Context, productID, warehouseID string) (*domain.Inventory, error)
	GetByProductIDFn             func(ctx context.Context, productID string) ([]*domain.Inventory, error)
	UpdateStockFn                func(ctx context.Context, productID, warehouseID string, quantityChange int, expectedVersion int64) error
	ReserveQuantityFn            func(ctx context.Context, productID, warehouseID string, quantity int, expectedVersion int64) error
	ReleaseQuantityFn            func(ctx context.Context, productID, warehouseID string, quantity int) error
	GetWarehouseByCodeFn         func(ctx context.Context, code string) (*domain.Warehouse, error)
	DBFn                         func() *sqlx.DB
}

func (m *MockInventoryRepository) Create(ctx context.Context, inventory *domain.Inventory) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, inventory)
	}
	return nil
}

func (m *MockInventoryRepository) CreateInTx(ctx context.Context, tx *sqlx.Tx, inventory *domain.Inventory) error {
	if m.CreateInTxFn != nil {
		return m.CreateInTxFn(ctx, tx, inventory)
	}
	return nil
}

func (m *MockInventoryRepository) GetByProductAndWarehouse(ctx context.Context, productID, warehouseID string) (*domain.Inventory, error) {
	if m.GetByProductAndWarehouseFn != nil {
		return m.GetByProductAndWarehouseFn(ctx, productID, warehouseID)
	}
	return nil, domain.ErrInventoryNotFound
}

func (m *MockInventoryRepository) GetByProductID(ctx context.Context, productID string) ([]*domain.Inventory, error) {
	if m.GetByProductIDFn != nil {
		return m.GetByProductIDFn(ctx, productID)
	}
	return nil, nil
}

func (m *MockInventoryRepository) UpdateStock(ctx context.Context, productID, warehouseID string, quantityChange int, expectedVersion int64) error {
	if m.UpdateStockFn != nil {
		return m.UpdateStockFn(ctx, productID, warehouseID, quantityChange, expectedVersion)
	}
	return nil
}

func (m *MockInventoryRepository) ReserveQuantity(ctx context.Context, productID, warehouseID string, quantity int, expectedVersion int64) error {
	if m.ReserveQuantityFn != nil {
		return m.ReserveQuantityFn(ctx, productID, warehouseID, quantity, expectedVersion)
	}
	return nil
}

func (m *MockInventoryRepository) ReleaseQuantity(ctx context.Context, productID, warehouseID string, quantity int) error {
	if m.ReleaseQuantityFn != nil {
		return m.ReleaseQuantityFn(ctx, productID, warehouseID, quantity)
	}
	return nil
}

func (m *MockInventoryRepository) GetWarehouseByCode(ctx context.Context, code string) (*domain.Warehouse, error) {
	if m.GetWarehouseByCodeFn != nil {
		return m.GetWarehouseByCodeFn(ctx, code)
	}
	return nil, domain.ErrInventoryNotFound
}

func (m *MockInventoryRepository) DB() *sqlx.DB {
	if m.DBFn != nil {
		return m.DBFn()
	}
	return nil
}
