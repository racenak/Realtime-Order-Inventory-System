package kafka

import (
	"context"
	"time"

	"github.com/racenak/Realtime-Order-Inventory-System/pkg/metrics"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type MessageHandler func(ctx context.Context, msg kafka.Message) error

type Consumer struct {
	reader  *kafka.Reader
	writer  *kafka.Writer
	handler MessageHandler
	logger  *zap.Logger
	config  ConsumerConfig
	service string
	tracer  trace.Tracer
}

type ConsumerConfig struct {
	Brokers     []string
	Topic       string
	GroupID     string
	MinBytes    int
	MaxBytes    int
	StartOffset int64
	MaxRetries  int
	RetryDelay  time.Duration
	DLQTopic    string
	Service     string
}

func NewConsumer(cfg ConsumerConfig, handler MessageHandler, logger *zap.Logger) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       cfg.MinBytes,
		MaxBytes:       cfg.MaxBytes,
		CommitInterval: 0,
	})

	var w *kafka.Writer
	if cfg.DLQTopic != "" {
		w = &kafka.Writer{
			Addr:         kafka.TCP(cfg.Brokers...),
			Balancer:     &kafka.LeastBytes{},
			BatchTimeout: 10 * time.Millisecond,
		}
	}

	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = time.Second
	}

	return &Consumer{
		reader:  r,
		writer:  w,
		handler: handler,
		logger:  logger,
		config:  cfg,
		service: cfg.Service,
		tracer:  otel.Tracer("kafka.consumer"),
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	c.logger.Info("kafka consumer started",
		zap.String("topic", c.reader.Config().Topic),
		zap.String("group_id", c.reader.Config().GroupID),
		zap.Int("max_retries", c.config.MaxRetries),
		zap.String("dlq_topic", c.config.DLQTopic),
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

		topic := msg.Topic

		metrics.KafkaMessagesConsumedTotal.WithLabelValues(c.service, topic).Inc()

		consumerCtx, span := c.tracer.Start(ctx, "kafka.consume",
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("messaging.system", "kafka"),
				attribute.String("messaging.operation", "process"),
				attribute.String("messaging.destination.name", topic),
				attribute.Int64("messaging.kafka.message.offset", msg.Offset),
				attribute.String("messaging.kafka.message.key", string(msg.Key)),
			),
		)

		if extractedCtx := ExtractTraceContext(msg.Headers); extractedCtx != nil {
			consumerCtx = extractedCtx
		}

		if err := c.handleWithRetry(consumerCtx, msg); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			c.logger.Error("message failed after all retries, sending to DLQ",
				zap.Error(err),
				zap.Int64("offset", msg.Offset),
				zap.String("topic", msg.Topic),
			)
			if dlqErr := c.sendToDLQ(consumerCtx, msg, err); dlqErr != nil {
				c.logger.Error("failed to send to DLQ",
					zap.Error(dlqErr),
					zap.Int64("offset", msg.Offset),
				)
			}
		} else {
			span.SetStatus(codes.Ok, "")
		}
		span.End()

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error("failed to commit offset",
				zap.Error(err),
				zap.Int64("offset", msg.Offset),
			)
		}

		c.logger.Debug("message processed",
			zap.Int64("offset", msg.Offset),
			zap.String("topic", msg.Topic),
		)
	}
}

func (c *Consumer) handleWithRetry(ctx context.Context, msg kafka.Message) error {
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			c.logger.Warn("retrying message",
				zap.Int("attempt", attempt),
				zap.Int64("offset", msg.Offset),
				zap.Duration("delay", c.config.RetryDelay),
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.config.RetryDelay):
			}
		}

		if err := c.handler(ctx, msg); err != nil {
			lastErr = err
			c.logger.Error("handler error",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.Int64("offset", msg.Offset),
			)
			continue
		}

		return nil
	}

	return lastErr
}

func (c *Consumer) sendToDLQ(ctx context.Context, originalMsg kafka.Message, handlerErr error) error {
	if c.writer == nil || c.config.DLQTopic == "" {
		return nil
	}

	dlqMsg := kafka.Message{
		Key:   originalMsg.Key,
		Value: originalMsg.Value,
		Headers: append(originalMsg.Headers,
			kafka.Header{Key: "dlq_original_topic", Value: []byte(originalMsg.Topic)},
			kafka.Header{Key: "dlq_error", Value: []byte(handlerErr.Error())},
			kafka.Header{Key: "dlq_timestamp", Value: []byte(time.Now().UTC().Format(time.RFC3339))},
		),
	}

	return c.writer.WriteMessages(ctx, dlqMsg)
}

func (c *Consumer) Close() error {
	if c.writer != nil {
		_ = c.writer.Close()
	}
	return c.reader.Close()
}

func (c *Consumer) HandleWithRetry(ctx context.Context, msg kafka.Message) error {
	return c.handleWithRetry(ctx, msg)
}

func (c *Consumer) SendToDLQ(ctx context.Context, originalMsg kafka.Message, handlerErr error) error {
	return c.sendToDLQ(ctx, originalMsg, handlerErr)
}

func (c *Consumer) Config() ConsumerConfig {
	return c.config
}
