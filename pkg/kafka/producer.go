package kafka

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// MessageWriter is an interface for writing Kafka messages.
// Both *kafka.Writer and test mocks implement this.
type MessageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

type Producer struct {
	writer MessageWriter
	logger *zap.Logger
}

type ProducerConfig struct {
	Brokers  []string
	Topic    string
	Balancer kafka.Balancer
}

func NewProducer(cfg ProducerConfig, logger *zap.Logger) *Producer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     cfg.Balancer,
		BatchTimeout: 10 * time.Millisecond,
	}

	return &Producer{
		writer: w,
		logger: logger,
	}
}

func (p *Producer) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	return p.writer.WriteMessages(ctx, msgs...)
}

func (p *Producer) Close() error {
	if closer, ok := p.writer.(*kafka.Writer); ok {
		return closer.Close()
	}
	return nil
}
