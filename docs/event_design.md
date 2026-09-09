# Event Design

## Overview

| Property | Value |
|----------|-------|
| Message Broker | Apache Kafka |
| Serialization | JSON |
| Delivery | At-least-once |
| Ordering | Per partition (by aggregate_id) |

## Event Bus Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              KAFKA CLUSTER                                       │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                           Topics                                        │   │
│  │                                                                         │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                 │   │
│  │  │order.created │  │order.paid    │  │order.cancelled│                │   │
│  │  │  (3 parts)   │  │  (3 parts)   │  │  (3 parts)   │                 │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘                 │   │
│  │                                                                         │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                 │   │
│  │  │inventory.    │  │inventory.    │  │inventory.    │                 │   │
│  │  │reserved      │  │updated       │  │low_stock     │                 │   │
│  │  │  (3 parts)   │  │  (3 parts)   │  │  (3 parts)   │                 │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘                 │   │
│  │                                                                         │   │
│  │  ┌──────────────┐  ┌──────────────┐                                   │   │
│  │  │order.        │  │notification. │                                   │   │
│  │  │status.changed│  │send          │                                   │   │
│  │  │  (3 parts)   │  │  (3 parts)   │                                   │   │
│  │  └──────────────┘  └──────────────┘                                   │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                        Consumer Groups                                  │   │
│  │                                                                         │   │
│  │  ┌────────────────┐  ┌────────────────┐  ┌────────────────┐           │   │
│  │  │inventory-svc   │  │order-svc       │  │websocket-svc   │           │   │
│  │  │consumer-group  │  │consumer-group  │  │consumer-group  │           │   │
│  │  └────────────────┘  └────────────────┘  └────────────────┘           │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Topics

| Topic | Partitions | Retention | Consumers |
|-------|------------|-----------|-----------|
| `order.created` | 3 | 7 days | inventory-svc |
| `order.paid` | 3 | 7 days | inventory-svc |
| `order.confirmed` | 3 | 7 days | inventory-svc |
| `order.cancelled` | 3 | 7 days | inventory-svc |
| `order.shipped` | 3 | 7 days | notification-svc |
| `order.delivered` | 3 | 7 days | notification-svc |
| `order.status.changed` | 3 | 7 days | websocket-svc |
| `inventory.reserved` | 3 | 7 days | order-svc |
| `inventory.released` | 3 | 7 days | order-svc |
| `inventory.updated` | 3 | 7 days | websocket-svc |
| `inventory.low_stock` | 3 | 30 days | notification-svc |
| `notification.send` | 3 | 7 days | notification-svc |
| `dead_letter` | 3 | 30 days | monitoring |

---

## Event Schemas

### Order Events

#### order.created

Triggered when a new order is placed.

```json
{
  "event_id": "evt_01H1234567890ABCDE",
  "event_type": "order.created",
  "event_version": "1.0",
  "timestamp": "2024-01-15T10:30:00Z",
  "producer": "order-service",
  "correlation_id": "corr_abc123",
  "data": {
    "order_id": "ord_abc123",
    "customer_id": "usr_xyz789",
    "status": "pending_payment",
    "currency": "USD",
    "subtotal": 109.97,
    "discount_amount": 0.00,
    "shipping_fee": 9.99,
    "tax_amount": 8.80,
    "total_amount": 118.77,
    "items": [
      {
        "order_item_id": "itm_001",
        "product_id": "prod_001",
        "sku": "WGT-001",
        "product_name": "Widget A",
        "quantity": 2,
        "unit_price": 29.99,
        "total_price": 59.98
      },
      {
        "order_item_id": "itm_002",
        "product_id": "prod_002",
        "sku": "GDG-002",
        "product_name": "Gadget B",
        "quantity": 1,
        "unit_price": 49.99,
        "total_price": 49.99
      }
    ],
    "shipping_address": {
      "street": "123 Main St",
      "city": "New York",
      "state": "NY",
      "zip": "10001",
      "country": "US"
    }
  }
}
```

---

#### order.paid

Triggered when payment is confirmed.

```json
{
  "event_id": "evt_01H1234567890ABCDEF",
  "event_type": "order.paid",
  "event_version": "1.0",
  "timestamp": "2024-01-15T10:31:15Z",
  "producer": "order-service",
  "correlation_id": "corr_abc123",
  "data": {
    "order_id": "ord_abc123",
    "customer_id": "usr_xyz789",
    "status": "paid",
    "payment": {
      "payment_id": "pay_xyz789",
      "amount": 118.77,
      "currency": "USD",
      "method": "credit_card",
      "paid_at": "2024-01-15T10:31:15Z"
    }
  }
}
```

---

#### order.confirmed

Triggered after inventory reservation is successful.

```json
{
  "event_id": "evt_01H1234567890ABCDF0",
  "event_type": "order.confirmed",
  "event_version": "1.0",
  "timestamp": "2024-01-15T10:35:00Z",
  "producer": "order-service",
  "correlation_id": "corr_abc123",
  "data": {
    "order_id": "ord_abc123",
    "customer_id": "usr_xyz789",
    "status": "processing",
    "reservations": [
      {
        "reservation_id": "rsv_001",
        "product_id": "prod_001",
        "warehouse_id": "wh_nyc",
        "quantity": 2
      },
      {
        "reservation_id": "rsv_002",
        "product_id": "prod_002",
        "warehouse_id": "wh_nyc",
        "quantity": 1
      }
    ]
  }
}
```

---

#### order.cancelled

Triggered when an order is cancelled.

```json
{
  "event_id": "evt_01H1234567890ABCDG1",
  "event_type": "order.cancelled",
  "event_version": "1.0",
  "timestamp": "2024-01-15T11:00:00Z",
  "producer": "order-service",
  "correlation_id": "corr_abc123",
  "data": {
    "order_id": "ord_abc123",
    "customer_id": "usr_xyz789",
    "status": "cancelled",
    "cancelled_by": "customer",
    "reason": "Changed my mind",
    "refund_required": true,
    "reservation_ids": ["rsv_001", "rsv_002"]
  }
}
```

---

#### order.shipped

Triggered when an order is shipped.

```json
{
  "event_id": "evt_01H1234567890ABCDG2",
  "event_type": "order.shipped",
  "event_version": "1.0",
  "timestamp": "2024-01-15T14:00:00Z",
  "producer": "order-service",
  "correlation_id": "corr_abc123",
  "data": {
    "order_id": "ord_abc123",
    "customer_id": "usr_xyz789",
    "status": "shipped",
    "shipment": {
      "carrier": "UPS",
      "tracking_number": "TRK123456",
      "estimated_delivery": "2024-01-18",
      "shipped_at": "2024-01-15T14:00:00Z"
    }
  }
}
```

---

#### order.status.changed

Triggered on any status change for WebSocket broadcast.

```json
{
  "event_id": "evt_01H1234567890ABCDG3",
  "event_type": "order.status.changed",
  "event_version": "1.0",
  "timestamp": "2024-01-15T14:00:00Z",
  "producer": "order-service",
  "correlation_id": "corr_abc123",
  "data": {
    "order_id": "ord_abc123",
    "customer_id": "usr_xyz789",
    "previous_status": "processing",
    "new_status": "shipped",
    "tracking_number": "TRK123456",
    "updated_at": "2024-01-15T14:00:00Z"
  }
}
```

---

### Inventory Events

#### inventory.reserved

Triggered when stock is successfully reserved.

```json
{
  "event_id": "evt_01H1234567890ABCDG4",
  "event_type": "inventory.reserved",
  "event_version": "1.0",
  "timestamp": "2024-01-15T10:33:00Z",
  "producer": "inventory-service",
  "correlation_id": "corr_abc123",
  "data": {
    "order_id": "ord_abc123",
    "reservations": [
      {
        "reservation_id": "rsv_001",
        "product_id": "prod_001",
        "product_name": "Widget A",
        "sku": "WGT-001",
        "warehouse_id": "wh_nyc",
        "warehouse_name": "New York Warehouse",
        "quantity": 2,
        "expires_at": "2024-01-15T10:48:00Z"
      },
      {
        "reservation_id": "rsv_002",
        "product_id": "prod_002",
        "product_name": "Gadget B",
        "sku": "GDG-002",
        "warehouse_id": "wh_nyc",
        "warehouse_name": "New York Warehouse",
        "quantity": 1,
        "expires_at": "2024-01-15T10:48:00Z"
      }
    ]
  }
}
```

---

#### inventory.released

Triggered when reserved stock is released.

```json
{
  "event_id": "evt_01H1234567890ABCDG5",
  "event_type": "inventory.released",
  "event_version": "1.0",
  "timestamp": "2024-01-15T11:05:00Z",
  "producer": "inventory-service",
  "correlation_id": "corr_abc123",
  "data": {
    "order_id": "ord_abc123",
    "released_items": [
      {
        "reservation_id": "rsv_001",
        "product_id": "prod_001",
        "warehouse_id": "wh_nyc",
        "quantity": 2
      },
      {
        "reservation_id": "rsv_002",
        "product_id": "prod_002",
        "warehouse_id": "wh_nyc",
        "quantity": 1
      }
    ]
  }
}
```

---

#### inventory.updated

Triggered when stock levels change.

```json
{
  "event_id": "evt_01H1234567890ABCDG6",
  "event_type": "inventory.updated",
  "event_version": "1.0",
  "timestamp": "2024-01-15T12:00:00Z",
  "producer": "inventory-service",
  "correlation_id": "corr_xyz456",
  "data": {
    "product_id": "prod_001",
    "product_name": "Widget A",
    "sku": "WGT-001",
    "warehouse_id": "wh_nyc",
    "warehouse_name": "New York Warehouse",
    "previous": {
      "quantity": 200,
      "reserved": 10,
      "available": 190
    },
    "current": {
      "quantity": 250,
      "reserved": 10,
      "available": 240
    },
    "movement": {
      "movement_id": "mov_xyz123",
      "type": "in",
      "quantity": 50,
      "reference_type": "restock",
      "reference_id": "po_abc123"
    }
  }
}
```

---

#### inventory.low_stock

Triggered when stock falls below threshold.

```json
{
  "event_id": "evt_01H1234567890ABCDG7",
  "event_type": "inventory.low_stock",
  "event_version": "1.0",
  "timestamp": "2024-01-15T14:10:00Z",
  "producer": "inventory-service",
  "correlation_id": "corr_def789",
  "data": {
    "product_id": "prod_001",
    "product_name": "Widget A",
    "sku": "WGT-001",
    "warehouse_id": "wh_nyc",
    "warehouse_name": "New York Warehouse",
    "current_stock": 45,
    "threshold": 50,
    "alert_level": "warning",
    "suggested_action": "reorder"
  }
}
```

---

### Notification Events

#### notification.send

Triggered to send email/SMS notifications.

```json
{
  "event_id": "evt_01H1234567890ABCDG8",
  "event_type": "notification.send",
  "event_version": "1.0",
  "timestamp": "2024-01-15T14:00:01Z",
  "producer": "notification-service",
  "correlation_id": "corr_abc123",
  "data": {
    "notification_id": "ntf_abc123",
    "recipient": {
      "user_id": "usr_xyz789",
      "email": "customer@example.com",
      "phone": "+1234567890"
    },
    "channels": ["email", "sms"],
    "template": "order_shipped",
    "variables": {
      "customer_name": "John Doe",
      "order_id": "ord_abc123",
      "tracking_number": "TRK123456",
      "carrier": "UPS",
      "estimated_delivery": "2024-01-18"
    }
  }
}
```

---

## Event Flow Diagrams

### Order Creation Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│    Client     │     │  Order Svc   │     │Inventory Svc │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │ POST /orders       │                     │
       │───────────────────▶│                     │
       │                    │                     │
       │                    │ Create order        │
       │                    │ Save outbox         │
       │                    │─────────┐           │
       │                    │◀────────┘           │
       │                    │                     │
       │                    │ Publish             │
       │                    │ order.created       │
       │                    │────────────────────▶│
       │                    │                     │
       │                    │                     │ Reserve stock
       │                    │                     │─────────┐
       │                    │                     │◀────────┘
       │                    │                     │
       │                    │ Publish             │
       │                    │ inventory.reserved  │
       │                    │◀────────────────────│
       │                    │                     │
       │  201 Created       │                     │
       │◀───────────────────│                     │
       │                    │                     │
```

### Order Cancellation Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│    Client     │     │  Order Svc   │     │Inventory Svc │     │ WebSocket Svc│
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │ POST /cancel       │                     │                     │
       │───────────────────▶│                     │                     │
       │                    │                     │                     │
       │                    │ Publish             │                     │
       │                    │ order.cancelled     │                     │
       │                    │────────────────────▶│                     │
       │                    │                     │                     │
       │                    │                     │ Release stock       │
       │                    │                     │─────────┐           │
       │                    │                     │◀────────┘           │
       │                    │                     │                     │
       │                    │ Publish             │                     │
       │                    │ inventory.released  │                     │
       │                    │◀────────────────────│                     │
       │                    │                     │                     │
       │                    │ Publish             │                     │
       │                    │ order.status.changed│                     │
       │                    │─────────────────────────────────────────▶│
       │                    │                     │                     │
       │                    │                     │    Broadcast to     │
       │                    │                     │    client (WS)      │
       │                    │                     │                     │
       │  200 OK            │                     │                     │
       │◀───────────────────│                     │                     │
```

### Low Stock Alert Flow

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│Inventory Svc │     │ Notification │     │    Admin      │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │                    │                     │
       │ Stock updated      │                     │
       │ (below threshold)  │                     │
       │─────────┐          │                     │
       │◀────────┘          │                     │
       │                    │                     │
       │ Publish            │                     │
       │ inventory.low_stock│                     │
       │───────────────────▶│                     │
       │                    │                     │
       │                    │ Send email alert    │
       │                    │────────────────────▶│
       │                    │                     │
```

---

## Partitioning Strategy

| Topic | Partition Key | Reason |
|-------|---------------|--------|
| `order.*` | `order_id` | Ensure ordering per order |
| `inventory.*` | `product_id` | Ensure ordering per product |
| `notification.*` | `user_id` | Ensure ordering per user |

**Partition Assignment:**

```
partition = hash(partition_key) % num_partitions
```

---

## Consumer Configuration

| Consumer Group | Topics | Offset Reset | Max Poll |
|----------------|--------|--------------|----------|
| `inventory-svc` | order.created, order.paid, order.cancelled | latest | 100 |
| `order-svc` | inventory.reserved, inventory.released | latest | 100 |
| `websocket-svc` | order.status.changed, inventory.updated | latest | 50 |
| `notification-svc` | order.shipped, inventory.low_stock | latest | 50 |

---

## Error Handling

### Retry Policy

```go
type RetryConfig struct {
    MaxRetries     int           // 3
    InitialBackoff time.Duration // 1s
    MaxBackoff     time.Duration // 30s
    Multiplier     float64       // 2.0
}
```

### Dead Letter Queue

Failed events after max retries are sent to `dead_letter` topic.

```json
{
  "original_topic": "order.created",
  "original_partition": 0,
  "original_offset": 12345,
  "original_event": { ... },
  "error": "inventory service unavailable",
  "retry_count": 3,
  "first_attempt_at": "2024-01-15T10:30:00Z",
  "last_attempt_at": "2024-01-15T10:35:00Z"
}
```

---

## Idempotency

### Event ID Deduplication

Each event has a unique `event_id`. Consumers must track processed IDs.

**Storage: Redis**

```
Key:   processed_events:{consumer_group}:{event_id}
TTL:   24 hours
Value: "processed"
```

### Idempotent Processing

```go
func (c *Consumer) HandleEvent(ctx context.Context, event Event) error {
    // Check if already processed
    if c.redis.Exists(ctx, fmt.Sprintf("processed_events:%s:%s", c.groupID, event.EventID)) {
        return nil // Already processed
    }

    // Process event
    if err := c.process(ctx, event); err != nil {
        return err
    }

    // Mark as processed
    c.redis.Set(ctx, fmt.Sprintf("processed_events:%s:%s", c.groupID, event.EventID), "processed", 24*time.Hour)

    return nil
}
```

---

## Event Versioning

### Version Format

```
event_version: "1.0"
```

### Schema Evolution Rules

1. **Add field**: New field must be optional
2. **Remove field**: Deprecate first, remove in next major version
3. **Rename field**: Add new field, deprecate old, remove in next major
4. **Breaking change**: Increment major version, run parallel consumers

### Version Header

```json
{
  "event_type": "order.created",
  "event_version": "1.0",
  "min_compatible_version": "1.0"
}
```

---

## Monitoring

### Key Metrics

| Metric | Description |
|--------|-------------|
| `kafka_consumer_lag` | Messages pending per partition |
| `kafka_producer_errors` | Failed publish attempts |
| `event_processing_duration` | Time to process each event |
| `dead_letter_queue_size` | Events in DLQ |

### Alerts

| Alert | Condition | Severity |
|-------|-----------|----------|
| High Consumer Lag | lag > 1000 | Warning |
| Consumer Group Down | no active consumers | Critical |
| DLQ Growing | dlq_size > 100 | Warning |
| Event Processing Slow | duration > 5s | Warning |
