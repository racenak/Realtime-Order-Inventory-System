package websocket

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	ws "github.com/racenak/Realtime-Order-Inventory-System/pkg/websocket"
)

type Subscriber struct {
	rdb    *redis.Client
	hub    *ws.Hub
	logger *zap.Logger
}

type EventMessage struct {
	EventID   string      `json:"event_id"`
	EventType string      `json:"event_type"`
	Data      interface{} `json:"data"`
}

func NewSubscriber(rdb *redis.Client, hub *ws.Hub, logger *zap.Logger) *Subscriber {
	return &Subscriber{
		rdb:    rdb,
		hub:    hub,
		logger: logger,
	}
}

func (s *Subscriber) Start(ctx context.Context) error {
	pubsub := s.rdb.Subscribe(ctx, "order.events", "inventory.events")
	defer func() { _ = pubsub.Close() }()

	ch := pubsub.Channel()
	s.logger.Info("redis subscriber started",
		zap.Strings("channels", []string{"order.events", "inventory.events"}),
	)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("redis subscriber stopping")
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return fmt.Errorf("redis channel closed")
			}

			if err := s.handleMessage(ctx, msg.Channel, msg.Payload); err != nil {
				s.logger.Error("failed to handle redis message",
					zap.String("channel", msg.Channel),
					zap.Error(err),
				)
			}
		}
	}
}

func (s *Subscriber) handleMessage(ctx context.Context, channel string, payload string) error {
	var event EventMessage
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	s.logger.Info("received event",
		zap.String("channel", channel),
		zap.String("event_type", event.EventType),
	)

	msg := ws.Message{
		Type:    event.EventType,
		Payload: event.Data,
	}

	switch channel {
	case "order.events":
		s.handleOrderEvent(event, msg)
	case "inventory.events":
		s.handleInventoryEvent(event, msg)
	}

	return nil
}

func (s *Subscriber) handleOrderEvent(event EventMessage, msg ws.Message) {
	switch event.EventType {
	case "order.status.changed":
		orderID := s.extractOrderID(event.Data)
		if orderID != "" {
			s.hub.BroadcastToChannel("order:"+orderID, msg)
		}
		s.hub.BroadcastToChannel("orders", msg)

	case "order.created", "order.cancelled", "order.confirmed":
		customerID := s.extractCustomerID(event.Data)
		if customerID != "" {
			s.hub.BroadcastToChannel("customer:"+customerID, msg)
		}

	default:
		s.hub.BroadcastToChannel("orders", msg)
	}
}

func (s *Subscriber) handleInventoryEvent(event EventMessage, msg ws.Message) {
	switch event.EventType {
	case "inventory.updated":
		productID := s.extractProductID(event.Data)
		if productID != "" {
			s.hub.BroadcastToChannel("product:"+productID, msg)
		}
		s.hub.BroadcastToChannel("inventory", msg)

	case "inventory.low_stock":
		s.hub.BroadcastToChannel("inventory:low_stock", msg)

	default:
		s.hub.BroadcastToChannel("inventory", msg)
	}
}

func (s *Subscriber) extractOrderID(data interface{}) string {
	if m, ok := data.(map[string]interface{}); ok {
		if id, ok := m["order_id"].(string); ok {
			return id
		}
	}
	return ""
}

func (s *Subscriber) extractCustomerID(data interface{}) string {
	if m, ok := data.(map[string]interface{}); ok {
		if id, ok := m["customer_id"].(string); ok {
			return id
		}
	}
	return ""
}

func (s *Subscriber) extractProductID(data interface{}) string {
	if m, ok := data.(map[string]interface{}); ok {
		if id, ok := m["product_id"].(string); ok {
			return id
		}
	}
	return ""
}
