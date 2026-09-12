package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	pkgkafka "github.com/racenak/Realtime-Order-Inventory-System/pkg/kafka"
)

type InventoryUseCase interface {
	ReserveStock(ctx context.Context, req ReserveStockRequest) error
	ReleaseReservation(ctx context.Context, orderID string) error
}

type ReserveStockRequest struct {
	OrderID     string `json:"order_id"`
	ProductID   string `json:"product_id"`
	SKU         string `json:"sku"`
	WarehouseID string `json:"warehouse_id"`
	Quantity    int    `json:"quantity"`
}

type OrderCreatedEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	Data      struct {
		OrderID     string  `json:"order_id"`
		CustomerID  string  `json:"customer_id"`
		Status      string  `json:"status"`
		Currency    string  `json:"currency"`
		TotalAmount float64 `json:"total_amount"`
		Items       []struct {
			OrderItemID string  `json:"order_item_id"`
			ProductID   string  `json:"product_id"`
			SKU         string  `json:"sku"`
			ProductName string  `json:"product_name"`
			Quantity    int     `json:"quantity"`
			UnitPrice   float64 `json:"unit_price"`
			TotalPrice  float64 `json:"total_price"`
		} `json:"items"`
	} `json:"data"`
}

type OrderCancelledEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	Data      struct {
		OrderID        string   `json:"order_id"`
		ReservationIDs []string `json:"reservation_ids"`
	} `json:"data"`
}

type OrderEventHandler struct {
	inventoryUC InventoryUseCase
	producer    pkgkafka.MessageWriter
	logger      *zap.Logger
}

func NewOrderEventHandler(
	inventoryUC InventoryUseCase,
	producer pkgkafka.MessageWriter,
	logger *zap.Logger,
) *OrderEventHandler {
	return &OrderEventHandler{
		inventoryUC: inventoryUC,
		producer:    producer,
		logger:      logger,
	}
}

func (h *OrderEventHandler) Handle(ctx context.Context, msg kafka.Message) error {
	eventType := getEventType(msg)
	h.logger.Info("processing order event",
		zap.String("event_type", eventType),
		zap.Int64("offset", msg.Offset),
	)

	switch eventType {
	case "order.created":
		return h.handleOrderCreated(ctx, msg)
	case "order.cancelled":
		return h.handleOrderCancelled(ctx, msg)
	default:
		h.logger.Debug("ignoring event type", zap.String("event_type", eventType))
		return nil
	}
}

func (h *OrderEventHandler) handleOrderCreated(ctx context.Context, msg kafka.Message) error {
	var event OrderCreatedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal order.created event: %w", err)
	}

	for _, item := range event.Data.Items {
		req := ReserveStockRequest{
			OrderID:   event.Data.OrderID,
			ProductID: item.ProductID,
			SKU:       item.SKU,
			Quantity:  item.Quantity,
		}

		if err := h.inventoryUC.ReserveStock(ctx, req); err != nil {
			h.logger.Error("failed to reserve stock",
				zap.String("order_id", event.Data.OrderID),
				zap.String("product_id", item.ProductID),
				zap.Error(err),
			)

			if err := h.publishInventoryReservationFailed(ctx, event.Data.OrderID, item.ProductID, err.Error()); err != nil {
				h.logger.Error("failed to publish reservation failed event", zap.Error(err))
			}

			return err
		}
	}

	h.logger.Info("stock reserved successfully",
		zap.String("order_id", event.Data.OrderID),
		zap.Int("items", len(event.Data.Items)),
	)

	return nil
}

func (h *OrderEventHandler) handleOrderCancelled(ctx context.Context, msg kafka.Message) error {
	var event OrderCancelledEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal order.cancelled event: %w", err)
	}

	if err := h.inventoryUC.ReleaseReservation(ctx, event.Data.OrderID); err != nil {
		h.logger.Error("failed to release reservation",
			zap.String("order_id", event.Data.OrderID),
			zap.Error(err),
		)
		return err
	}

	h.logger.Info("reservation released successfully",
		zap.String("order_id", event.Data.OrderID),
	)

	return nil
}

func (h *OrderEventHandler) publishInventoryReservationFailed(ctx context.Context, orderID, productID, reason string) error {
	event := map[string]interface{}{
		"event_type": "inventory.reservation_failed",
		"data": map[string]interface{}{
			"order_id":   orderID,
			"product_id": productID,
			"reason":     reason,
		},
	}

	value, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Topic: "inventory.reservation_failed",
		Key:   []byte(orderID),
		Value: value,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("inventory.reservation_failed")},
		},
	}

	return h.producer.WriteMessages(ctx, msg)
}

func getEventType(msg kafka.Message) string {
	for _, h := range msg.Headers {
		if h.Key == "event_type" {
			return string(h.Value)
		}
	}
	return ""
}
