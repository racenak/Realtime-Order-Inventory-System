package domain

import (
	"context"
)

type InventoryRepository interface {
	GetByProductAndWarehouse(ctx context.Context, productID, warehouseID string) (*Inventory, error)
	GetByProductID(ctx context.Context, productID string) ([]*Inventory, error)
	UpdateStock(ctx context.Context, productID, warehouseID string, quantityChange int, expectedVersion int64) error
}

type ReservationRepository interface {
	Create(ctx context.Context, reservation *Reservation) error
	GetByID(ctx context.Context, id string) (*Reservation, error)
	GetByOrderID(ctx context.Context, orderID string) ([]*Reservation, error)
	UpdateStatus(ctx context.Context, id string, status string) error
}

type MovementRepository interface {
	Create(ctx context.Context, movement *InventoryMovement) error
	GetByProductID(ctx context.Context, productID string) ([]*InventoryMovement, error)
}
