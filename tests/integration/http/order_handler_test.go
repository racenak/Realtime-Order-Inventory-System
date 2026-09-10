package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/httpd"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/postgres"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/usecase"
	"github.com/racenak/Realtime-Order-Inventory-System/tests/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupOrderHandler(t *testing.T) *httpd.OrderHandler {
	db := helpers.SetupTestDB(t)

	orderRepo := postgres.NewOrderRepository(db)
	orderItemRepo := postgres.NewOrderItemRepository(db)
	outboxRepo := postgres.NewOutboxRepository(db)

	uc := usecase.NewCreateOrderUseCase(orderRepo, orderItemRepo, outboxRepo)
	return httpd.NewOrderHandler(uc)
}

func TestOrderHandler_CreateOrder(t *testing.T) {
	handler := setupOrderHandler(t)

	body := map[string]interface{}{
		"customer_id": uuid.New().String(),
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 2},
		},
		"shipping_address": map[string]interface{}{
			"street":  "123 Main St",
			"city":    "New York",
			"state":   "NY",
			"zip":     "10001",
			"country": "US",
		},
	}

	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonBody))
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

func TestOrderHandler_CreateOrder_EmptyCart(t *testing.T) {
	handler := setupOrderHandler(t)

	body := map[string]interface{}{
		"customer_id": uuid.New().String(),
		"items":       []map[string]interface{}{},
		"shipping_address": map[string]interface{}{
			"street": "123 Main St",
			"city":   "New York",
		},
	}

	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := handler.Routes()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOrderHandler_CreateOrder_InvalidJSON(t *testing.T) {
	handler := setupOrderHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := handler.Routes()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestOrderHandler_GetOrder_NotFound(t *testing.T) {
	handler := setupOrderHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	router := handler.Routes()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestOrderHandler_ListOrders_Empty(t *testing.T) {
	handler := setupOrderHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/?customer_id="+uuid.New().String(), nil)
	w := httptest.NewRecorder()

	router := handler.Routes()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response["success"].(bool))
}
