# Concurrency Control

## Overview

The system uses two concurrency control mechanisms depending on the use case:

1. **Optimistic Locking** — Inventory stock updates (low contention)
2. **Row-Level Locking (FOR UPDATE SKIP LOCKED)** — Outbox event processing (high contention)

---

## Optimistic Locking

### Purpose

Prevent lost updates when multiple requests try to modify the same inventory record simultaneously.

### Mechanism

Each inventory record has a `version` column. Updates must include the current version:

```sql
UPDATE inventory
SET quantity_on_hand = $1, version = version + 1, updated_at = NOW()
WHERE id = $2 AND version = $3
```

If `rows affected == 0`, another request modified the record first → retry or return error.

### Usage

```go
// internal/inventory/adapter/repository/postgres.go
func (r *InventoryRepository) UpdateStock(ctx context.Context, productID, warehouseID string, quantity int) error {
    query := `
        UPDATE inventory
        SET quantity_on_hand = $1, version = version + 1, updated_at = NOW()
        WHERE product_id = $2 AND warehouse_id = $3`

    result, err := r.db.ExecContext(ctx, query, quantity, productID, warehouseID)
    if err != nil {
        return err
    }

    rows, _ := result.RowsAffected()
    if rows == 0 {
        return domain.ErrInsufficientStock
    }
    return nil
}
```

### When to Use

- Inventory stock updates (reserve, release, update)
- Low contention scenarios
- When retries are cheap

### Trade-offs

| Pros | Cons |
|------|------|
| No blocking | Requires retry logic |
| Scalable | Wasted work on conflict |
| Simple implementation | Not suitable for high contention |

---

## Row-Level Locking (FOR UPDATE SKIP LOCKED)

### Purpose

Prevent duplicate processing of outbox events when multiple publisher instances poll simultaneously.

### Mechanism

```sql
SELECT * FROM outbox_events
WHERE status = 'PENDING'
ORDER BY created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED
```

- `FOR UPDATE` locks selected rows
- `SKIP LOCKED` skips rows already locked by another transaction
- Atomic: SELECT + UPDATE in single query

### Usage

```go
// internal/order/adapter/repository/outbox_postgres.go
func (r *OutboxRepository) ClaimBatch(ctx context.Context, batchSize int) ([]domain.OutboxEvent, error) {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer func() { _ = tx.Rollback() }()

    query := `
        SELECT id, aggregate_type, aggregate_id, event_type, payload, status, created_at
        FROM outbox_events
        WHERE status = 'PENDING'
        ORDER BY created_at ASC
        LIMIT $1
        FOR UPDATE SKIP LOCKED`

    var events []domain.OutboxEvent
    err = tx.SelectContext(ctx, &events, query, batchSize)
    if err != nil {
        return nil, err
    }

    if len(events) == 0 {
        return nil, nil
    }

    // Mark as CLAIMED
    ids := make([]string, len(events))
    for i, e := range events {
        ids[i] = e.ID
    }

    updateQuery := `
        UPDATE outbox_events
        SET status = 'CLAIMED', published_at = NOW()
        WHERE id = ANY($1)`

    _, err = tx.ExecContext(ctx, updateQuery, pq.Array(ids))
    if err != nil {
        return nil, err
    }

    return events, tx.Commit()
}
```

### When to Use

- Outbox event processing
- High contention scenarios
- When multiple workers process the same queue

### Trade-offs

| Pros | Cons |
|------|------|
| No duplicate processing | PostgreSQL row-level locks |
| Non-blocking (SKIP LOCKED) | Requires transaction |
| Atomic claim | Slightly complex implementation |

---

## Data Race Fix: WebSocket Hub

### Problem

The original `Hub.Unregister()` method was called inside `RLock()` (read lock), but `delete()` requires a write lock.

### Solution

Slow clients are sent to an `unregister` channel instead of calling `delete()` under RLock. The hub's run loop processes unregistrations under its own write lock.

```go
// pkg/websocket/hub.go
func (h *Hub) Unregister(client *Client) {
    select {
    case h.unregister <- client:
    default:
        // Channel full, client already being unregistered
    }
}
```

---

## Data Race Fix: Outbox Publisher Topic Mutation

### Problem

`OutboxPublisher` mutated `p.writer.Topic` before calling `WriteMessages`, causing a data race when multiple goroutines shared the same writer.

### Solution

Topic is now set on each `kafka.Message` directly, not on the writer:

```go
func (p *OutboxPublisher) PublishEvent(ctx context.Context, event domain.OutboxEvent) error {
    topic := p.getTopicForEvent(event.EventType)
    msg := kafka.Message{
        Topic: topic,  // Set on message, not writer
        Key:   []byte(event.AggregateID),
        // ...
    }
    return p.writer.WriteMessages(ctx, msg)
}
```

---

## DB Transaction Pattern

All write operations use explicit transactions:

```go
tx, err := r.db.BeginTxx(ctx, nil)
if err != nil {
    return err
}
defer func() { _ = tx.Rollback() }()  // No-op after commit

// ... business logic ...

if err := tx.Commit(); err != nil {
    return err
}
```

**Guarantee**: `defer func() { _ = tx.Rollback() }()` ensures cleanup on error. After `Commit()`, `Rollback()` is a no-op.

---

## Summary

| Mechanism | Use Case | Lock Type | Blocking |
|-----------|----------|-----------|----------|
| Optimistic Locking | Inventory updates | None (version check) | No |
| FOR UPDATE SKIP LOCKED | Outbox processing | Row-level | No (SKIP LOCKED) |
| Channel-based unregister | WebSocket hub | None (channel) | No |
| DB Transactions | All writes | Transaction | Brief |
