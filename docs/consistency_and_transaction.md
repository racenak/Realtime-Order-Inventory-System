# Consistency and Transaction Design

## Overview

| Property | Value |
|----------|-------|
| Architecture | Event-Driven Microservices |
| Consistency Model | Eventual Consistency (strong where critical) |
| Transaction Pattern | Saga + Outbox |
| Distributed TX | Avoided (no 2PC) |

---

## Consistency Challenges

### The Problem

In a microservices architecture, a single business operation spans multiple services and databases:

```
Place Order
    ├── Order Service: Create order record
    ├── Inventory Service: Reserve stock
    └── Notification Service: Send confirmation

All three must succeed or all must fail.
```

### Why Not 2PC (Two-Phase Commit)?

| Issue | Impact |
|-------|--------|
| Single point of failure | Coordinator failure blocks all |
| Performance | Locks held during entire transaction |
| Scalability | Doesn't work across service boundaries |
| Availability | Reduced availability during coordination |

---

## Consistency Patterns Used

### 1. Saga Pattern

A sequence of local transactions with compensation for rollback.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         ORDER CREATION SAGA                              │
│                                                                         │
│  Step 1           Step 2           Step 3           Step 4             │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐        │
│  │ Create   │───▶│ Reserve  │───▶│ Confirm  │───▶│ Send     │        │
│  │ Order    │    │ Stock    │    │ Payment  │    │ Notif    │        │
│  └──────────┘    └──────────┘    └──────────┘    └──────────┘        │
│       │               │               │               │               │
│       ▼               ▼               ▼               ▼               │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐        │
│  │ Cancel   │◀───│ Release  │◀───│ Refund   │    │          │        │
│  │ Order    │    │ Stock    │    │ Payment  │    │          │        │
│  └──────────┘    └──────────┘    └──────────┘    └──────────┘        │
│                                                                         │
│                    COMPENSATION STEPS (on failure)                      │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2. Outbox Pattern

Guarantees event publication as part of the local transaction.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        OUTBOX PATTERN                                   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────┐       │
│  │                   LOCAL TRANSACTION                          │       │
│  │                                                             │       │
│  │  1. INSERT INTO orders (...)                                │       │
│  │  2. INSERT INTO outbox_events (type: 'order.created')      │       │
│  │                                                             │       │
│  │  COMMIT (atomic)                                            │       │
│  └─────────────────────────────────────────────────────────────┘       │
│                           │                                             │
│                           ▼                                             │
│  ┌─────────────────────────────────────────────────────────────┐       │
│  │                   POLLING PUBLISHER                          │       │
│  │                                                             │       │
│  │  SELECT * FROM outbox_events WHERE status = 'PENDING'      │       │
│  │  → Publish to Kafka                                         │       │
│  │  → UPDATE status = 'PUBLISHED'                              │       │
│  └─────────────────────────────────────────────────────────────┘       │
│                           │                                             │
│                           ▼                                             │
│                    ┌──────────────┐                                     │
│                    │    KAFKA      │                                     │
│                    └──────────────┘                                     │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3. Choreography vs Orchestration

#### Choreography (Used)

Services react to events independently.

```
Order Svc                Inventory Svc             Notification Svc
    │                         │                          │
    │──order.created────────▶│                          │
    │                         │──inventory.reserved────▶│
    │◀──────────────────────────inventory.reserved──────│
    │                         │                          │
    │──order.confirmed─────────────────────────────────▶│
    │                         │                          │
```

**Pros:** Loose coupling, no single point of failure
**Cons:** Harder to track flow, cyclic dependencies possible

#### Orchestration (Alternative)

Central coordinator manages the flow.

```
                    ┌──────────────────┐
                    │  Saga Orchestrator│
                    └────────┬─────────┘
                             │
           ┌─────────────────┼─────────────────┐
           ▼                 ▼                 ▼
      Order Svc        Inventory Svc      Payment Svc
```

---

## Transaction Boundaries

### Order Service Transaction

```sql
BEGIN;

-- 1. Create order
INSERT INTO orders (id, customer_id, status, ...)
VALUES ('ord_001', 'usr_001', 'pending_payment', ...);

-- 2. Create order items
INSERT INTO order_items (id, order_id, product_id, ...)
VALUES ('itm_001', 'ord_001', 'prod_001', ...);

-- 3. Create outbox event
INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, status)
VALUES (uuidv7(), 'order', 'ord_001', 'order.created', '{...}', 'PENDING');

COMMIT;
```

### Inventory Service Transaction

```sql
BEGIN;

-- 1. Check available stock
SELECT quantity_on_hand, quantity_reserved
FROM inventory
WHERE product_id = 'prod_001' AND warehouse_id = 'wh_nyc'
FOR UPDATE;  -- Pessimistic lock

-- 2. Reserve stock
UPDATE inventory
SET quantity_reserved = quantity_reserved + 2,
    version = version + 1
WHERE product_id = 'prod_001' AND warehouse_id = 'wh_nyc'
  AND (quantity_on_hand - quantity_reserved) >= 2;  -- Optimistic check

-- 3. Create reservation record
INSERT INTO inventory_reservations (...)
VALUES (...);

-- 4. Create outbox event
INSERT INTO outbox_events (...)
VALUES (...);

COMMIT;
```

---

## Compensation Logic

### Compensation Triggers

| Failure Point | Compensation | Trigger |
|---------------|--------------|---------|
| Inventory reservation fails | Cancel order | inventory.reservation_failed |
| Payment fails | Release stock, cancel order | payment.failed |
| Order cancelled | Release stock | order.cancelled |
| Reservation expired | Release stock, cancel order | timeout |

### Compensation Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    COMPENSATION: Order Cancel                            │
│                                                                         │
│  1. Receive order.cancelled event                                       │
│                                                                         │
│  2. Inventory Service:                                                  │
│     BEGIN;                                                              │
│     UPDATE inventory SET quantity_reserved = quantity_reserved - qty    │
│     WHERE product_id = ? AND warehouse_id = ?;                         │
│     UPDATE inventory_reservations SET status = 'released'              │
│     WHERE id = ?;                                                       │
│     INSERT INTO inventory_movements (...);                              │
│     INSERT INTO outbox_events (event_type: 'inventory.released');      │
│     COMMIT;                                                             │
│                                                                         │
│  3. Order Service:                                                      │
│     BEGIN;                                                              │
│     UPDATE orders SET status = 'cancelled'                             │
│     WHERE id = ? AND status NOT IN ('shipped', 'delivered');           │
│     INSERT INTO order_status_history (...);                             │
│     INSERT INTO outbox_events (event_type: 'order.status.changed');   │
│     COMMIT;                                                             │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Idempotent Compensation

Compensation must be idempotent (safe to retry):

```go
func (s *InventoryService) ReleaseReservation(ctx context.Context, reservationID string) error {
    // Check current status first
    reservation, err := s.repo.GetReservation(ctx, reservationID)
    if err != nil {
        return err
    }

    // Idempotent: already released or fulfilled
    if reservation.Status == "released" || reservation.Status == "fulfilled" {
        return nil
    }

    // Only release if currently reserved
    if reservation.Status != "reserved" {
        return fmt.Errorf("unexpected reservation status: %s", reservation.Status)
    }

    // Release stock
    return s.repo.ReleaseReservation(ctx, reservationID)
}
```

---

## Consistency Guarantees

### By Operation

| Operation | Consistency Level | Mechanism |
|-----------|-------------------|-----------|
| Create order | Strong | Local transaction + Outbox |
| Reserve stock | Strong | Local transaction + Optimistic lock |
| Payment processing | Strong | Payment service transaction |
| Order confirmation | Eventual | Saga + Compensation |
| Stock update broadcast | Eventual | Kafka + Consumer |
| Real-time notification | Eventual | WebSocket + Redis Pub/Sub |

### Order Lifecycle Consistency

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     STATE MACHINE                                        │
│                                                                         │
│  ┌──────────────┐                                                       │
│  │   CREATED    │                                                       │
│  └──────┬───────┘                                                       │
│         │                                                               │
│         ▼                                                               │
│  ┌──────────────┐     ┌──────────────┐                                 │
│  │   PENDING    │────▶│    PAID      │                                 │
│  │   PAYMENT    │     └──────┬───────┘                                 │
│  └──────┬───────┘            │                                          │
│         │                    │                                          │
│         │ timeout (15min)    │ payment confirmed                        │
│         ▼                    ▼                                          │
│  ┌──────────────┐     ┌──────────────┐                                 │
│  │  CANCELLED   │◀───│  PROCESSING  │                                 │
│  └──────────────┘     └──────┬───────┘                                 │
│                              │                                          │
│                              │ shipped                                  │
│                              ▼                                          │
│                         ┌──────────────┐                               │
│                         │   SHIPPED    │                               │
│                         └──────┬───────┘                               │
│                                │                                        │
│                                │ delivered                              │
│                                ▼                                        │
│                         ┌──────────────┐                               │
│                         │  DELIVERED   │                               │
│                         └──────────────┘                               │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Inventory Consistency

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    INVENTORY STATE TRANSITIONS                          │
│                                                                         │
│  ┌────────────────────────────────────────────────────────────────┐    │
│  │                                                                │    │
│  │   quantity_on_hand: 200                                        │    │
│  │   quantity_reserved: 10  ← (reserved for order A)             │    │
│  │   available: 190          ← (200 - 10)                        │    │
│  │                                                                │    │
│  └────────────────────────────────────────────────────────────────┘    │
│                                                                         │
│  Operations:                                                            │
│  ┌─────────────────┬────────────────────┬────────────────────────┐     │
│  │ Operation       │ quantity_on_hand   │ quantity_reserved      │     │
│  ├─────────────────┼────────────────────┼────────────────────────┤     │
│  │ Reserve         │ unchanged          │ +quantity              │     │
│  │ Confirm (ship)  │ -quantity          │ -quantity              │     │
│  │ Release         │ unchanged          │ -quantity              │     │
│  │ Restock         │ +quantity          │ unchanged              │     │
│  │ Adjust          │ ±quantity          │ unchanged              │     │
│  └─────────────────┴────────────────────┴────────────────────────┘     │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Optimistic Locking

### Implementation

```go
// UpdateInventoryWithLock uses optimistic locking via version field
func (r *InventoryRepo) UpdateInventoryWithLock(
    ctx context.Context,
    productID string,
    warehouseID string,
    quantityChange int,
    expectedVersion int64,
) error {
    result, err := r.db.ExecContext(ctx, `
        UPDATE inventory
        SET quantity_on_hand = quantity_on_hand + $1,
            version = version + 1,
            updated_at = NOW()
        WHERE product_id = $2
          AND warehouse_id = $3
          AND version = $4`,
        quantityChange, productID, warehouseID, expectedVersion,
    )
    if err != nil {
        return err
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return err
    }

    if rows == 0 {
        return ErrConcurrentModification
    }

    return nil
}
```

### Handling Conflicts

```go
func (s *InventoryService) UpdateStock(ctx context.Context, req UpdateStockRequest) error {
    maxRetries := 3

    for i := 0; i < maxRetries; i++ {
        // Get current version
        inventory, err := s.repo.GetInventory(ctx, req.ProductID, req.WarehouseID)
        if err != nil {
            return err
        }

        // Attempt update
        err = s.repo.UpdateInventoryWithLock(
            ctx, req.ProductID, req.WarehouseID,
            req.QuantityChange, inventory.Version,
        )

        if err == ErrConcurrentModification {
            // Retry with exponential backoff
            time.Sleep(time.Duration(1<<i) * 100 * time.Millisecond)
            continue
        }

        if err != nil {
            return err
        }

        return nil
    }

    return ErrMaxRetriesExceeded
}
```

---

## Distributed Locking (Redis)

For operations requiring cross-service coordination:

```go
// Distributed lock for reservation expiry
func (s *InventoryService) ProcessExpiredReservations(ctx context.Context) error {
    lockKey := "lock:reservation:expiry"
    lockTTL := 30 * time.Second

    // Acquire lock
    acquired, err := s.redis.SetNX(ctx, lockKey, "1", lockTTL).Result()
    if err != nil || !acquired {
        return nil // Another instance is processing
    }

    defer s.redis.Del(ctx, lockKey)

    // Process expired reservations
    expired, err := s.repo.GetExpiredReservations(ctx)
    if err != nil {
        return err
    }

    for _, reservation := range expired {
        if err := s.ReleaseReservation(ctx, reservation.ID); err != nil {
            s.logger.Error("failed to release expired reservation",
                zap.String("reservation_id", reservation.ID),
                zap.Error(err),
            )
            continue
        }
    }

    return nil
}
```

---

## Eventual Consistency Patterns

### 1. Read-Your-Writes

After a write, subsequent reads reflect the write.

```go
func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
    // Create order in database
    order, err := s.repo.CreateOrder(ctx, req)
    if err != nil {
        return nil, err
    }

    // Publish event
    if err := s.publisher.Publish(ctx, "order.created", order); err != nil {
        s.logger.Error("failed to publish event", zap.Error(err))
    }

    // Return order directly (not waiting for event processing)
    return order, nil
}
```

### 2. CQRS (Command Query Responsibility Segregation)

Separate read and write models for scalability.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           CQRS PATTERN                                  │
│                                                                         │
│  Commands (Writes)              Queries (Reads)                        │
│  ┌──────────────────┐          ┌──────────────────┐                   │
│  │                  │          │                  │                   │
│  │  Order Service   │─────────▶│  Read Database   │                   │
│  │  (PostgreSQL)    │  events  │  (Redis/ES)      │                   │
│  │                  │          │                  │                   │
│  └──────────────────┘          └────────┬─────────┘                   │
│                                         │                              │
│                                         ▼                              │
│                                ┌──────────────────┐                   │
│                                │  API Responses   │                   │
│                                └──────────────────┘                   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3. Cache Invalidation

```go
func (s *InventoryService) UpdateStock(ctx context.Context, req UpdateStockRequest) error {
    // Update database
    if err := s.repo.UpdateStock(ctx, req); err != nil {
        return err
    }

    // Invalidate cache
    cacheKey := fmt.Sprintf("inventory:%s", req.ProductID)
    s.redis.Del(ctx, cacheKey)

    // Publish event for other services
    s.publisher.Publish(ctx, "inventory.updated", ...)

    return nil
}
```

---

## Failure Scenarios and Handling

### Scenario 1: Inventory Service Down

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Order Service                    Inventory Service                     │
│  ┌──────────────┐                 ┌──────────────┐                    │
│  │ Create order │ ───────────────▶│              │  DOWN              │
│  │ + outbox     │   Connection    │              │                    │
│  └──────────────┘   refused       └──────────────┘                    │
│         │                                                                    │
│         │ Order created, event in outbox (PENDING)                         │
│         │                                                                    │
│         ▼                                                                    │
│  ┌──────────────┐                                                           │
│  │ Retry Logic  │                                                           │
│  │ (Polling     │                                                           │
│  │  Publisher)  │                                                           │
│  └──────────────┘                                                           │
│         │                                                                    │
│         │ Inventory comes back up                                           │
│         │                                                                    │
│         ▼                                                                    │
│  ┌──────────────┐                                                           │
│  │ Event        │                                                           │
│  │ Published    │                                                           │
│  └──────────────┘                                                           │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Scenario 2: Reservation Expires

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│  1. Order created, stock reserved (expires_at: now + 15min)            │
│                                                                         │
│  2. Payment processing takes > 15 minutes                              │
│                                                                         │
│  3. Reservation expires automatically                                   │
│     ┌─────────────────────────────────────────────────┐                │
│     │ UPDATE inventory_reservations                   │                │
│     │ SET status = 'expired'                          │                │
│     │ WHERE status = 'reserved' AND expires_at < NOW()│                │
│     └─────────────────────────────────────────────────┘                │
│                                                                         │
│  4. Compensation: Release stock back to available                      │
│                                                                         │
│  5. Order marked as cancelled due to reservation expiry                │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Scenario 3: Duplicate Event Processing

```
┌─────────────────────────────────────────────────────────────────────────┐
│                                                                         │
│  1. order.created event published twice (at-least-once)                │
│                                                                         │
│  2. Consumer checks Redis:                                              │
│     ┌─────────────────────────────────────────────────┐                │
│     │ EXISTS processed_events:inventory-svc:evt_001   │                │
│     │ → 0 (not processed)                             │                │
│     └─────────────────────────────────────────────────┘                │
│                                                                         │
│  3. Process event                                                       │
│                                                                         │
│  4. Mark as processed:                                                  │
│     ┌─────────────────────────────────────────────────┐                │
│     │ SET processed_events:inventory-svc:evt_001 "1"  │                │
│     │ EX 86400                                         │                │
│     └─────────────────────────────────────────────────┘                │
│                                                                         │
│  5. Duplicate arrives, already marked → skip                           │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Data Consistency Checks

### Periodic Reconciliation

```go
// ReconcileInventory runs periodically to ensure consistency
func (s *InventoryService) ReconcileInventory(ctx context.Context) error {
    // Compare reservation sum with inventory.reserved
    discrepancies, err := s.repo.FindDiscrepancies(ctx)
    if err != nil {
        return err
    }

    for _, d := range discrepancies {
        s.logger.Warn("inventory discrepancy found",
            zap.String("product_id", d.ProductID),
            zap.String("warehouse_id", d.WarehouseID),
            zap.Int("expected_reserved", d.ExpectedReserved),
            zap.Int("actual_reserved", d.ActualReserved),
        )

        // Auto-correct or alert
        if err := s.repo.CorrectReservation(ctx, d); err != nil {
            s.logger.Error("failed to correct discrepancy", zap.Error(err))
        }
    }

    return nil
}
```

### Consistency Metrics

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `outbox_pending_count` | Events waiting to be published | > 100 |
| `outbox_age_seconds` | Age of oldest pending event | > 60s |
| `reservation_expired_count` | Reservations expired in last hour | > 10 |
| `consistency_check_failures` | Reconciliation failures | > 0 |
| `compensation_triggered_count` | Compensations in last hour | > 5 |

---

## Summary

| Pattern | Usage | Guarantee |
|---------|-------|-----------|
| Saga | Order creation, cancellation | Eventual consistency with compensation |
| Outbox | All state changes | Atomic write + event publication |
| Optimistic Locking | Inventory updates | Conflict detection |
| Idempotency | Event processing | Exactly-once semantics |
| Distributed Lock | Scheduled tasks | Mutual exclusion |
| CQRS | Read-heavy operations | Scalable reads |
| Cache Invalidation | Stock queries | Fresh data |
