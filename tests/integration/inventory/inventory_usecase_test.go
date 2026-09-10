package inventory_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/adapter/postgres"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/domain"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/usecase"
	"github.com/racenak/Realtime-Order-Inventory-System/tests/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupInventoryUsecase(t *testing.T) (usecase.InventoryUseCase, string, string) {
	db := helpers.SetupTestDB(t)

	warehouseID := uuid.New().String()
	productID := uuid.New().String()

	helpers.SeedWarehouse(t, db, warehouseID, "WH-TEST", "Test Warehouse")
	helpers.SeedInventory(t, db, uuid.New().String(), productID, "SKU-001", warehouseID, 100)

	inventoryRepo := postgres.NewInventoryRepository(db)
	reservationRepo := postgres.NewReservationRepository(db)
	movementRepo := postgres.NewMovementRepository(db)

	uc := usecase.NewInventoryUseCase(inventoryRepo, reservationRepo, movementRepo)
	return uc, warehouseID, productID
}

func TestGetStock_Success(t *testing.T) {
	uc, warehouseID, productID := setupInventoryUsecase(t)

	stock, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, productID, stock.ProductID)
	assert.Len(t, stock.Warehouses, 1)
	assert.Equal(t, warehouseID, stock.Warehouses[0].WarehouseID)
	assert.Equal(t, 100, stock.Warehouses[0].Quantity)
	assert.Equal(t, 0, stock.Warehouses[0].Reserved)
	assert.Equal(t, 100, stock.Warehouses[0].Available)
}

func TestGetStock_NotFound(t *testing.T) {
	uc, _, _ := setupInventoryUsecase(t)

	stock, err := uc.GetStock(context.Background(), uuid.New().String())
	require.NoError(t, err)
	assert.Len(t, stock.Warehouses, 0)
}

func TestReserveStock_Success(t *testing.T) {
	uc, warehouseID, productID := setupInventoryUsecase(t)

	req := usecase.ReserveStockRequest{
		OrderID:     uuid.New().String(),
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    10,
	}

	reservation, err := uc.ReserveStock(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, reservation)
	assert.Equal(t, "reserved", reservation.Status)
	assert.Equal(t, 10, reservation.Quantity)
}

func TestReserveStock_InsufficientStock(t *testing.T) {
	uc, warehouseID, productID := setupInventoryUsecase(t)

	req := usecase.ReserveStockRequest{
		OrderID:     uuid.New().String(),
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    200,
	}

	_, err := uc.ReserveStock(context.Background(), req)
	assert.ErrorIs(t, err, domain.ErrInsufficientStock)
}

func TestReserveStock_InvalidQuantity(t *testing.T) {
	uc, warehouseID, productID := setupInventoryUsecase(t)

	req := usecase.ReserveStockRequest{
		OrderID:     uuid.New().String(),
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    0,
	}

	_, err := uc.ReserveStock(context.Background(), req)
	assert.ErrorIs(t, err, domain.ErrInvalidQuantity)
}

func TestReserveStock_NegativeQuantity(t *testing.T) {
	uc, warehouseID, productID := setupInventoryUsecase(t)

	req := usecase.ReserveStockRequest{
		OrderID:     uuid.New().String(),
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    -5,
	}

	_, err := uc.ReserveStock(context.Background(), req)
	assert.ErrorIs(t, err, domain.ErrInvalidQuantity)
}

func TestReleaseReservation_Success(t *testing.T) {
	uc, warehouseID, productID := setupInventoryUsecase(t)

	// First reserve
	reserveReq := usecase.ReserveStockRequest{
		OrderID:     uuid.New().String(),
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    10,
	}

	reservation, err := uc.ReserveStock(context.Background(), reserveReq)
	require.NoError(t, err)

	// Then release
	err = uc.ReleaseReservation(context.Background(), reservation.ID)
	require.NoError(t, err)
}

func TestReleaseReservation_NotFound(t *testing.T) {
	uc, _, _ := setupInventoryUsecase(t)

	err := uc.ReleaseReservation(context.Background(), uuid.New().String())
	assert.ErrorIs(t, err, domain.ErrReservationNotFound)
}

func TestUpdateStock_Success(t *testing.T) {
	uc, warehouseID, productID := setupInventoryUsecase(t)

	req := usecase.UpdateStockRequest{
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    50,
	}

	err := uc.UpdateStock(context.Background(), req)
	require.NoError(t, err)

	// Verify stock increased
	stock, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, 150, stock.Warehouses[0].Quantity)
}

func TestUpdateStock_DecreaseStock(t *testing.T) {
	uc, warehouseID, productID := setupInventoryUsecase(t)

	req := usecase.UpdateStockRequest{
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    -30,
	}

	err := uc.UpdateStock(context.Background(), req)
	require.NoError(t, err)

	stock, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, 70, stock.Warehouses[0].Quantity)
}

func TestReserveStock_MultipleReservations(t *testing.T) {
	uc, warehouseID, productID := setupInventoryUsecase(t)

	for i := 0; i < 3; i++ {
		req := usecase.ReserveStockRequest{
			OrderID:     uuid.New().String(),
			ProductID:   productID,
			WarehouseID: warehouseID,
			Quantity:    10,
		}

		_, err := uc.ReserveStock(context.Background(), req)
		require.NoError(t, err)
	}

	stock, err := uc.GetStock(context.Background(), productID)
	require.NoError(t, err)
	assert.Equal(t, 100, stock.Warehouses[0].Quantity)
	assert.Equal(t, 30, stock.Warehouses[0].Reserved)
	assert.Equal(t, 70, stock.Warehouses[0].Available)
}

func TestReserveStock_ExceedAvailable(t *testing.T) {
	uc, warehouseID, productID := setupInventoryUsecase(t)

	// Reserve 90 (leaves 10 available)
	req1 := usecase.ReserveStockRequest{
		OrderID:     uuid.New().String(),
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    90,
	}
	_, err := uc.ReserveStock(context.Background(), req1)
	require.NoError(t, err)

	// Try to reserve 20 (should fail, only 10 available)
	req2 := usecase.ReserveStockRequest{
		OrderID:     uuid.New().String(),
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    20,
	}
	_, err = uc.ReserveStock(context.Background(), req2)
	assert.ErrorIs(t, err, domain.ErrInsufficientStock)
}
