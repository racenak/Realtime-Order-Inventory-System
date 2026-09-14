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

type MessageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

type Producer struct {
	writer  MessageWriter
	logger  *zap.Logger
	service string
	tracer  trace.Tracer
}

type ProducerConfig struct {
	Brokers  []string
	Topic    string
	Balancer kafka.Balancer
	Service  string
}

func NewProducer(cfg ProducerConfig, logger *zap.Logger) *Producer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     cfg.Balancer,
		BatchTimeout: 10 * time.Millisecond,
	}

	return &Producer{
		writer:  w,
		logger:  logger,
		service: cfg.Service,
		tracer:  otel.Tracer("kafka.producer"),
	}
}

func (p *Producer) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	for _, msg := range msgs {
		topic := msg.Topic
		if topic == "" {
			if w, ok := p.writer.(*kafka.Writer); ok {
				topic = w.Topic
			}
		}

		ctx, span := p.tracer.Start(ctx, "kafka.produce",
			trace.WithSpanKind(trace.SpanKindProducer),
			trace.WithAttributes(
				attribute.String("messaging.system", "kafka"),
				attribute.String("messaging.operation", "produce"),
				attribute.String("messaging.destination.name", topic),
				attribute.String("messaging.kafka.message.key", string(msg.Key)),
			),
		)

		InjectTraceHeaders(ctx, &msg)

		if err := p.writer.WriteMessages(ctx, msg); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.End()
			metrics.KafkaMessagesProducedTotal.WithLabelValues(p.service, topic).Inc()
			return err
		}

		span.End()
		metrics.KafkaMessagesProducedTotal.WithLabelValues(p.service, topic).Inc()
	}
	return nil
}

func (p *Producer) Close() error {
	if closer, ok := p.writer.(*kafka.Writer); ok {
		return closer.Close()
	}
	return nil
}
