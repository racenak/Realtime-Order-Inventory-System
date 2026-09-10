package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type MessageHandler func(ctx context.Context, msg kafka.Message) error

type Consumer struct {
	reader  *kafka.Reader
	handler MessageHandler
	logger  *zap.Logger
}

type ConsumerConfig struct {
	Brokers     []string
	Topic       string
	GroupID     string
	MinBytes    int
	MaxBytes    int
	StartOffset int64
}

func NewConsumer(cfg ConsumerConfig, handler MessageHandler, logger *zap.Logger) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.Brokers,
		Topic:       cfg.Topic,
		GroupID:     cfg.GroupID,
		MinBytes:    cfg.MinBytes,
		MaxBytes:    cfg.MaxBytes,
		StartOffset: cfg.StartOffset,
	})

	return &Consumer{
		reader:  r,
		handler: handler,
		logger:  logger,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("kafka consumer started",
		zap.String("topic", c.reader.Config().Topic),
		zap.String("group_id", c.reader.Config().GroupID),
	)

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.Error("failed to read message", zap.Error(err))
			continue
		}

		if err := c.handler(ctx, msg); err != nil {
			c.logger.Error("failed to handle message",
				zap.Error(err),
				zap.Int64("offset", msg.Offset),
				zap.String("topic", msg.Topic),
			)
			continue
		}

		c.logger.Debug("message processed",
			zap.Int64("offset", msg.Offset),
			zap.String("topic", msg.Topic),
		)
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
