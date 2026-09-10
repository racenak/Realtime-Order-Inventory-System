package httpd

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/usecase"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/response"
)

type OrderHandler struct {
	uc usecase.OrderUseCase
}

func NewOrderHandler(uc usecase.OrderUseCase) *OrderHandler {
	return &OrderHandler{uc: uc}
}

func (h *OrderHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.CreateOrder)
	r.Get("/{id}", h.GetOrder)
	r.Get("/", h.ListOrders)
	r.Post("/{id}/cancel", h.CancelOrder)
	return r
}

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req usecase.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	order, err := h.uc.CreateOrder(r.Context(), req)
	if err != nil {
		handleError(w, r, err)
		return
	}

	response.JSON(w, r, http.StatusCreated, order)
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	order, err := h.uc.GetOrder(r.Context(), id)
	if err != nil {
		handleError(w, r, err)
		return
	}

	response.JSON(w, r, http.StatusOK, order)
}

func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	customerID := r.URL.Query().Get("customer_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 {
		limit = 20
	}

	orders, total, err := h.uc.ListOrders(r.Context(), customerID, limit, offset)
	if err != nil {
		handleError(w, r, err)
		return
	}

	response.JSONWithPagination(w, r, http.StatusOK, orders, total, limit, offset)
}

func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := h.uc.CancelOrder(r.Context(), id, req.Reason); err != nil {
		handleError(w, r, err)
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]string{"status": "cancelled"})
}

func handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch err.Error() {
	case "order not found":
		response.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Order not found")
	case "order is not cancellable":
		response.Error(w, r, http.StatusConflict, "ORDER_NOT_CANCELLABLE", "Order cannot be cancelled")
	case "order must contain at least one item":
		response.Error(w, r, http.StatusBadRequest, "EMPTY_CART", "Order must contain at least one item")
	case "quantity must be greater than 0":
		response.Error(w, r, http.StatusBadRequest, "INVALID_QUANTITY", "Quantity must be greater than 0")
	default:
		response.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred")
	}
}
