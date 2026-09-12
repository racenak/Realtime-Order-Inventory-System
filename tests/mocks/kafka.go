package mocks

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type MockMessageWriter struct {
	WriteMessagesFn func(ctx context.Context, msgs ...kafka.Message) error
}

func (m *MockMessageWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	if m.WriteMessagesFn != nil {
		return m.WriteMessagesFn(ctx, msgs...)
	}
	return nil
}
