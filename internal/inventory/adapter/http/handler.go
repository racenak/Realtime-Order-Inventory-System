package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/usecase"
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

func (h *InventoryHandler) ReserveStock(w http.ResponseWriter, r *http.Request) {
	var req usecase.ReserveStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	reservation, err := h.uc.ReserveStock(r.Context(), req)
	if err != nil {
		handleError(w, r, err)
		return
	}

	response.JSON(w, r, http.StatusCreated, reservation)
}

func (h *InventoryHandler) ReleaseReservation(w http.ResponseWriter, r *http.Request) {
	reservationID := chi.URLParam(r, "reservation_id")

	if err := h.uc.ReleaseReservation(r.Context(), reservationID); err != nil {
		handleError(w, r, err)
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"status": "released"})
}

func (h *InventoryHandler) UpdateStock(w http.ResponseWriter, r *http.Request) {
	var req usecase.UpdateStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := h.uc.UpdateStock(r.Context(), req); err != nil {
		handleError(w, r, err)
		return
	}

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
