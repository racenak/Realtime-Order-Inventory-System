# Consistency and Transaction Management

## Overview

The system provides **strong consistency within services** and **eventual consistency across services** using the Outbox Pattern.

---

## Consistency Model

### Within Service (Strong Consistency)

All write operations use database transactions:

```go
tx, err := r.db.BeginTxx(ctx, nil)
defer tx.Rollback()

// Business data + outbox event written atomically
// ...

tx.Commit()
```

**Guarantee**: Business state and outbox event are written atomically. No lost events.

### Across Services (Eventual Consistency)

Services communicate via Kafka events. The flow:

1. Service A writes business data + outbox event (atomic)
2. Outbox publisher polls and publishes to Kafka (at-least-once)
3. Service B consumes event, processes, produces new event (at-least-once)
4. WebSocket service broadcasts to clients (best-effort)

**Guarantee**: All events are eventually processed. Idempotent consumers handle duplicates.

---

## Transaction Boundaries

### Order Service

| Operation | Transaction Scope |
|-----------|-------------------|
| CreateOrder | Create order + items + outbox event |
| CancelOrder | Update status + create history + outbox event |
| ConfirmOrder | Update status + create history + outbox event |

```go
// internal/order/usecase/create_order.go
func (u *OrderUseCase) CreateOrder(ctx context.Context, req domain.CreateOrderRequest) (*domain.Order, error) {
    tx, err := u.orderRepo.DB().BeginTxx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    // 1. Check idempotency key
    existing, _ := u.orderRepo.GetByIDempotencyKey(ctx, req.IdempotencyKey)
    if existing != nil {
        return nil, domain.ErrOrderAlreadyExists
    }

    // 2. Create order
    order := domain.NewOrder(req)
    if err := u.orderRepo.CreateInTx(ctx, tx, order); err != nil {
        return nil, err
    }

    // 3. Create order items
    for _, item := range order.Items {
        if err := u.orderItemRepo.CreateInTx(ctx, tx, item); err != nil {
            return nil, err
        }
    }

    // 4. Create outbox event
    outboxEvent := domain.NewOutboxEvent("order", order.ID, "order.created", order)
    if err := u.outboxRepo.CreateInTx(ctx, tx, outboxEvent); err != nil {
        return nil, err
    }

    return order, tx.Commit()
}
```

### Inventory Service

| Operation | Transaction Scope |
|-----------|-------------------|
| ReserveStock | Create reservation + update inventory + create movement |
| ReleaseReservation | Update reservation status + restore inventory + create movement |
| UpdateStock | Update inventory + create movement |

```go
// internal/inventory/usecase/reserve_stock.go
func (u *InventoryUseCase) ReserveStock(ctx context.Context, req domain.ReserveStockRequest) (*domain.Reservation, error) {
    tx, err := u.inventoryRepo.DB().BeginTxx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    // 1. Resolve warehouse code → ID
    warehouse, err := u.inventoryRepo.GetWarehouseByCode(ctx, req.WarehouseCode)
    if err != nil {
        return nil, err
    }

    // 2. Check stock availability
    inv, err := u.inventoryRepo.GetByProductAndWarehouse(ctx, req.ProductID, warehouse.ID)
    if err != nil {
        return nil, err
    }
    if inv.Available() < req.Quantity {
        return nil, domain.ErrInsufficientStock
    }

    // 3. Reserve stock (optimistic locking)
    if err := u.inventoryRepo.ReserveQuantity(ctx, tx, req.ProductID, warehouse.ID, req.Quantity); err != nil {
        return nil, err
    }

    // 4. Create reservation
    reservation := domain.NewReservation(req)
    if err := u.reservationRepo.CreateInTx(ctx, tx, reservation); err != nil {
        return nil, err
    }

    // 5. Create movement
    movement := domain.NewMovement(req, "reserve")
    if err := u.movementRepo.CreateInTx(ctx, tx, movement); err != nil {
        return nil, err
    }

    return reservation, tx.Commit()
}
```

---

## MessageWriter Interface (Kafka Decoupling)

The `MessageWriter` interface decouples Kafka components from the concrete `*kafka.Writer`:

```go
// pkg/kafka/producer.go
type MessageWriter interface {
    WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}
```

Both `*kafka.Writer` and `MockMessageWriter` implement this interface.

**Why**: Prevents data race when multiple goroutines share a writer. Topic is set on each message, not on the writer.

```go
// Topic set on message, not writer
msg := kafka.Message{
    Topic: topic,  // Per-message topic
    Key:   []byte(event.AggregateID),
    Value: payload,
}
return p.writer.WriteMessages(ctx, msg)
```

---

## DBExecutor Interface

```go
// pkg/database/postgres.go
type DBExecutor interface {
    ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}
```

Both `*sqlx.DB` and `*sqlx.Tx` implement this interface, enabling repository methods to work within or outside transactions.

---

## Outbox Pattern Flow

```
1. Service writes business data + outbox event (atomic, DB transaction)
2. Outbox publisher polls for PENDING events
3. ClaimBatch uses FOR UPDATE SKIP LOCKED (atomic batch claim)
4. Publisher writes to Kafka
5. On success: status → PUBLISHED
6. On failure (retry exhausted): status → FAILED, publish to DLQ
```

**Status Flow**: `PENDING → CLAIMED → PUBLISHED/FAILED`

---

## At-Least-Once Delivery

Kafka consumers must be idempotent because:

1. Producer may retry on network error (duplicate messages)
2. Consumer may process message but crash before offset commit (reprocess)
3. Outbox publisher may re-claim events if previous publisher crashed

**Mitigation**: Consumers use idempotent operations (upsert, conditional updates).

---

## Failure Scenarios

| Failure | Recovery |
|---------|----------|
| Service crashes mid-transaction | `defer tx.Rollback()` ensures cleanup |
| Outbox publisher crashes mid-batch | CLAIMED events re-claimed after timeout |
| Kafka publish fails | Retry with exponential backoff, then DLQ |
| Consumer crashes mid-processing | Offset not committed, message reprocessed |
| Redis cache failure | Fail-open, fall through to DB |

---

## What's Implemented

- [x] DB transactions for all write use cases (CreateOrder, CancelOrder, ConfirmOrder, ReserveStock, ReleaseReservation)
- [x] `defer tx.Rollback()` pattern in all write use cases
- [x] `DBExecutor` interface for transaction polymorphism
- [x] Outbox pattern with ClaimBatch (FOR UPDATE SKIP LOCKED)
- [x] At-least-once delivery with idempotent consumers
- [x] DLQ for failed events (after retry exhaustion)
- [x] `MessageWriter` interface for Kafka decoupling
- [x] Manual offset commit in Kafka consumer
