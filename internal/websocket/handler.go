package websocket

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/racenak/Realtime-Order-Inventory-System/pkg/metrics"
	ws "github.com/racenak/Realtime-Order-Inventory-System/pkg/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Handler struct {
	hub    *ws.Hub
	logger *zap.Logger
}

func NewHandler(hub *ws.Hub, logger *zap.Logger) *Handler {
	return &Handler{
		hub:    hub,
		logger: logger,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.HandleWebSocket)
	r.Get("/stats", h.HandleStats)
	return r
}

func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("failed to upgrade websocket", zap.Error(err))
		return
	}

	client := ws.NewClient(conn, h.hub, h.logger)

	h.hub.Register(client)

	metrics.WSConnectionsTotal.WithLabelValues("websocket-service").Inc()
	metrics.WSConnectionsActive.WithLabelValues("websocket-service").Inc()

	go client.WritePump()
	go client.ReadPump()

	h.logger.Info("new websocket connection",
		zap.String("client_id", client.ID()),
		zap.String("remote_addr", r.RemoteAddr),
	)
}

func (h *Handler) HandleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.hub.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
