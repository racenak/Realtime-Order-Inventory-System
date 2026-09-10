package domain

import (
	"time"
)

type Warehouse struct {
	ID        string    `json:"id" db:"id"`
	Code      string    `json:"code" db:"code"`
	Name      string    `json:"name" db:"name"`
	Status    string    `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Inventory struct {
	ID               string    `json:"id" db:"id"`
	ProductID        string    `json:"product_id" db:"product_id"`
	SKU              string    `json:"sku" db:"sku"`
	WarehouseID      string    `json:"warehouse_id" db:"warehouse_id"`
	QuantityOnHand   int       `json:"quantity_on_hand" db:"quantity_on_hand"`
	QuantityReserved int       `json:"quantity_reserved" db:"quantity_reserved"`
	Version          int64     `json:"version" db:"version"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

func (i *Inventory) Available() int {
	return i.QuantityOnHand - i.QuantityReserved
}

type Reservation struct {
	ID          string     `json:"id" db:"id"`
	OrderID     string     `json:"order_id" db:"order_id"`
	OrderItemID string     `json:"order_item_id" db:"order_item_id"`
	ProductID   string     `json:"product_id" db:"product_id"`
	SKU         string     `json:"sku" db:"sku"`
	WarehouseID string     `json:"warehouse_id" db:"warehouse_id"`
	Quantity    int        `json:"quantity" db:"quantity"`
	Status      string     `json:"status" db:"status"`
	ExpiresAt   *time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

type MovementType string

const (
	MovementIn          MovementType = "in"
	MovementOut         MovementType = "out"
	MovementReserve     MovementType = "reserve"
	MovementRelease     MovementType = "release"
	MovementAdjustment  MovementType = "adjustment"
	MovementTransferIn  MovementType = "transfer_in"
	MovementTransferOut MovementType = "transfer_out"
)

type InventoryMovement struct {
	ID            string       `json:"id" db:"id"`
	ProductID     string       `json:"product_id" db:"product_id"`
	SKU           string       `json:"sku" db:"sku"`
	WarehouseID   string       `json:"warehouse_id" db:"warehouse_id"`
	MovementType  MovementType `json:"movement_type" db:"movement_type"`
	Quantity      int          `json:"quantity" db:"quantity"`
	ReferenceType string       `json:"reference_type" db:"reference_type"`
	ReferenceID   string       `json:"reference_id" db:"reference_id"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
}
