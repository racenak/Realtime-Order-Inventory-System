package kafka

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Producer struct {
	writer *kafka.Writer
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
	return p.writer.Close()
}
