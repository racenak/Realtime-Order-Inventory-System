package usecase

import (
	"context"

	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/domain"
)

type InventoryUseCase interface {
	GetStock(ctx context.Context, productID string) (*StockResponse, error)
	ReserveStock(ctx context.Context, req ReserveStockRequest) (*domain.Reservation, error)
	ReleaseReservation(ctx context.Context, reservationID string) error
	UpdateStock(ctx context.Context, req UpdateStockRequest) error
}

type StockResponse struct {
	ProductID  string           `json:"product_id"`
	Warehouses []WarehouseStock `json:"warehouses"`
}

type WarehouseStock struct {
	WarehouseID string `json:"warehouse_id"`
	Quantity    int    `json:"quantity"`
	Reserved    int    `json:"reserved"`
	Available   int    `json:"available"`
}

type ReserveStockRequest struct {
	OrderID       string `json:"order_id"`
	ProductID     string `json:"product_id"`
	WarehouseID   string `json:"warehouse_id"`
	WarehouseCode string `json:"warehouse_code"`
	SKU           string `json:"sku"`
	Quantity      int    `json:"quantity"`
}

type UpdateStockRequest struct {
	ProductID     string `json:"product_id"`
	WarehouseID   string `json:"warehouse_id"`
	WarehouseCode string `json:"warehouse_code"`
	SKU           string `json:"sku"`
	Quantity      int    `json:"quantity"`
}
