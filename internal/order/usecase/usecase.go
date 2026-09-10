package usecase

import (
	"context"

	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
)

type OrderUseCase interface {
	CreateOrder(ctx context.Context, req CreateOrderRequest) (*domain.Order, error)
	GetOrder(ctx context.Context, id string) (*domain.Order, error)
	ListOrders(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, int, error)
	CancelOrder(ctx context.Context, id string, reason string) error
}

type CreateOrderRequest struct {
	CustomerID      string                 `json:"customer_id"`
	Items           []OrderItemRequest     `json:"items"`
	ShippingAddress ShippingAddressRequest `json:"shipping_address"`
}

type OrderItemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type ShippingAddressRequest struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	State   string `json:"state"`
	Zip     string `json:"zip"`
	Country string `json:"country"`
}
