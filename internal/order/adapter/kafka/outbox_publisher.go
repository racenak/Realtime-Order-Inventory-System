package kafka

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
	pkgkafka "github.com/racenak/Realtime-Order-Inventory-System/pkg/kafka"
)

type OutboxRepository interface {
	ClaimBatch(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkPublished(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string) error
}

type OutboxPublisher struct {
	repo   OutboxRepository
	writer pkgkafka.MessageWriter
	logger *zap.Logger
}

func NewOutboxPublisher(
	repo OutboxRepository,
	brokers []string,
	logger *zap.Logger,
) *OutboxPublisher {
	return &OutboxPublisher{
		repo: repo,
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.LeastBytes{},
			BatchTimeout: 10 * time.Millisecond,
		},
		logger: logger,
	}
}

func NewOutboxPublisherWithWriter(
	repo OutboxRepository,
	writer pkgkafka.MessageWriter,
	logger *zap.Logger,
) *OutboxPublisher {
	return &OutboxPublisher{
		repo:   repo,
		writer: writer,
		logger: logger,
	}
}

func (p *OutboxPublisher) Start(ctx context.Context, interval time.Duration, batchSize int) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	p.logger.Info("outbox publisher started",
		zap.Duration("interval", interval),
		zap.Int("batch_size", batchSize),
	)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("outbox publisher stopping")
			return
		case <-ticker.C:
			if err := p.publishBatch(ctx, batchSize); err != nil {
				p.logger.Error("failed to publish batch", zap.Error(err))
			}
		}
	}
}

func (p *OutboxPublisher) publishBatch(ctx context.Context, batchSize int) error {
	events, err := p.repo.ClaimBatch(ctx, batchSize)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	p.logger.Info("publishing outbox events", zap.Int("count", len(events)))

	for _, event := range events {
		if err := p.publishEvent(ctx, event); err != nil {
			p.logger.Error("failed to publish event, marking as failed",
				zap.String("event_id", event.ID),
				zap.Error(err),
			)
			if markErr := p.repo.MarkFailed(ctx, event.ID); markErr != nil {
				p.logger.Error("failed to mark event as failed",
					zap.String("event_id", event.ID),
					zap.Error(markErr),
				)
			}
			continue
		}

		if err := p.repo.MarkPublished(ctx, event.ID); err != nil {
			p.logger.Error("failed to mark event as published",
				zap.String("event_id", event.ID),
				zap.Error(err),
			)
		}
	}

	return nil
}

func (p *OutboxPublisher) publishEvent(ctx context.Context, event domain.OutboxEvent) error {
	topic := p.getTopicForEvent(event.EventType)

	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(event.AggregateID),
		Value: []byte(event.Payload),
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.EventType)},
			{Key: "event_id", Value: []byte(event.ID)},
			{Key: "correlation_id", Value: []byte(uuid.New().String())},
		},
	}

	return p.writer.WriteMessages(ctx, msg)
}

func (p *OutboxPublisher) getTopicForEvent(eventType string) string {
	switch eventType {
	case "order.created", "order.paid", "order.confirmed", "order.cancelled", "order.shipped", "order.delivered":
		return eventType
	case "order.status.changed":
		return "order.status.changed"
	default:
		return "default"
	}
}

func (p *OutboxPublisher) Close() error {
	if closer, ok := p.writer.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func (p *OutboxPublisher) GetTopicForEvent(eventType string) string {
	return p.getTopicForEvent(eventType)
}

func (p *OutboxPublisher) PublishBatch(ctx context.Context, batchSize int) error {
	return p.publishBatch(ctx, batchSize)
}

func (p *OutboxPublisher) PublishEvent(ctx context.Context, event domain.OutboxEvent) error {
	return p.publishEvent(ctx, event)
}
