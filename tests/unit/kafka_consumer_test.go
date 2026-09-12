package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	pkgkafka "github.com/racenak/Realtime-Order-Inventory-System/pkg/kafka"
)

func newTestConsumer(handler func(ctx context.Context, msg kafkago.Message) error, cfg pkgkafka.ConsumerConfig) *pkgkafka.Consumer {
	logger := zap.NewNop()
	return pkgkafka.NewConsumer(cfg, handler, logger)
}

func TestConsumer_HandleWithRetry_SuccessFirstAttempt(t *testing.T) {
	attempts := 0
	handler := func(ctx context.Context, msg kafkago.Message) error {
		attempts++
		return nil
	}

	consumer := newTestConsumer(handler, pkgkafka.ConsumerConfig{
		Brokers:    []string{"localhost:9092"},
		Topic:      "test",
		GroupID:    "test-group",
		MaxRetries: 3,
		RetryDelay: time.Millisecond,
	})

	msg := kafkago.Message{Offset: 1, Value: []byte("test")}

	err := consumer.HandleWithRetry(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestConsumer_HandleWithRetry_SuccessAfterRetry(t *testing.T) {
	attempts := 0
	handler := func(ctx context.Context, msg kafkago.Message) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}

	consumer := newTestConsumer(handler, pkgkafka.ConsumerConfig{
		Brokers:    []string{"localhost:9092"},
		Topic:      "test",
		GroupID:    "test-group",
		MaxRetries: 3,
		RetryDelay: time.Millisecond,
	})

	msg := kafkago.Message{Offset: 1, Value: []byte("test")}

	err := consumer.HandleWithRetry(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestConsumer_HandleWithRetry_ExhaustsRetries(t *testing.T) {
	attempts := 0
	handler := func(ctx context.Context, msg kafkago.Message) error {
		attempts++
		return errors.New("persistent failure")
	}

	consumer := newTestConsumer(handler, pkgkafka.ConsumerConfig{
		Brokers:    []string{"localhost:9092"},
		Topic:      "test",
		GroupID:    "test-group",
		MaxRetries: 2,
		RetryDelay: time.Millisecond,
	})

	msg := kafkago.Message{Offset: 1, Value: []byte("test")}

	err := consumer.HandleWithRetry(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// MaxRetries=2 means 1 initial + 2 retries = 3 total attempts
	if attempts != 3 {
		t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", attempts)
	}
}

func TestConsumer_HandleWithRetry_ContextCancelled(t *testing.T) {
	handler := func(ctx context.Context, msg kafkago.Message) error {
		return errors.New("failure")
	}

	consumer := newTestConsumer(handler, pkgkafka.ConsumerConfig{
		Brokers:    []string{"localhost:9092"},
		Topic:      "test",
		GroupID:    "test-group",
		MaxRetries: 5,
		RetryDelay: time.Second,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := kafkago.Message{Offset: 1, Value: []byte("test")}

	err := consumer.HandleWithRetry(ctx, msg)
	if err == nil {
		t.Fatal("expected error after context cancellation")
	}
}

func TestConsumer_HandleWithRetry_DefaultConfig(t *testing.T) {
	attempts := 0
	handler := func(ctx context.Context, msg kafkago.Message) error {
		attempts++
		return errors.New("always fail")
	}

	consumer := newTestConsumer(handler, pkgkafka.ConsumerConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "test",
		GroupID: "test-group",
	})

	cfg := consumer.Config()
	if cfg.MaxRetries != 3 {
		t.Errorf("expected default MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if cfg.RetryDelay != time.Second {
		t.Errorf("expected default RetryDelay 1s, got %v", cfg.RetryDelay)
	}

	msg := kafkago.Message{Offset: 1, Value: []byte("test")}

	err := consumer.HandleWithRetry(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// Default MaxRetries=3 means 1 initial + 3 retries = 4 total attempts
	if attempts != 4 {
		t.Errorf("expected 4 attempts (1 initial + 3 retries), got %d", attempts)
	}
}

func TestConsumer_SendToDLQ_NoWriter(t *testing.T) {
	consumer := newTestConsumer(
		func(ctx context.Context, msg kafkago.Message) error { return nil },
		pkgkafka.ConsumerConfig{
			Brokers: []string{"localhost:9092"},
			Topic:   "test",
			GroupID: "test-group",
		},
	)

	msg := kafkago.Message{
		Key:   []byte("key"),
		Value: []byte("value"),
	}

	err := consumer.SendToDLQ(context.Background(), msg, errors.New("error"))
	if err != nil {
		t.Fatalf("expected nil error when no DLQ writer, got: %v", err)
	}
}

func TestConsumer_Config_DLQTopic(t *testing.T) {
	consumer := newTestConsumer(
		func(ctx context.Context, msg kafkago.Message) error { return nil },
		pkgkafka.ConsumerConfig{
			Brokers:  []string{"localhost:9092"},
			Topic:    "test",
			GroupID:  "test-group",
			DLQTopic: "test.dlq",
		},
	)

	cfg := consumer.Config()
	if cfg.DLQTopic != "test.dlq" {
		t.Errorf("expected DLQTopic test.dlq, got %s", cfg.DLQTopic)
	}
}
