package e2e_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/httpd"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/postgres"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/usecase"
	"github.com/racenak/Realtime-Order-Inventory-System/tests/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupE2ERouter(t *testing.T) chi.Router {
	db := helpers.SetupTestDB(t)

	orderRepo := postgres.NewOrderRepository(db)
	orderItemRepo := postgres.NewOrderItemRepository(db)
	outboxRepo := postgres.NewOutboxRepository(db)

	uc := usecase.NewCreateOrderUseCase(orderRepo, orderItemRepo, outboxRepo)
	handler := httpd.NewOrderHandler(uc)

	r := chi.NewRouter()
	r.Mount("/api/orders", handler.Routes())
	return r
}

func TestE2E_CreateOrder_FullFlow(t *testing.T) {
	router := setupE2ERouter(t)

	// Step 1: Create order
	createBody := map[string]interface{}{
		"customer_id": uuid.New().String(),
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "sku": "WIDGET-1", "product_name": "Widget", "quantity": 2, "unit_price": 25.00},
			{"product_id": uuid.New().String(), "sku": "GADGET-1", "product_name": "Gadget", "quantity": 1, "unit_price": 50.00},
		},
		"shipping_address": map[string]interface{}{
			"street":  "123 Main St",
			"city":    "New York",
			"state":   "NY",
			"zip":     "10001",
			"country": "US",
		},
	}

	jsonBody, _ := json.Marshal(createBody)
	req := httptest.NewRequest(http.MethodPost, "/api/orders/", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var createResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &createResp)
	require.NoError(t, err)
	assert.True(t, createResp["success"].(bool))

	orderData := createResp["data"].(map[string]interface{})
	orderID := orderData["id"].(string)
	assert.NotEmpty(t, orderID)
	assert.Equal(t, "pending_payment", orderData["status"])

	// Step 2: Get order
	req = httptest.NewRequest(http.MethodGet, "/api/orders/"+orderID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var getResp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &getResp)
	require.NoError(t, err)
	assert.True(t, getResp["success"].(bool))

	fetchedData := getResp["data"].(map[string]interface{})
	assert.Equal(t, orderID, fetchedData["id"])

	// Step 3: List orders
	req = httptest.NewRequest(http.MethodGet, "/api/orders/?limit=10&offset=0", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var listResp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &listResp)
	require.NoError(t, err)
	assert.True(t, listResp["success"].(bool))

	meta := listResp["meta"].(map[string]interface{})
	assert.GreaterOrEqual(t, meta["total"].(float64), float64(1))

	// Step 4: Cancel order
	cancelBody := map[string]interface{}{
		"reason": "Changed my mind",
	}
	jsonCancelBody, _ := json.Marshal(cancelBody)
	req = httptest.NewRequest(http.MethodPost, "/api/orders/"+orderID+"/cancel", bytes.NewBuffer(jsonCancelBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Step 5: Verify cancelled
	req = httptest.NewRequest(http.MethodGet, "/api/orders/"+orderID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var cancelledResp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &cancelledResp)
	require.NoError(t, err)
	cancelledData := cancelledResp["data"].(map[string]interface{})
	assert.Equal(t, "cancelled", cancelledData["status"])
}

func TestE2E_IdempotencyKey_DuplicateReturnsConflict(t *testing.T) {
	router := setupE2ERouter(t)

	body := map[string]interface{}{
		"customer_id": uuid.New().String(),
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 1, "unit_price": 10.00},
		},
		"shipping_address": map[string]interface{}{
			"street": "123 Main St",
			"city":   "New York",
		},
	}

	jsonBody, _ := json.Marshal(body)

	// First request
	req := httptest.NewRequest(http.MethodPost, "/api/orders/", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "e2e-test-key-001")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	// Duplicate request with same key
	req = httptest.NewRequest(http.MethodPost, "/api/orders/", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "e2e-test-key-001")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestE2E_CreateOrder_InvalidJSON(t *testing.T) {
	router := setupE2ERouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/orders/", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestE2E_CreateOrder_EmptyCart(t *testing.T) {
	router := setupE2ERouter(t)

	body := map[string]interface{}{
		"customer_id": uuid.New().String(),
		"items":       []map[string]interface{}{},
		"shipping_address": map[string]interface{}{
			"street": "123 Main St",
			"city":   "New York",
		},
	}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/orders/", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestE2E_GetOrder_NotFound(t *testing.T) {
	router := setupE2ERouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/orders/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestE2E_CancelOrder_NotFound(t *testing.T) {
	router := setupE2ERouter(t)

	body := map[string]interface{}{"reason": "test"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/orders/"+uuid.New().String()+"/cancel", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestE2E_CreateOrder_TotalCalculation(t *testing.T) {
	router := setupE2ERouter(t)

	body := map[string]interface{}{
		"customer_id": uuid.New().String(),
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 3, "unit_price": 20.00},
			{"product_id": uuid.New().String(), "quantity": 1, "unit_price": 10.00},
		},
		"shipping_address": map[string]interface{}{
			"street": "123 Main St",
			"city":   "New York",
		},
	}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/orders/", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	orderData := resp["data"].(map[string]interface{})
	// TotalAmount = (3*20 + 1*10) = 70
	assert.Equal(t, float64(70), orderData["total_amount"])
}

func TestE2E_CreateOrder_DefaultCurrency(t *testing.T) {
	router := setupE2ERouter(t)

	body := map[string]interface{}{
		"customer_id": uuid.New().String(),
		"items": []map[string]interface{}{
			{"product_id": uuid.New().String(), "quantity": 1, "unit_price": 10.00},
		},
		"shipping_address": map[string]interface{}{
			"street": "123 Main St",
			"city":   "New York",
		},
	}

	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/orders/", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	orderData := resp["data"].(map[string]interface{})
	assert.Equal(t, "USD", orderData["currency"])
}
