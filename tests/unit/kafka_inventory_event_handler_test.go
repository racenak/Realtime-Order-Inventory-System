package unit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	orderkafka "github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/kafka"
	"github.com/racenak/Realtime-Order-Inventory-System/tests/mocks"
)

func makeInventoryReservedEvent(orderID string) []byte {
	event := map[string]interface{}{
		"event_id":   "evt-1",
		"event_type": "inventory.reserved",
		"data": map[string]interface{}{
			"order_id": orderID,
			"reservation": map[string]interface{}{
				"id":          "res-1",
				"order_id":    orderID,
				"product_id":  "prod-1",
				"warehouse_id": "wh-1",
				"quantity":    5,
			},
		},
	}
	b, _ := json.Marshal(event)
	return b
}

func makeReservationFailedEvent(orderID, reason string) []byte {
	event := map[string]interface{}{
		"event_id":   "evt-2",
		"event_type": "inventory.reservation_failed",
		"data": map[string]interface{}{
			"order_id":   orderID,
			"product_id": "prod-1",
			"reason":     reason,
		},
	}
	b, _ := json.Marshal(event)
	return b
}

func makeInventoryReleasedEvent(orderID string) []byte {
	event := map[string]interface{}{
		"event_id":   "evt-3",
		"event_type": "inventory.released",
		"data": map[string]interface{}{
			"order_id": orderID,
		},
	}
	b, _ := json.Marshal(event)
	return b
}

func TestInventoryEventHandler_InventoryReserved_ConfirmsOrder(t *testing.T) {
	logger := zap.NewNop()
	var confirmedOrderID string

	mockUC := &mocks.MockOrderUseCase{
		ConfirmOrderFn: func(ctx context.Context, orderID string) error {
			confirmedOrderID = orderID
			return nil
		},
	}

	handler := orderkafka.NewInventoryEventHandler(mockUC, &mocks.MockMessageWriter{}, logger)

	msg := kafka.Message{
		Value: makeInventoryReservedEvent("order-123"),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("inventory.reserved")},
		},
	}

	err := handler.Handle(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if confirmedOrderID != "order-123" {
		t.Errorf("expected ConfirmOrder called with order-123, got %s", confirmedOrderID)
	}
}

func TestInventoryEventHandler_ReservationFailed_CancelsOrder(t *testing.T) {
	logger := zap.NewNop()
	var cancelledOrderID, cancelledReason string

	mockUC := &mocks.MockOrderUseCase{
		CancelOrderFn: func(ctx context.Context, orderID string, reason string) error {
			cancelledOrderID = orderID
			cancelledReason = reason
			return nil
		},
	}

	handler := orderkafka.NewInventoryEventHandler(mockUC, &mocks.MockMessageWriter{}, logger)

	msg := kafka.Message{
		Value: makeReservationFailedEvent("order-456", "insufficient stock"),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("inventory.reservation_failed")},
		},
	}

	err := handler.Handle(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cancelledOrderID != "order-456" {
		t.Errorf("expected CancelOrder called with order-456, got %s", cancelledOrderID)
	}
	if cancelledReason != "insufficient stock" {
		t.Errorf("expected reason 'insufficient stock', got %s", cancelledReason)
	}
}

func TestInventoryEventHandler_InventoryReleased_LogsOnly(t *testing.T) {
	logger := zap.NewNop()

	handler := orderkafka.NewInventoryEventHandler(&mocks.MockOrderUseCase{}, &mocks.MockMessageWriter{}, logger)

	msg := kafka.Message{
		Value: makeInventoryReleasedEvent("order-789"),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("inventory.released")},
		},
	}

	err := handler.Handle(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInventoryEventHandler_UnknownEvent_Ignored(t *testing.T) {
	logger := zap.NewNop()

	handler := orderkafka.NewInventoryEventHandler(&mocks.MockOrderUseCase{}, &mocks.MockMessageWriter{}, logger)

	msg := kafka.Message{
		Value: []byte(`{}`),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("unknown.event")},
		},
	}

	err := handler.Handle(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInventoryEventHandler_BadJSON_ReturnsError(t *testing.T) {
	logger := zap.NewNop()

	handler := orderkafka.NewInventoryEventHandler(&mocks.MockOrderUseCase{}, &mocks.MockMessageWriter{}, logger)

	msg := kafka.Message{
		Value: []byte(`not json`),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("inventory.reserved")},
		},
	}

	err := handler.Handle(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestInventoryEventHandler_ConfirmOrderError_ReturnsError(t *testing.T) {
	logger := zap.NewNop()

	mockUC := &mocks.MockOrderUseCase{
		ConfirmOrderFn: func(ctx context.Context, orderID string) error {
			return errors.New("db error")
		},
	}

	handler := orderkafka.NewInventoryEventHandler(mockUC, &mocks.MockMessageWriter{}, logger)

	msg := kafka.Message{
		Value: makeInventoryReservedEvent("order-err"),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("inventory.reserved")},
		},
	}

	err := handler.Handle(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error from ConfirmOrder")
	}
}
