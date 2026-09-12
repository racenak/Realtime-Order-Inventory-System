package unit

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	orderkafka "github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/kafka"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
	"github.com/racenak/Realtime-Order-Inventory-System/tests/mocks"
)

func TestOutboxPublisher_GetTopicForEvent(t *testing.T) {
	tests := []struct {
		eventType string
		expected  string
	}{
		{"order.created", "order.created"},
		{"order.paid", "order.paid"},
		{"order.confirmed", "order.confirmed"},
		{"order.cancelled", "order.cancelled"},
		{"order.shipped", "order.shipped"},
		{"order.delivered", "order.delivered"},
		{"order.status.changed", "order.status.changed"},
		{"unknown.event", "default"},
		{"", "default"},
	}

	logger := zap.NewNop()
	mockWriter := &mocks.MockMessageWriter{}
	mockOutbox := &mocks.MockOutboxRepository{}

	pub := orderkafka.NewOutboxPublisherWithWriter(mockOutbox, mockWriter, logger)

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			result := pub.GetTopicForEvent(tt.eventType)
			if result != tt.expected {
				t.Errorf("getTopicForEvent(%q) = %q, want %q", tt.eventType, result, tt.expected)
			}
		})
	}
}

func TestOutboxPublisher_PublishBatch_EmptyBatch(t *testing.T) {
	logger := zap.NewNop()
	mockWriter := &mocks.MockMessageWriter{}
	mockOutbox := &mocks.MockOutboxRepository{
		ClaimBatchFn: func(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
			return []domain.OutboxEvent{}, nil
		},
	}

	pub := orderkafka.NewOutboxPublisherWithWriter(mockOutbox, mockWriter, logger)

	err := pub.PublishBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mockWriter.WriteMessagesFn != nil {
		t.Error("WriteMessages should not be called for empty batch")
	}
}

func TestOutboxPublisher_PublishBatch_Success(t *testing.T) {
	logger := zap.NewNop()

	var publishedIDs []string
	var writtenMsgs []kafka.Message

	mockWriter := &mocks.MockMessageWriter{
		WriteMessagesFn: func(ctx context.Context, msgs ...kafka.Message) error {
			writtenMsgs = append(writtenMsgs, msgs...)
			return nil
		},
	}

	mockOutbox := &mocks.MockOutboxRepository{
		ClaimBatchFn: func(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
			return []domain.OutboxEvent{
				{ID: "evt-1", AggregateID: "order-1", EventType: "order.created", Payload: `{"order_id":"order-1"}`},
				{ID: "evt-2", AggregateID: "order-2", EventType: "order.cancelled", Payload: `{"order_id":"order-2"}`},
			}, nil
		},
		MarkPublishedFn: func(ctx context.Context, id string) error {
			publishedIDs = append(publishedIDs, id)
			return nil
		},
	}

	pub := orderkafka.NewOutboxPublisherWithWriter(mockOutbox, mockWriter, logger)

	err := pub.PublishBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(writtenMsgs) != 2 {
		t.Fatalf("expected 2 messages written, got %d", len(writtenMsgs))
	}
	if len(publishedIDs) != 2 {
		t.Fatalf("expected 2 events marked published, got %d", len(publishedIDs))
	}
	if writtenMsgs[0].Topic != "order.created" {
		t.Errorf("expected topic order.created, got %s", writtenMsgs[0].Topic)
	}
	if writtenMsgs[1].Topic != "order.cancelled" {
		t.Errorf("expected topic order.cancelled, got %s", writtenMsgs[1].Topic)
	}
}

func TestOutboxPublisher_PublishBatch_PartialFailure(t *testing.T) {
	logger := zap.NewNop()

	var publishedIDs []string
	var failedIDs []string

	mockWriter := &mocks.MockMessageWriter{
		WriteMessagesFn: func(ctx context.Context, msgs ...kafka.Message) error {
			for _, msg := range msgs {
				for _, h := range msg.Headers {
					if h.Key == "event_id" {
						id := string(h.Value)
						if id == "evt-2" {
							return errors.New("kafka write failed")
						}
					}
				}
			}
			return nil
		},
	}

	mockOutbox := &mocks.MockOutboxRepository{
		ClaimBatchFn: func(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
			return []domain.OutboxEvent{
				{ID: "evt-1", AggregateID: "order-1", EventType: "order.created", Payload: `{"order_id":"order-1"}`},
				{ID: "evt-2", AggregateID: "order-2", EventType: "order.cancelled", Payload: `{"order_id":"order-2"}`},
			}, nil
		},
		MarkPublishedFn: func(ctx context.Context, id string) error {
			publishedIDs = append(publishedIDs, id)
			return nil
		},
		MarkFailedFn: func(ctx context.Context, id string) error {
			failedIDs = append(failedIDs, id)
			return nil
		},
	}

	pub := orderkafka.NewOutboxPublisherWithWriter(mockOutbox, mockWriter, logger)

	err := pub.PublishBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(publishedIDs) != 1 || publishedIDs[0] != "evt-1" {
		t.Errorf("expected evt-1 published, got %v", publishedIDs)
	}
	if len(failedIDs) != 1 || failedIDs[0] != "evt-2" {
		t.Errorf("expected evt-2 failed, got %v", failedIDs)
	}
}

func TestOutboxPublisher_PublishBatch_ClaimBatchError(t *testing.T) {
	logger := zap.NewNop()
	mockWriter := &mocks.MockMessageWriter{}

	mockOutbox := &mocks.MockOutboxRepository{
		ClaimBatchFn: func(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
			return nil, errors.New("db connection lost")
		},
	}

	pub := orderkafka.NewOutboxPublisherWithWriter(mockOutbox, mockWriter, logger)

	err := pub.PublishBatch(context.Background(), 10)
	if err == nil {
		t.Fatal("expected error from ClaimBatch")
	}
}

func TestOutboxPublisher_PublishEvent_CorrectHeaders(t *testing.T) {
	logger := zap.NewNop()

	var capturedMsg kafka.Message
	mockWriter := &mocks.MockMessageWriter{
		WriteMessagesFn: func(ctx context.Context, msgs ...kafka.Message) error {
			capturedMsg = msgs[0]
			return nil
		},
	}

	mockOutbox := &mocks.MockOutboxRepository{}
	pub := orderkafka.NewOutboxPublisherWithWriter(mockOutbox, mockWriter, logger)

	event := domain.OutboxEvent{
		ID:            "evt-123",
		AggregateID:   "order-abc",
		EventType:     "order.created",
		Payload:       `{"test": true}`,
	}

	err := pub.PublishEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Equal(capturedMsg.Key, []byte("order-abc")) {
		t.Errorf("expected key order-abc, got %s", string(capturedMsg.Key))
	}
	if !bytes.Equal(capturedMsg.Value, []byte(`{"test": true}`)) {
		t.Errorf("unexpected payload: %s", string(capturedMsg.Value))
	}

	headers := make(map[string]string)
	for _, h := range capturedMsg.Headers {
		headers[h.Key] = string(h.Value)
	}
	if headers["event_type"] != "order.created" {
		t.Errorf("expected event_type header order.created, got %s", headers["event_type"])
	}
	if headers["event_id"] != "evt-123" {
		t.Errorf("expected event_id header evt-123, got %s", headers["event_id"])
	}
	if _, ok := headers["correlation_id"]; !ok {
		t.Error("expected correlation_id header to be set")
	}
}
