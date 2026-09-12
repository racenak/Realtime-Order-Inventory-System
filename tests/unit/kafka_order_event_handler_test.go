package unit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	inventorykafka "github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/adapter/kafka"
	"github.com/racenak/Realtime-Order-Inventory-System/tests/mocks"
)

func makeOrderCreatedEvent(orderID string, items []map[string]interface{}) []byte {
	event := map[string]interface{}{
		"event_id":   "evt-1",
		"event_type": "order.created",
		"data": map[string]interface{}{
			"order_id":     orderID,
			"customer_id":  "cust-1",
			"status":       "pending",
			"currency":     "USD",
			"total_amount": 100.0,
			"items":        items,
		},
	}
	b, _ := json.Marshal(event)
	return b
}

func makeOrderCancelledEvent(orderID string) []byte {
	event := map[string]interface{}{
		"event_id":   "evt-2",
		"event_type": "order.cancelled",
		"data": map[string]interface{}{
			"order_id":        orderID,
			"reservation_ids": []string{"res-1"},
		},
	}
	b, _ := json.Marshal(event)
	return b
}

func TestOrderEventHandler_OrderCreated_ReservesStock(t *testing.T) {
	logger := zap.NewNop()
	var reserved []inventorykafka.ReserveStockRequest

	mockUC := &mocks.MockInventoryUseCase{
		ReserveStockFn: func(ctx context.Context, req inventorykafka.ReserveStockRequest) error {
			reserved = append(reserved, req)
			return nil
		},
	}

	handler := inventorykafka.NewOrderEventHandler(mockUC, &mocks.MockMessageWriter{}, logger)

	items := []map[string]interface{}{
		{"order_item_id": "oi-1", "product_id": "prod-1", "sku": "SKU-1", "product_name": "Widget", "quantity": 2, "unit_price": 50.0, "total_price": 100.0},
	}

	msg := kafka.Message{
		Value: makeOrderCreatedEvent("order-123", items),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("order.created")},
		},
	}

	err := handler.Handle(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reserved) != 1 {
		t.Fatalf("expected 1 reservation, got %d", len(reserved))
	}
	if reserved[0].OrderID != "order-123" {
		t.Errorf("expected order_id order-123, got %s", reserved[0].OrderID)
	}
	if reserved[0].ProductID != "prod-1" {
		t.Errorf("expected product_id prod-1, got %s", reserved[0].ProductID)
	}
	if reserved[0].Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", reserved[0].Quantity)
	}
}

func TestOrderEventHandler_OrderCreated_MultipleItems(t *testing.T) {
	logger := zap.NewNop()
	var reserved []inventorykafka.ReserveStockRequest

	mockUC := &mocks.MockInventoryUseCase{
		ReserveStockFn: func(ctx context.Context, req inventorykafka.ReserveStockRequest) error {
			reserved = append(reserved, req)
			return nil
		},
	}

	handler := inventorykafka.NewOrderEventHandler(mockUC, &mocks.MockMessageWriter{}, logger)

	items := []map[string]interface{}{
		{"order_item_id": "oi-1", "product_id": "prod-1", "sku": "SKU-1", "quantity": 2, "unit_price": 50.0, "total_price": 100.0},
		{"order_item_id": "oi-2", "product_id": "prod-2", "sku": "SKU-2", "quantity": 1, "unit_price": 30.0, "total_price": 30.0},
	}

	msg := kafka.Message{
		Value: makeOrderCreatedEvent("order-multi", items),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("order.created")},
		},
	}

	err := handler.Handle(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reserved) != 2 {
		t.Fatalf("expected 2 reservations, got %d", len(reserved))
	}
}

func TestOrderEventHandler_OrderCreated_ReserveFails(t *testing.T) {
	logger := zap.NewNop()

	mockUC := &mocks.MockInventoryUseCase{
		ReserveStockFn: func(ctx context.Context, req inventorykafka.ReserveStockRequest) error {
			return errors.New("insufficient stock")
		},
	}

	mockWriter := &mocks.MockMessageWriter{
		WriteMessagesFn: func(ctx context.Context, msgs ...kafka.Message) error {
			return nil
		},
	}

	handler := inventorykafka.NewOrderEventHandler(mockUC, mockWriter, logger)

	items := []map[string]interface{}{
		{"order_item_id": "oi-1", "product_id": "prod-1", "sku": "SKU-1", "quantity": 100, "unit_price": 50.0, "total_price": 5000.0},
	}

	msg := kafka.Message{
		Value: makeOrderCreatedEvent("order-fail", items),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("order.created")},
		},
	}

	err := handler.Handle(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error when ReserveStock fails")
	}
}

func TestOrderEventHandler_OrderCancelled_ReleasesReservation(t *testing.T) {
	logger := zap.NewNop()
	var releasedOrderID string

	mockUC := &mocks.MockInventoryUseCase{
		ReleaseReservationFn: func(ctx context.Context, orderID string) error {
			releasedOrderID = orderID
			return nil
		},
	}

	handler := inventorykafka.NewOrderEventHandler(mockUC, &mocks.MockMessageWriter{}, logger)

	msg := kafka.Message{
		Value: makeOrderCancelledEvent("order-789"),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("order.cancelled")},
		},
	}

	err := handler.Handle(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if releasedOrderID != "order-789" {
		t.Errorf("expected ReleaseReservation called with order-789, got %s", releasedOrderID)
	}
}

func TestOrderEventHandler_UnknownEvent_Ignored(t *testing.T) {
	logger := zap.NewNop()

	handler := inventorykafka.NewOrderEventHandler(&mocks.MockInventoryUseCase{}, &mocks.MockMessageWriter{}, logger)

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

func TestOrderEventHandler_BadJSON_ReturnsError(t *testing.T) {
	logger := zap.NewNop()

	handler := inventorykafka.NewOrderEventHandler(&mocks.MockInventoryUseCase{}, &mocks.MockMessageWriter{}, logger)

	msg := kafka.Message{
		Value: []byte(`not json`),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("order.created")},
		},
	}

	err := handler.Handle(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
}

func TestOrderEventHandler_ReserveFails_PublishesFailureEvent(t *testing.T) {
	logger := zap.NewNop()

	mockUC := &mocks.MockInventoryUseCase{
		ReserveStockFn: func(ctx context.Context, req inventorykafka.ReserveStockRequest) error {
			return errors.New("out of stock")
		},
	}

	var capturedMsg kafka.Message
	mockWriter := &mocks.MockMessageWriter{
		WriteMessagesFn: func(ctx context.Context, msgs ...kafka.Message) error {
			capturedMsg = msgs[0]
			return nil
		},
	}

	handler := inventorykafka.NewOrderEventHandler(mockUC, mockWriter, logger)

	items := []map[string]interface{}{
		{"order_item_id": "oi-1", "product_id": "prod-1", "sku": "SKU-1", "quantity": 10, "unit_price": 50.0, "total_price": 500.0},
	}

	msg := kafka.Message{
		Value: makeOrderCreatedEvent("order-pub-fail", items),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte("order.created")},
		},
	}

	err := handler.Handle(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error from ReserveStock")
	}

	if capturedMsg.Topic != "inventory.reservation_failed" {
		t.Errorf("expected topic inventory.reservation_failed, got %s", capturedMsg.Topic)
	}

	headers := make(map[string]string)
	for _, h := range capturedMsg.Headers {
		headers[h.Key] = string(h.Value)
	}
	if headers["event_type"] != "inventory.reservation_failed" {
		t.Errorf("expected event_type header inventory.reservation_failed, got %s", headers["event_type"])
	}
}
