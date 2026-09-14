package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
)

type KafkaMessageCarrier struct {
	msg *kafka.Message
}

func NewKafkaMessageCarrier(msg *kafka.Message) *KafkaMessageCarrier {
	return &KafkaMessageCarrier{msg: msg}
}

func (c *KafkaMessageCarrier) Get(key string) string {
	for _, h := range c.msg.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *KafkaMessageCarrier) Set(key string, value string) {
	for i, h := range c.msg.Headers {
		if h.Key == key {
			c.msg.Headers[i].Value = []byte(value)
			return
		}
	}
	c.msg.Headers = append(c.msg.Headers, kafka.Header{
		Key:   key,
		Value: []byte(value),
	})
}

func (c *KafkaMessageCarrier) Keys() []string {
	keys := make([]string, len(c.msg.Headers))
	for i, h := range c.msg.Headers {
		keys[i] = h.Key
	}
	return keys
}

func InjectTraceHeaders(ctx context.Context, msg *kafka.Message) {
	carrier := NewKafkaMessageCarrier(msg)
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

func ExtractTraceContext(headers []kafka.Header) context.Context {
	if len(headers) == 0 {
		return nil
	}
	msg := kafka.Message{Headers: headers}
	carrier := NewKafkaMessageCarrier(&msg)
	return otel.GetTextMapPropagator().Extract(context.Background(), carrier)
}
