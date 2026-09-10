package inventory_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/adapter/postgres"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/domain"
	"github.com/racenak/Realtime-Order-Inventory-System/tests/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	testDB = helpers.SetupTestDB(&testing.T{})
	m.Run()
}

func setupTest(t *testing.T) {
	helpers.CleanDatabase(t, testDB)
}

func seedTestData(t *testing.T) (warehouseID, productID string) {
	warehouseID = uuid.New().String()
	productID = uuid.New().String()

	helpers.SeedWarehouse(t, testDB, warehouseID, "WH-TEST", "Test Warehouse")
	helpers.SeedInventory(t, testDB, uuid.New().String(), productID, "SKU-001", warehouseID, 100)

	return warehouseID, productID
}

func TestInventoryRepository_GetByProductAndWarehouse(t *testing.T) {
	setupTest(t)
	warehouseID, productID := seedTestData(t)

	repo := postgres.NewInventoryRepository(testDB)

	inv, err := repo.GetByProductAndWarehouse(context.Background(), productID, warehouseID)
	require.NoError(t, err)
	assert.Equal(t, productID, inv.ProductID)
	assert.Equal(t, warehouseID, inv.WarehouseID)
	assert.Equal(t, 100, inv.QuantityOnHand)
	assert.Equal(t, 0, inv.QuantityReserved)
}

func TestInventoryRepository_GetByProductAndWarehouse_NotFound(t *testing.T) {
	setupTest(t)

	repo := postgres.NewInventoryRepository(testDB)

	_, err := repo.GetByProductAndWarehouse(context.Background(), uuid.New().String(), uuid.New().String())
	assert.ErrorIs(t, err, domain.ErrInventoryNotFound)
}

func TestInventoryRepository_GetByProductID(t *testing.T) {
	setupTest(t)
	productID := uuid.New().String()
	warehouse1ID := uuid.New().String()
	warehouse2ID := uuid.New().String()

	helpers.SeedWarehouse(t, testDB, warehouse1ID, "WH-001", "Warehouse 1")
	helpers.SeedWarehouse(t, testDB, warehouse2ID, "WH-002", "Warehouse 2")
	helpers.SeedInventory(t, testDB, uuid.New().String(), productID, "SKU-001", warehouse1ID, 50)
	helpers.SeedInventory(t, testDB, uuid.New().String(), productID, "SKU-001", warehouse2ID, 75)

	repo := postgres.NewInventoryRepository(testDB)

	inventories, err := repo.GetByProductID(context.Background(), productID)
	require.NoError(t, err)
	assert.Len(t, inventories, 2)
}

func TestInventoryRepository_UpdateStock(t *testing.T) {
	setupTest(t)
	warehouseID, productID := seedTestData(t)

	repo := postgres.NewInventoryRepository(testDB)

	inv, err := repo.GetByProductAndWarehouse(context.Background(), productID, warehouseID)
	require.NoError(t, err)

	err = repo.UpdateStock(context.Background(), productID, warehouseID, 50, inv.Version)
	require.NoError(t, err)

	updated, err := repo.GetByProductAndWarehouse(context.Background(), productID, warehouseID)
	require.NoError(t, err)
	assert.Equal(t, 150, updated.QuantityOnHand)
	assert.Equal(t, inv.Version+1, updated.Version)
}

func TestInventoryRepository_UpdateStock_ConcurrentModification(t *testing.T) {
	setupTest(t)
	warehouseID, productID := seedTestData(t)

	repo := postgres.NewInventoryRepository(testDB)

	err := repo.UpdateStock(context.Background(), productID, warehouseID, 10, 0)
	require.NoError(t, err)

	err = repo.UpdateStock(context.Background(), productID, warehouseID, 10, 0)
	assert.ErrorIs(t, err, domain.ErrConcurrentModification)
}

func TestReservationRepository_Create(t *testing.T) {
	setupTest(t)
	warehouseID, productID := seedTestData(t)

	repo := postgres.NewReservationRepository(testDB)

	reservation := &domain.Reservation{
		ID:          uuid.New().String(),
		OrderID:     uuid.New().String(),
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    10,
		Status:      "reserved",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := repo.Create(context.Background(), reservation)
	require.NoError(t, err)

	fetched, err := repo.GetByID(context.Background(), reservation.ID)
	require.NoError(t, err)
	assert.Equal(t, reservation.ID, fetched.ID)
	assert.Equal(t, "reserved", fetched.Status)
}

func TestReservationRepository_GetByID_NotFound(t *testing.T) {
	setupTest(t)

	repo := postgres.NewReservationRepository(testDB)

	_, err := repo.GetByID(context.Background(), uuid.New().String())
	assert.ErrorIs(t, err, domain.ErrReservationNotFound)
}

func TestReservationRepository_GetByOrderID(t *testing.T) {
	setupTest(t)
	warehouseID, productID := seedTestData(t)

	repo := postgres.NewReservationRepository(testDB)
	orderID := uuid.New().String()

	for i := 0; i < 3; i++ {
		reservation := &domain.Reservation{
			ID:          uuid.New().String(),
			OrderID:     orderID,
			ProductID:   productID,
			WarehouseID: warehouseID,
			Quantity:    5,
			Status:      "reserved",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		err := repo.Create(context.Background(), reservation)
		require.NoError(t, err)
	}

	reservations, err := repo.GetByOrderID(context.Background(), orderID)
	require.NoError(t, err)
	assert.Len(t, reservations, 3)
}

func TestReservationRepository_UpdateStatus(t *testing.T) {
	setupTest(t)
	warehouseID, productID := seedTestData(t)

	repo := postgres.NewReservationRepository(testDB)

	reservation := &domain.Reservation{
		ID:          uuid.New().String(),
		OrderID:     uuid.New().String(),
		ProductID:   productID,
		WarehouseID: warehouseID,
		Quantity:    10,
		Status:      "reserved",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := repo.Create(context.Background(), reservation)
	require.NoError(t, err)

	err = repo.UpdateStatus(context.Background(), reservation.ID, "confirmed")
	require.NoError(t, err)

	fetched, err := repo.GetByID(context.Background(), reservation.ID)
	require.NoError(t, err)
	assert.Equal(t, "confirmed", fetched.Status)
}

func TestMovementRepository_Create(t *testing.T) {
	setupTest(t)
	warehouseID, productID := seedTestData(t)

	repo := postgres.NewMovementRepository(testDB)

	movement := &domain.InventoryMovement{
		ID:            uuid.New().String(),
		ProductID:     productID,
		WarehouseID:   warehouseID,
		MovementType:  domain.MovementIn,
		Quantity:      50,
		ReferenceType: "restock",
		CreatedAt:     time.Now(),
	}

	err := repo.Create(context.Background(), movement)
	require.NoError(t, err)

	movements, err := repo.GetByProductID(context.Background(), productID)
	require.NoError(t, err)
	assert.Len(t, movements, 1)
	assert.Equal(t, domain.MovementIn, movements[0].MovementType)
}

func TestInventoryRepository_AvailableStock(t *testing.T) {
	setupTest(t)
	warehouseID, productID := seedTestData(t)

	repo := postgres.NewInventoryRepository(testDB)

	inv, err := repo.GetByProductAndWarehouse(context.Background(), productID, warehouseID)
	require.NoError(t, err)

	assert.Equal(t, 100, inv.Available())
	assert.Equal(t, 100, inv.QuantityOnHand-inv.QuantityReserved)
}
