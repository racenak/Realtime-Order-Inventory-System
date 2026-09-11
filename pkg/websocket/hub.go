package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/racenak/Realtime-Order-Inventory-System/pkg/metrics"
)

type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	logger     *zap.Logger
}

func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Info("client connected",
				zap.String("client_id", client.ID()),
				zap.Int("total_clients", h.ClientCount()),
			)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			metrics.WSConnectionsActive.WithLabelValues("websocket-service").Dec()
			h.logger.Info("client disconnected",
				zap.String("client_id", client.ID()),
				zap.Int("total_clients", h.ClientCount()),
			)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) BroadcastMessage(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal broadcast message", zap.Error(err))
		return
	}
	h.broadcast <- data
}

func (h *Hub) BroadcastToChannel(channel string, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal broadcast message", zap.Error(err))
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.HasChannel(channel) {
			select {
			case client.send <- data:
				metrics.WSMessagesSentTotal.WithLabelValues("websocket-service", channel).Inc()
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	channels := make(map[string]int)
	for client := range h.clients {
		for ch := range client.channels {
			channels[ch]++
		}
	}

	return map[string]interface{}{
		"total_clients": len(h.clients),
		"channels":      channels,
		"timestamp":     time.Now().UTC(),
	}
}
