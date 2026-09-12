package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/adapter/httpd"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/adapter/postgres"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/usecase"
	"github.com/racenak/Realtime-Order-Inventory-System/tests/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupInventoryHandler(t *testing.T) (*httpd.InventoryHandler, string, string) {
	db := helpers.SetupTestDB(t)

	warehouseID := uuid.New().String()
	productID := uuid.New().String()

	helpers.SeedWarehouse(t, db, warehouseID, "WH-TEST", "Test Warehouse")
	helpers.SeedInventory(t, db, uuid.New().String(), productID, "SKU-001", warehouseID, 100)

	inventoryRepo := postgres.NewInventoryRepository(db)
	reservationRepo := postgres.NewReservationRepository(db)
	movementRepo := postgres.NewMovementRepository(db)

	uc := usecase.NewInventoryUseCase(inventoryRepo, reservationRepo, movementRepo)
	return httpd.NewInventoryHandler(uc), warehouseID, productID
}

func TestInventoryHandler_GetStock(t *testing.T) {
	handler, _, productID := setupInventoryHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/stock/"+productID, nil)
	w := httptest.NewRecorder()

	router := handler.Routes()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))
}

func TestInventoryHandler_ReserveStock(t *testing.T) {
	handler, _, productID := setupInventoryHandler(t)

	body := map[string]interface{}{
		"order_id": uuid.New().String(),
		"items": []map[string]interface{}{
			{
				"product_id":    productID,
				"sku":           "SKU-001",
				"warehouse_code": "WH-TEST",
				"quantity":      10,
			},
		},
	}

	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/reserve", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := handler.Routes()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))
}

func TestInventoryHandler_ReserveStock_InsufficientStock(t *testing.T) {
	handler, _, productID := setupInventoryHandler(t)

	body := map[string]interface{}{
		"order_id": uuid.New().String(),
		"items": []map[string]interface{}{
			{
				"product_id":    productID,
				"sku":           "SKU-001",
				"warehouse_code": "WH-TEST",
				"quantity":      200,
			},
		},
	}

	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/reserve", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := handler.Routes()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestInventoryHandler_UpdateStock(t *testing.T) {
	handler, warehouseID, productID := setupInventoryHandler(t)

	body := map[string]interface{}{
		"product_id":   productID,
		"warehouse_id": warehouseID,
		"quantity":     50,
	}

	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/stock", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := handler.Routes()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestInventoryHandler_GetStock_NotFound(t *testing.T) {
	handler, _, _ := setupInventoryHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/stock/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	router := handler.Routes()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
