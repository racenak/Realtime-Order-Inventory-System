package mocks

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/domain"
)

type MockReservationRepository struct {
	CreateFn       func(ctx context.Context, reservation *domain.Reservation) error
	CreateInTxFn   func(ctx context.Context, tx *sqlx.Tx, reservation *domain.Reservation) error
	GetByIDFn      func(ctx context.Context, id string) (*domain.Reservation, error)
	GetByOrderIDFn func(ctx context.Context, orderID string) ([]*domain.Reservation, error)
	UpdateStatusFn func(ctx context.Context, id string, status string) error
}

func (m *MockReservationRepository) Create(ctx context.Context, reservation *domain.Reservation) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, reservation)
	}
	return nil
}

func (m *MockReservationRepository) CreateInTx(ctx context.Context, tx *sqlx.Tx, reservation *domain.Reservation) error {
	if m.CreateInTxFn != nil {
		return m.CreateInTxFn(ctx, tx, reservation)
	}
	return nil
}

func (m *MockReservationRepository) GetByID(ctx context.Context, id string) (*domain.Reservation, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, domain.ErrReservationNotFound
}

func (m *MockReservationRepository) GetByOrderID(ctx context.Context, orderID string) ([]*domain.Reservation, error) {
	if m.GetByOrderIDFn != nil {
		return m.GetByOrderIDFn(ctx, orderID)
	}
	return nil, nil
}

func (m *MockReservationRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	if m.UpdateStatusFn != nil {
		return m.UpdateStatusFn(ctx, id, status)
	}
	return nil
}
