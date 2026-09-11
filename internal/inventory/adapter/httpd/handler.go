package httpd

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/usecase"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/metrics"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/response"
)

type InventoryHandler struct {
	uc usecase.InventoryUseCase
}

func NewInventoryHandler(uc usecase.InventoryUseCase) *InventoryHandler {
	return &InventoryHandler{uc: uc}
}

func (h *InventoryHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/stock/{product_id}", h.GetStock)
	r.Post("/reserve", h.ReserveStock)
	r.Post("/release/{reservation_id}", h.ReleaseReservation)
	r.Put("/stock", h.UpdateStock)
	return r
}

func (h *InventoryHandler) GetStock(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "product_id")

	stock, err := h.uc.GetStock(r.Context(), productID)
	if err != nil {
		handleError(w, r, err)
		return
	}

	response.JSON(w, r, http.StatusOK, stock)
}

type reserveStockHTTPItem struct {
	ProductID     string `json:"product_id"`
	SKU           string `json:"sku"`
	Quantity      int    `json:"quantity"`
	WarehouseCode string `json:"warehouse_code"`
}

type reserveStockHTTPRequest struct {
	OrderID string                 `json:"order_id"`
	Items   []reserveStockHTTPItem `json:"items"`
}

func (h *InventoryHandler) ReserveStock(w http.ResponseWriter, r *http.Request) {
	var req reserveStockHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	type reservationResult struct {
		Reservation interface{}          `json:"reservation"`
		Item        reserveStockHTTPItem `json:"item"`
	}
	var results []reservationResult
	var lastErr error

	for _, item := range req.Items {
		reservation, err := h.uc.ReserveStock(r.Context(), usecase.ReserveStockRequest{
			OrderID:       req.OrderID,
			ProductID:     item.ProductID,
			WarehouseCode: item.WarehouseCode,
			SKU:           item.SKU,
			Quantity:      item.Quantity,
		})
		if err != nil {
			metrics.InventoryFailedTotal.WithLabelValues("inventory-service", err.Error()).Inc()
			lastErr = err
			continue
		}
		metrics.InventoryReservationsTotal.WithLabelValues("inventory-service", "success").Inc()
		results = append(results, reservationResult{
			Reservation: reservation,
			Item:        item,
		})
	}

	if len(results) == 0 && lastErr != nil {
		handleError(w, r, lastErr)
		return
	}

	response.JSON(w, r, http.StatusCreated, map[string]interface{}{
		"reservations": results,
	})
}

func (h *InventoryHandler) ReleaseReservation(w http.ResponseWriter, r *http.Request) {
	reservationID := chi.URLParam(r, "reservation_id")

	if err := h.uc.ReleaseReservation(r.Context(), reservationID); err != nil {
		metrics.InventoryFailedTotal.WithLabelValues("inventory-service", err.Error()).Inc()
		handleError(w, r, err)
		return
	}

	metrics.InventoryReleasesTotal.WithLabelValues("inventory-service").Inc()
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "released"})
}

func (h *InventoryHandler) UpdateStock(w http.ResponseWriter, r *http.Request) {
	var req usecase.UpdateStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := h.uc.UpdateStock(r.Context(), req); err != nil {
		metrics.InventoryFailedTotal.WithLabelValues("inventory-service", err.Error()).Inc()
		handleError(w, r, err)
		return
	}

	metrics.InventoryUpdatesTotal.WithLabelValues("inventory-service").Inc()
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "updated"})
}

func handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch err.Error() {
	case "insufficient stock":
		response.Error(w, r, http.StatusConflict, "INSUFFICIENT_STOCK", "Not enough stock available")
	case "inventory not found":
		response.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Inventory not found")
	case "concurrent modification detected":
		response.Error(w, r, http.StatusConflict, "CONCURRENT_MODIFICATION", "Resource was modified by another request")
	case "reservation not found":
		response.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Reservation not found")
	case "invalid quantity":
		response.Error(w, r, http.StatusBadRequest, "INVALID_QUANTITY", "Quantity must be greater than 0")
	default:
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred")
	}
}
