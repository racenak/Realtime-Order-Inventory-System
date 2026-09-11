package usecase_test

import (
	"context"
	"testing"

	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/domain"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/usecase"
	"github.com/racenak/Realtime-Order-Inventory-System/tests/mocks"
	"github.com/stretchr/testify/assert"
)

func TestReserveStock_InvalidQuantity(t *testing.T) {
	uc := usecase.NewInventoryUseCase(
		&mocks.MockInventoryRepository{},
		&mocks.MockReservationRepository{},
		&mocks.MockMovementRepository{},
	)

	_, err := uc.ReserveStock(context.Background(), usecase.ReserveStockRequest{
		ProductID: "p1",
		Quantity:  0,
	})
	assert.ErrorIs(t, err, domain.ErrInvalidQuantity)

	_, err = uc.ReserveStock(context.Background(), usecase.ReserveStockRequest{
		ProductID: "p1",
		Quantity:  -5,
	})
	assert.ErrorIs(t, err, domain.ErrInvalidQuantity)
}

func TestReserveStock_InsufficientStock(t *testing.T) {
	invRepo := &mocks.MockInventoryRepository{
		GetByProductAndWarehouseFn: func(ctx context.Context, productID, warehouseID string) (*domain.Inventory, error) {
			return &domain.Inventory{
				QuantityOnHand:   10,
				QuantityReserved: 5,
			}, nil
		},
	}

	uc := usecase.NewInventoryUseCase(
		invRepo,
		&mocks.MockReservationRepository{},
		&mocks.MockMovementRepository{},
	)

	_, err := uc.ReserveStock(context.Background(), usecase.ReserveStockRequest{
		ProductID:   "p1",
		WarehouseID: "wh1",
		Quantity:    10, // only 5 available (10 - 5)
	})
	assert.ErrorIs(t, err, domain.ErrInsufficientStock)
}

func TestReleaseReservation_AlreadyReleased(t *testing.T) {
	reservationRepo := &mocks.MockReservationRepository{
		GetByIDFn: func(ctx context.Context, id string) (*domain.Reservation, error) {
			return &domain.Reservation{
				ID:     "res-1",
				Status: "released",
			}, nil
		},
	}

	uc := usecase.NewInventoryUseCase(
		&mocks.MockInventoryRepository{},
		reservationRepo,
		&mocks.MockMovementRepository{},
	)

	err := uc.ReleaseReservation(context.Background(), "res-1")
	assert.NoError(t, err) // Should be no-op
}

func TestReleaseReservation_NotFound(t *testing.T) {
	reservationRepo := &mocks.MockReservationRepository{
		GetByIDFn: func(ctx context.Context, id string) (*domain.Reservation, error) {
			return nil, domain.ErrReservationNotFound
		},
	}

	uc := usecase.NewInventoryUseCase(
		&mocks.MockInventoryRepository{},
		reservationRepo,
		&mocks.MockMovementRepository{},
	)

	err := uc.ReleaseReservation(context.Background(), "res-1")
	assert.ErrorIs(t, err, domain.ErrReservationNotFound)
}

func TestGetStock_Success(t *testing.T) {
	invRepo := &mocks.MockInventoryRepository{
		GetByProductIDFn: func(ctx context.Context, productID string) ([]*domain.Inventory, error) {
			return []*domain.Inventory{
				{
					WarehouseID:      "wh1",
					QuantityOnHand:   100,
					QuantityReserved: 20,
				},
				{
					WarehouseID:      "wh2",
					QuantityOnHand:   50,
					QuantityReserved: 10,
				},
			}, nil
		},
	}

	uc := usecase.NewInventoryUseCase(invRepo, &mocks.MockReservationRepository{}, &mocks.MockMovementRepository{})

	result, err := uc.GetStock(context.Background(), "p1")
	assert.NoError(t, err)
	assert.Equal(t, "p1", result.ProductID)
	assert.Len(t, result.Warehouses, 2)
	assert.Equal(t, 80, result.Warehouses[0].Available)  // 100 - 20
	assert.Equal(t, 40, result.Warehouses[1].Available)   // 50 - 10
}

func TestGetStock_Empty(t *testing.T) {
	invRepo := &mocks.MockInventoryRepository{
		GetByProductIDFn: func(ctx context.Context, productID string) ([]*domain.Inventory, error) {
			return []*domain.Inventory{}, nil
		},
	}

	uc := usecase.NewInventoryUseCase(invRepo, &mocks.MockReservationRepository{}, &mocks.MockMovementRepository{})

	result, err := uc.GetStock(context.Background(), "p1")
	assert.NoError(t, err)
	assert.Empty(t, result.Warehouses)
}
