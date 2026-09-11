package mocks

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
)

type MockOutboxRepository struct {
	CreateFn       func(ctx context.Context, event domain.OutboxEvent) error
	CreateInTxFn   func(ctx context.Context, tx *sqlx.Tx, event domain.OutboxEvent) error
	GetPendingFn   func(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	ClaimBatchFn   func(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkPublishedFn func(ctx context.Context, id string) error
	MarkFailedFn   func(ctx context.Context, id string) error
}

func (m *MockOutboxRepository) Create(ctx context.Context, event domain.OutboxEvent) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, event)
	}
	return nil
}

func (m *MockOutboxRepository) CreateInTx(ctx context.Context, tx *sqlx.Tx, event domain.OutboxEvent) error {
	if m.CreateInTxFn != nil {
		return m.CreateInTxFn(ctx, tx, event)
	}
	return nil
}

func (m *MockOutboxRepository) GetPending(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	if m.GetPendingFn != nil {
		return m.GetPendingFn(ctx, limit)
	}
	return nil, nil
}

func (m *MockOutboxRepository) ClaimBatch(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	if m.ClaimBatchFn != nil {
		return m.ClaimBatchFn(ctx, limit)
	}
	return nil, nil
}

func (m *MockOutboxRepository) MarkPublished(ctx context.Context, id string) error {
	if m.MarkPublishedFn != nil {
		return m.MarkPublishedFn(ctx, id)
	}
	return nil
}

func (m *MockOutboxRepository) MarkFailed(ctx context.Context, id string) error {
	if m.MarkFailedFn != nil {
		return m.MarkFailedFn(ctx, id)
	}
	return nil
}
