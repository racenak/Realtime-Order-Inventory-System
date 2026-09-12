# Event Design

## Overview

Events are the backbone of inter-service communication. The system uses **Kafka** as the event bus with the **Outbox Pattern** for reliable publishing.

---

## Event Types

| Event | Topic | Producer | Consumer | Description |
|-------|-------|----------|----------|-------------|
| `order.created` | `order.created` | Order Service | Inventory Service | Trigger stock reservation |
| `order.confirmed` | `order.confirmed` | Order Service | Inventory Service | Confirm stock deduction |
| `order.cancelled` | `order.cancelled` | Order Service | Inventory Service | Release reserved stock |
| `order.paid` | `order.paid` | Order Service | — | Payment confirmed |
| `order.shipped` | `order.shipped` | Order Service | — | Order shipped |
| `order.delivered` | `order.delivered` | Order Service | — | Order delivered |
| `order.status.changed` | `order.status.changed` | Order Service | WebSocket Service | Notify order progress |
| `inventory.reserved` | `inventory.reserved` | Inventory Service | Order Service | Confirm reservation success |
| `inventory.released` | `inventory.released` | Inventory Service | Order Service | Reservation released |
| `inventory.reservation_failed` | `inventory.reservation_failed` | Inventory Service | Order Service | Reservation failed |
| `inventory.updated` | `inventory.updated` | Inventory Service | WebSocket Service | Broadcast stock changes |
| `inventory.low_stock` | `inventory.low_stock` | Inventory Service | — | Low stock alert |

**Consumer config**: Manual offset commit (`CommitInterval: 0`), configurable retry (`MaxRetries: 3`, `RetryDelay: 1s`), DLQ on final failure.

---

## Event Schema

### CloudEvents-inspired (simplified)

```json
{
  "id": "evt_01HXYZ123456",
  "type": "order.created",
  "source": "order-service",
  "data": {
    "order_id": "01HXYZ1234567890ABCDEF01",
    "customer_id": "cust_01HXYZ123456",
    "items": [...],
    "total_amount": 1299.99
  },
  "timestamp": "2026-09-09T10:30:00Z"
}
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | Yes | Unique event ID (UUID v7) |
| type | string | Yes | Event type (e.g., `order.created`) |
| source | string | Yes | Service that produced the event |
| data | object | Yes | Event payload |
| timestamp | string | Yes | ISO 8601 timestamp |

---

## Outbox Pattern

### Flow

```
1. Service writes business data + outbox event (atomic, DB transaction)
2. Outbox publisher polls for PENDING events
3. ClaimBatch uses FOR UPDATE SKIP LOCKED (atomic batch claim)
4. Publisher writes to Kafka
5. On success: status → PUBLISHED
6. On failure (retry exhausted): status → FAILED
```

### Outbox Event Schema

```go
type OutboxEvent struct {
    ID            string
    AggregateType string    // "order" or "inventory"
    AggregateID   string    // ID of the aggregate root
    EventType     string    // e.g., "order.created"
    Payload       []byte    // JSON-encoded event data
    Status        string    // PENDING, CLAIMED, PUBLISHED, FAILED
    CreatedAt     time.Time
    PublishedAt   *time.Time
}
```

### Status Flow

```
PENDING → CLAIMED → PUBLISHED
                 → FAILED (after retry exhaustion)
```

---

## Topic Routing

Events are routed to topics based on their type:

```go
func getTopicForEvent(eventType string) string {
    switch {
    case strings.HasPrefix(eventType, "order."):
        return eventType // order.created → order.created topic
    case strings.HasPrefix(eventType, "inventory."):
        return eventType // inventory.reserved → inventory.reserved topic
    default:
        return "default"
    }
}
```

---

## Consumer Design

### Idempotency

Consumers must be idempotent because:
- Kafka provides at-least-once delivery
- Producer may retry on network error
- Consumer may crash before offset commit

**Strategy**: Use conditional updates (e.g., `UPDATE ... WHERE status = 'pending'`).

### Retry Logic

```go
func (c *Consumer) HandleWithRetry(ctx context.Context, msg kafka.Message) error {
    var lastErr error
    for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
        if attempt > 0 {
            time.Sleep(c.config.RetryDelay * time.Duration(attempt))
        }
        lastErr = c.handler(ctx, msg)
        if lastErr == nil {
            return nil
        }
    }
    // Exhausted retries → DLQ
    return c.SendToDLQ(ctx, msg, lastErr)
}
```

### DLQ (Dead Letter Queue)

Failed messages are published to `<topic>.dlq` with error metadata headers:

| Header | Value |
|--------|-------|
| `error-message` | Original error description |
| `retry-count` | Number of retries attempted |
| `original-topic` | Source topic name |
| `original-partition` | Source partition |
| `original-offset` | Source offset |
| `timestamp` | ISO 8601 timestamp |

---

## Event Handler Routing

### Order Service (consuming inventory events)

| Topic | Handler | Action |
|-------|---------|--------|
| `inventory.reserved` | `InventoryEventHandler` | Confirm order (update status to `confirmed`) |
| `inventory.reservation_failed` | `InventoryEventHandler` | Cancel order (update status to `cancelled`) |
| `inventory.released` | `InventoryEventHandler` | Log only (no order state change) |

### Inventory Service (consuming order events)

| Topic | Handler | Action |
|-------|---------|--------|
| `order.created` | `OrderEventHandler` | Reserve stock for each order item |
| `order.cancelled` | `OrderEventHandler` | Release reserved stock |

---

## Kafka Topics

### Auto-created Topics

The `kafka-init` container creates all required topics on startup:

```
order.created
order.confirmed
order.cancelled
order.paid
order.shipped
order.delivered
order.status.changed
inventory.reserved
inventory.released
inventory.reservation_failed
inventory.updated
inventory.low_stock
default
```

### DLQ Topics (created on first failed message)

```
order.created.dlq
inventory.reserved.dlq
...
```

---

## What's Implemented

- [x] 13 Kafka topics with auto-creation
- [x] Outbox pattern with ClaimBatch (FOR UPDATE SKIP LOCKED)
- [x] Topic routing based on event type prefix
- [x] Consumer retry with configurable max retries
- [x] DLQ publishing with error metadata headers
- [x] Manual offset commit
- [x] Order event handler (inventory service): order.created → reserve stock
- [x] Order event handler (inventory service): order.cancelled → release stock
- [x] Inventory event handler (order service): inventory.reserved → confirm order
- [x] Inventory event handler (order service): inventory.reservation_failed → cancel order
- [x] `MessageWriter` interface for testable Kafka components
