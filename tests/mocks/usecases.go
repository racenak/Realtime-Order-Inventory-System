package mocks

import (
	"context"

	inventorykafka "github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/adapter/kafka"
)

type MockOrderUseCase struct {
	ConfirmOrderFn func(ctx context.Context, orderID string) error
	CancelOrderFn  func(ctx context.Context, orderID string, reason string) error
}

func (m *MockOrderUseCase) ConfirmOrder(ctx context.Context, orderID string) error {
	if m.ConfirmOrderFn != nil {
		return m.ConfirmOrderFn(ctx, orderID)
	}
	return nil
}

func (m *MockOrderUseCase) CancelOrder(ctx context.Context, orderID string, reason string) error {
	if m.CancelOrderFn != nil {
		return m.CancelOrderFn(ctx, orderID, reason)
	}
	return nil
}

type MockInventoryUseCase struct {
	ReserveStockFn      func(ctx context.Context, req inventorykafka.ReserveStockRequest) error
	ReleaseReservationFn func(ctx context.Context, orderID string) error
}

func (m *MockInventoryUseCase) ReserveStock(ctx context.Context, req inventorykafka.ReserveStockRequest) error {
	if m.ReserveStockFn != nil {
		return m.ReserveStockFn(ctx, req)
	}
	return nil
}

func (m *MockInventoryUseCase) ReleaseReservation(ctx context.Context, orderID string) error {
	if m.ReleaseReservationFn != nil {
		return m.ReleaseReservationFn(ctx, orderID)
	}
	return nil
}
