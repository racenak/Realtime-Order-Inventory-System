# Error Handling

## Error Model

All services return errors in a consistent JSON format:

```json
{
  "error": {
    "code": "ORDER_NOT_FOUND",
    "message": "order not found"
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Invalid request body, missing fields |
| `ORDER_NOT_FOUND` | 404 | Order doesn't exist |
| `ORDER_ALREADY_EXISTS` | 409 | Duplicate idempotency key |
| `ORDER_CANNOT_CANCEL` | 409 | Order in non-cancellable state |
| `ORDER_CANNOT_CONFIRM` | 409 | Order in non-confirmable state |
| `INVENTORY_NOT_FOUND` | 404 | Inventory record not found |
| `INSUFFICIENT_STOCK` | 409 | Not enough stock to reserve |
| `RESERVATION_NOT_FOUND` | 404 | Reservation doesn't exist |
| `RESERVATION_ALREADY_RELEASED` | 409 | Reservation already released |
| `WAREHOUSE_NOT_FOUND` | 404 | Warehouse code not found |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

### Domain Error Types

```go
// internal/order/domain/errors.go
var (
    ErrOrderNotFound          = errors.New("order not found")
    ErrOrderAlreadyExists     = errors.New("order already exists")
    ErrOrderCannotCancel      = errors.New("order cannot be cancelled")
    ErrOrderCannotConfirm     = errors.New("order cannot be confirmed")
    ErrInvalidRequest         = errors.New("invalid request")
)

// internal/inventory/domain/errors.go
var (
    ErrInsufficientStock      = errors.New("insufficient stock")
    ErrReservationNotFound    = errors.New("reservation not found")
    ErrReservationAlreadyReleased = errors.New("reservation already released")
    ErrInventoryNotFound      = errors.New("inventory not found")
    ErrWarehouseNotFound      = errors.New("warehouse not found")
)
```

---

## Error Propagation

### Layer-by-Layer

```
Domain errors (sentinels)
    ↓
Usecase (wraps with context)
    ↓
Handler (maps to HTTP status + JSON)
```

### Handler Mapping

```go
// internal/order/adapter/httpd/handler.go
func (h *OrderHandler) handleError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, domain.ErrOrderNotFound):
        response.NotFound(w, "order not found")
    case errors.Is(err, domain.ErrOrderAlreadyExists):
        response.Conflict(w, "duplicate idempotency key")
    case errors.Is(err, domain.ErrOrderCannotCancel):
        response.Conflict(w, "order cannot be cancelled")
    case errors.Is(err, domain.ErrInvalidRequest):
        response.BadRequest(w, "invalid request")
    default:
        response.InternalError(w, "internal server error")
    }
}
```

---

## Kafka Error Handling

### Producer Errors

| Scenario | Behavior |
|----------|----------|
| Publish fails | Retry with exponential backoff |
| Retry exhausted | Mark outbox event as FAILED |
| DLQ publish | Send to `<topic>.dlq` with error metadata headers |

### Consumer Errors

| Scenario | Behavior |
|----------|----------|
| Processing fails | Retry up to `MaxRetries` (default: 3) |
| Retry exhausted | Publish to DLQ topic |
| Context cancelled | Stop processing, commit offset |
| Bad JSON payload | Log error, skip message |

### DLQ Message Headers

Failed messages include error metadata:

| Header | Value |
|--------|-------|
| `error-message` | Original error description |
| `retry-count` | Number of retries attempted |
| `original-topic` | Source topic name |
| `original-partition` | Source partition |
| `original-offset` | Source offset |
| `timestamp` | ISO 8601 timestamp |

### DLQ Topic Naming

`<original-topic>.dlq`

Example: `order.created.dlq`, `inventory.reserved.dlq`

---

## HTTP Error Responses

### 400 Bad Request

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "invalid request body"
  }
}
```

### 404 Not Found

```json
{
  "error": {
    "code": "ORDER_NOT_FOUND",
    "message": "order not found"
  }
}
```

### 409 Conflict

```json
{
  "error": {
    "code": "ORDER_ALREADY_EXISTS",
    "message": "duplicate idempotency key",
    "details": "existing order 01HXYZ... returned"
  }
}
```

### 500 Internal Server Error

```json
{
  "error": {
    "code": "INTERNAL_ERROR",
    "message": "internal server error"
  }
}
```

---

## Logging

All errors are logged at ERROR level with structured fields:

```go
logger.Error("failed to reserve stock",
    zap.String("order_id", orderID),
    zap.String("product_id", productID),
    zap.Error(err),
)
```

### Log Levels

| Level | Usage |
|-------|-------|
| ERROR | Unexpected failures, DLQ publishes |
| WARN | Cache misses, retryable errors |
| INFO | Request processing, event publishing |
| DEBUG | SQL queries, cache operations |

Logs ship to Loki via Promtail.

---

## What's Implemented

- [x] Domain error sentinels (order + inventory)
- [x] Handler error mapping (HTTP status codes)
- [x] Structured JSON logging (Zap)
- [x] DLQ with error metadata headers
- [x] Consumer retry with configurable max retries
- [x] Kafka producer retry with backoff
- [x] Outbox event FAILED status on exhaustion
- [x] Log shipping to Loki via Promtail
