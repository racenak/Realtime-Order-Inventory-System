package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	pkgkafka "github.com/racenak/Realtime-Order-Inventory-System/pkg/kafka"
)

type OrderUseCase interface {
	ConfirmOrder(ctx context.Context, orderID string) error
	CancelOrder(ctx context.Context, orderID string, reason string) error
}

type InventoryReservedEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	Data      struct {
		OrderID     string `json:"order_id"`
		Reservation struct {
			ID          string `json:"id"`
			OrderID     string `json:"order_id"`
			ProductID   string `json:"product_id"`
			WarehouseID string `json:"warehouse_id"`
			Quantity    int    `json:"quantity"`
		} `json:"reservation"`
	} `json:"data"`
}

type InventoryReservationFailedEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	Data      struct {
		OrderID   string `json:"order_id"`
		ProductID string `json:"product_id"`
		Reason    string `json:"reason"`
	} `json:"data"`
}

type InventoryEventHandler struct {
	orderUC  OrderUseCase
	producer pkgkafka.MessageWriter
	logger   *zap.Logger
}

func NewInventoryEventHandler(
	orderUC OrderUseCase,
	producer pkgkafka.MessageWriter,
	logger *zap.Logger,
) *InventoryEventHandler {
	return &InventoryEventHandler{
		orderUC:  orderUC,
		producer: producer,
		logger:   logger,
	}
}

func (h *InventoryEventHandler) Handle(ctx context.Context, msg kafka.Message) error {
	eventType := getOrderEventType(msg)

	if extractedCtx := pkgkafka.ExtractTraceContext(msg.Headers); extractedCtx != nil {
		ctx = extractedCtx
	}

	_, span := otel.Tracer("inventory-event-handler").Start(ctx, "kafka.handle_inventory_event",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.operation", "process"),
			attribute.String("event.type", eventType),
			attribute.Int64("messaging.kafka.message.offset", msg.Offset),
		),
	)
	defer span.End()

	h.logger.Info("processing inventory event",
		zap.String("event_type", eventType),
		zap.Int64("offset", msg.Offset),
	)

	var err error
	switch eventType {
	case "inventory.reserved":
		err = h.handleInventoryReserved(ctx, msg)
	case "inventory.reservation_failed":
		err = h.handleReservationFailed(ctx, msg)
	case "inventory.released":
		err = h.handleInventoryReleased(ctx, msg)
	default:
		h.logger.Debug("ignoring event type", zap.String("event_type", eventType))
		return nil
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return err
}

func (h *InventoryEventHandler) handleInventoryReserved(ctx context.Context, msg kafka.Message) error {
	var event InventoryReservedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal inventory.reserved event: %w", err)
	}

	if err := h.orderUC.ConfirmOrder(ctx, event.Data.OrderID); err != nil {
		h.logger.Error("failed to confirm order",
			zap.String("order_id", event.Data.OrderID),
			zap.Error(err),
		)
		return err
	}

	h.logger.Info("order confirmed after inventory reservation",
		zap.String("order_id", event.Data.OrderID),
	)

	return nil
}

func (h *InventoryEventHandler) handleReservationFailed(ctx context.Context, msg kafka.Message) error {
	var event InventoryReservationFailedEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal inventory.reservation_failed event: %w", err)
	}

	if err := h.orderUC.CancelOrder(ctx, event.Data.OrderID, event.Data.Reason); err != nil {
		h.logger.Error("failed to cancel order",
			zap.String("order_id", event.Data.OrderID),
			zap.Error(err),
		)
		return err
	}

	h.logger.Info("order cancelled due to inventory reservation failure",
		zap.String("order_id", event.Data.OrderID),
		zap.String("reason", event.Data.Reason),
	)

	return nil
}

func (h *InventoryEventHandler) handleInventoryReleased(ctx context.Context, msg kafka.Message) error {
	var event struct {
		Data struct {
			OrderID string `json:"order_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal inventory.released event: %w", err)
	}

	h.logger.Info("inventory released for order",
		zap.String("order_id", event.Data.OrderID),
	)

	return nil
}

func getOrderEventType(msg kafka.Message) string {
	for _, h := range msg.Headers {
		if h.Key == "event_type" {
			return string(h.Value)
		}
	}
	return ""
}
