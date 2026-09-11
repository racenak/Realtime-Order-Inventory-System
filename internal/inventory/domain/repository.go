package domain

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type InventoryRepository interface {
	Create(ctx context.Context, inventory *Inventory) error
	CreateInTx(ctx context.Context, tx *sqlx.Tx, inventory *Inventory) error
	GetByProductAndWarehouse(ctx context.Context, productID, warehouseID string) (*Inventory, error)
	GetByProductID(ctx context.Context, productID string) ([]*Inventory, error)
	UpdateStock(ctx context.Context, productID, warehouseID string, quantityChange int, expectedVersion int64) error
	ReserveQuantity(ctx context.Context, productID, warehouseID string, quantity int, expectedVersion int64) error
	ReleaseQuantity(ctx context.Context, productID, warehouseID string, quantity int) error
	GetWarehouseByCode(ctx context.Context, code string) (*Warehouse, error)
	DB() *sqlx.DB
}

type ReservationRepository interface {
	Create(ctx context.Context, reservation *Reservation) error
	CreateInTx(ctx context.Context, tx *sqlx.Tx, reservation *Reservation) error
	GetByID(ctx context.Context, id string) (*Reservation, error)
	GetByOrderID(ctx context.Context, orderID string) ([]*Reservation, error)
	UpdateStatus(ctx context.Context, id string, status string) error
}

type MovementRepository interface {
	Create(ctx context.Context, movement *InventoryMovement) error
	CreateInTx(ctx context.Context, tx *sqlx.Tx, movement *InventoryMovement) error
	GetByProductID(ctx context.Context, productID string) ([]*InventoryMovement, error)
}
