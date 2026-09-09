# Error Handling

## Overview

| Principle | Description |
|-----------|-------------|
| Fail Fast | Detect and report errors immediately |
| Fail Open | Degrade gracefully when possible |
| Idempotent | Safe to retry failed operations |
| Observable | Log, trace, and metrics for all errors |

---

## Error Categories

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              ERROR CATEGORIES                                    │
│                                                                                 │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐               │
│  │  Client Errors  │  │  Server Errors  │  │  System Errors  │               │
│  │  (4xx)          │  │  (5xx)          │  │  (Internal)     │               │
│  ├─────────────────┤  ├─────────────────┤  ├─────────────────┤               │
│  │ - Validation    │  │ - Database      │  │ - Out of memory │               │
│  │ - Authentication│  │ - External API  │  │ - Disk full     │               │
│  │ - Authorization │  │ - Kafka         │  │ - Network       │               │
│  │ - Not Found     │  │ - Redis         │  │ - CPU overload  │               │
│  │ - Conflict      │  │ - Timeout       │  │                 │               │
│  │ - Rate Limit    │  │ - Circuit Open  │  │                 │               │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘               │
│                                                                                 │
│  Recoverable         Transient/Fatal       Fatal (escalate)                   │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Error Types

### Application Errors

```go
// AppError represents application-level errors
type AppError struct {
    Code       string                 `json:"code"`
    Message    string                 `json:"message"`
    Details    map[string]interface{} `json:"details,omitempty"`
    HTTPStatus int                    `json:"-"`
    Err        error                  `json:"-"`
}

func (e *AppError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
    return e.Err
}
```

### Error Definitions

```go
var (
    // Validation Errors (400)
    ErrInvalidRequest = func(details map[string]interface{}) *AppError {
        return &AppError{
            Code:       "INVALID_REQUEST",
            Message:    "The request is invalid",
            Details:    details,
            HTTPStatus: http.StatusBadRequest,
        }
    }

    ErrInvalidQuantity = &AppError{
        Code:       "INVALID_QUANTITY",
        Message:    "Quantity must be greater than 0",
        HTTPStatus: http.StatusBadRequest,
    }

    ErrEmptyCart = &AppError{
        Code:       "EMPTY_CART",
        Message:    "Order must contain at least one item",
        HTTPStatus: http.StatusBadRequest,
    }

    // Authentication Errors (401)
    ErrUnauthorized = &AppError{
        Code:       "UNAUTHORIZED",
        Message:    "Authentication required",
        HTTPStatus: http.StatusUnauthorized,
    }

    ErrInvalidToken = &AppError{
        Code:       "INVALID_TOKEN",
        Message:    "The provided token is invalid or expired",
        HTTPStatus: http.StatusUnauthorized,
    }

    // Authorization Errors (403)
    ErrForbidden = &AppError{
        Code:       "FORBIDDEN",
        Message:    "Insufficient permissions",
        HTTPStatus: http.StatusForbidden,
    }

    // Not Found Errors (404)
    ErrNotFound = func(resource string) *AppError {
        return &AppError{
            Code:       "NOT_FOUND",
            Message:    fmt.Sprintf("%s not found", resource),
            HTTPStatus: http.StatusNotFound,
        }
    }

    ErrOrderNotFound = ErrNotFound("Order")
    ErrProductNotFound = ErrNotFound("Product")
    ErrInventoryNotFound = ErrNotFound("Inventory")

    // Conflict Errors (409)
    ErrConflict = func(resource string) *AppError {
        return &AppError{
            Code:       "CONFLICT",
            Message:    fmt.Sprintf("%s conflict", resource),
            HTTPStatus: http.StatusConflict,
        }
    }

    ErrInsufficientStock = &AppError{
        Code:       "INSUFFICIENT_STOCK",
        Message:    "Not enough stock available",
        HTTPStatus: http.StatusConflict,
    }

    ErrOrderNotCancellable = &AppError{
        Code:       "ORDER_NOT_CANCELLABLE",
        Message:    "Order cannot be cancelled in current status",
        HTTPStatus: http.StatusConflict,
    }

    ErrConcurrentModification = &AppError{
        Code:       "CONCURRENT_MODIFICATION",
        Message:    "Resource was modified by another request",
        HTTPStatus: http.StatusConflict,
    }

    // Rate Limit Errors (429)
    ErrRateLimited = &AppError{
        Code:       "RATE_LIMITED",
        Message:    "Too many requests",
        HTTPStatus: http.StatusTooManyRequests,
    }

    // Server Errors (500)
    ErrInternalError = &AppError{
        Code:       "INTERNAL_ERROR",
        Message:    "An internal error occurred",
        HTTPStatus: http.StatusInternalServerError,
    }

    ErrDatabaseError = &AppError{
        Code:       "DATABASE_ERROR",
        Message:    "Database operation failed",
        HTTPStatus: http.StatusInternalServerError,
    }

    ErrCacheError = &AppError{
        Code:       "CACHE_ERROR",
        Message:    "Cache operation failed",
        HTTPStatus: http.StatusInternalServerError,
    }

    ErrKafkaError = &AppError{
        Code:       "KAFKA_ERROR",
        Message:    "Message broker operation failed",
        HTTPStatus: http.StatusInternalServerError,
    }

    // Service Unavailable (503)
    ErrServiceUnavailable = func(service string) *AppError {
        return &AppError{
            Code:       "SERVICE_UNAVAILABLE",
            Message:    fmt.Sprintf("%s service is unavailable", service),
            HTTPStatus: http.StatusServiceUnavailable,
        }
    }
)
```

---

## Error Response Format

### Standard Error Response

```json
{
  "success": false,
  "error": {
    "code": "INSUFFICIENT_STOCK",
    "message": "Not enough stock available",
    "details": {
      "product_id": "prod_001",
      "requested": 10,
      "available": 5
    }
  },
  "request_id": "req_abc123xyz",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Validation Error Response

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "details": {
      "fields": {
        "email": "Invalid email format",
        "quantity": "Must be greater than 0"
      }
    }
  },
  "request_id": "req_def456abc",
  "timestamp": "2024-01-15T10:31:00Z"
}
```

### Partial Success Response

```json
{
  "success": false,
  "error": {
    "code": "PARTIAL_FAILURE",
    "message": "Some operations failed",
    "details": {
      "successful": [
        {"product_id": "prod_001", "quantity": 2}
      ],
      "failed": [
        {"product_id": "prod_002", "reason": "INSUFFICIENT_STOCK"}
      ]
    }
  },
  "request_id": "req_ghi789jkl",
  "timestamp": "2024-01-15T10:32:00Z"
}
```

---

## Error Propagation

### Internal Error Flow

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           ERROR PROPAGATION                                      │
│                                                                                 │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────┐  │
│  │   Handler    │◀────│   Service    │◀────│  Repository  │◀────│Database  │  │
│  └──────┬───────┘     └──────┬───────┘     └──────┬───────┘     └──────────┘  │
│         │                     │                     │                           │
│         │ Transform to        │ Wrap with           │ Raw error                │
│         │ AppError            │ context             │                          │
│         │◀────────────────────│◀────────────────────│                          │
│         │                     │                     │                           │
│         ▼                     ▼                     ▼                           │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐                   │
│  │ HTTP Response│     │ Logged with  │     │ Sentinel     │                   │
│  │              │     │ context      │     │ errors       │                   │
│  └──────────────┘     └──────────────┘     └──────────────┘                   │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Error Wrapping

```go
// Repository layer
func (r *InventoryRepo) GetInventory(ctx context.Context, productID, warehouseID string) (*Inventory, error) {
    var inventory Inventory
    err := r.db.GetContext(ctx, &inventory, `
        SELECT * FROM inventory
        WHERE product_id = $1 AND warehouse_id = $2`,
        productID, warehouseID,
    )

    if err == sql.ErrNoRows {
        return nil, ErrInventoryNotFound
    }

    if err != nil {
        return nil, fmt.Errorf("failed to get inventory: %w", err)
    }

    return &inventory, nil
}

// Service layer
func (s *InventoryService) GetInventory(ctx context.Context, productID, warehouseID string) (*Inventory, error) {
    inventory, err := s.repo.GetInventory(ctx, productID, warehouseID)
    if err != nil {
        if errors.Is(err, ErrInventoryNotFound) {
            return nil, err
        }
        return nil, fmt.Errorf("inventory service error: %w", err)
    }
    return inventory, nil
}

// Handler layer
func (h *InventoryHandler) GetInventory(w http.ResponseWriter, r *http.Request) {
    inventory, err := h.service.GetInventory(r.Context(), productID, warehouseID)
    if err != nil {
        respondError(w, r, err)
        return
    }
    respondJSON(w, http.StatusOK, inventory)
}
```

---

## Retry Strategies

### Exponential Backoff

```go
type RetryConfig struct {
    MaxRetries     int
    InitialBackoff time.Duration
    MaxBackoff     time.Duration
    Multiplier     float64
    RetryableErrors []string
}

func RetryWithBackoff(ctx context.Context, config RetryConfig, fn func() error) error {
    var lastErr error
    backoff := config.InitialBackoff

    for attempt := 0; attempt <= config.MaxRetries; attempt++ {
        err := fn()
        if err == nil {
            return nil
        }

        lastErr = err

        // Check if error is retryable
        if !isRetryable(err, config.RetryableErrors) {
            return err
        }

        // Check context cancellation
        if ctx.Err() != nil {
            return ctx.Err()
        }

        // Wait before retry
        select {
        case <-time.After(backoff):
        case <-ctx.Done():
            return ctx.Err()
        }

        // Increase backoff
        backoff = time.Duration(float64(backoff) * config.Multiplier)
        if backoff > config.MaxBackoff {
            backoff = config.MaxBackoff
        }
    }

    return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func isRetryable(err error, retryableErrors []string) bool {
    for _, code := range retryableErrors {
        if strings.Contains(err.Error(), code) {
            return true
        }
    }
    return false
}
```

### Usage Examples

```go
// Retry Kafka publish
err := RetryWithBackoff(ctx, RetryConfig{
    MaxRetries:     3,
    InitialBackoff: 100 * time.Millisecond,
    MaxBackoff:     5 * time.Second,
    Multiplier:     2.0,
    RetryableErrors: []string{"KAFKA_ERROR", "TIMEOUT"},
}, func() error {
    return s.publisher.Publish(ctx, topic, event)
})

// Retry inventory reservation
err := RetryWithBackoff(ctx, RetryConfig{
    MaxRetries:     3,
    InitialBackoff: 50 * time.Millisecond,
    MaxBackoff:     1 * time.Second,
    Multiplier:     2.0,
    RetryableErrors: []string{"CONCURRENT_MODIFICATION"},
}, func() error {
    return s.repo.ReserveStock(ctx, productID, warehouseID, quantity)
})
```

---

## Circuit Breaker

### Implementation

```go
type CircuitBreaker struct {
    mu              sync.Mutex
    state           CircuitState
    failureCount    int
    successCount    int
    failureThreshold int
    successThreshold int
    timeout         time.Duration
    lastFailureTime time.Time
}

type CircuitState int

const (
    CircuitClosed CircuitState = iota
    CircuitOpen
    CircuitHalfOpen
)

func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        state:            CircuitClosed,
        failureThreshold: failureThreshold,
        successThreshold: successThreshold,
        timeout:         timeout,
    }
}

func (cb *CircuitBreaker) Allow() bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    switch cb.state {
    case CircuitClosed:
        return true
    case CircuitOpen:
        if time.Since(cb.lastFailureTime) > cb.timeout {
            cb.state = CircuitHalfOpen
            return true
        }
        return false
    case CircuitHalfOpen:
        return true
    default:
        return false
    }
}

func (cb *CircuitBreaker) RecordSuccess() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    if cb.state == CircuitHalfOpen {
        cb.successCount++
        if cb.successCount >= cb.successThreshold {
            cb.state = CircuitClosed
            cb.failureCount = 0
            cb.successCount = 0
        }
    } else {
        cb.failureCount = 0
    }
}

func (cb *CircuitBreaker) RecordFailure() {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    cb.failureCount++
    cb.lastFailureTime = time.Now()

    if cb.state == CircuitHalfOpen {
        cb.state = CircuitOpen
        cb.successCount = 0
    } else if cb.failureCount >= cb.failureThreshold {
        cb.state = CircuitOpen
    }
}

func (cb *CircuitBreaker) State() CircuitState {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    return cb.state
}
```

### Usage

```go
type CircuitBreakerClient struct {
    httpClient *http.Client
    breaker    *CircuitBreaker
}

func (c *CircuitBreakerClient) Call(ctx context.Context, url string) ([]byte, error) {
    if !c.breaker.Allow() {
        return nil, ErrCircuitOpen
    }

    resp, err := c.httpClient.Get(url)
    if err != nil {
        c.breaker.RecordFailure()
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 500 {
        c.breaker.RecordFailure()
        return nil, ErrServerError
    }

    c.breaker.RecordSuccess()
    return io.ReadAll(resp.Body)
}
```

---

## Fallback Mechanisms

### Cache Fallback

```go
func (s *InventoryService) GetInventory(ctx context.Context, productID, warehouseID string) (*Inventory, error) {
    // Try primary source (database)
    inventory, err := s.repo.GetInventory(ctx, productID, warehouseID)
    if err != nil {
        // Fallback to cache
        s.logger.Warn("database unavailable, falling back to cache", zap.Error(err))

        var cached Inventory
        if err := s.cache.Get(ctx, CacheKeyBuilder{}.InventoryKey(productID, warehouseID), &cached); err != nil {
            return nil, ErrServiceUnavailable("inventory")
        }
        return &cached, nil
    }

    return inventory, nil
}
```

### Default Value Fallback

```go
func (s *OrderService) CalculateTax(ctx context.Context, amount float64, state string) (float64, error) {
    tax, err := s.taxService.GetTaxRate(ctx, state)
    if err != nil {
        // Fallback to default tax rate
        s.logger.Warn("tax service unavailable, using default rate", zap.Error(err))
        tax = 0.08 // 8% default
    }

    return amount * tax, nil
}
```

### Graceful Degradation

```go
func (s *NotificationService) SendOrderConfirmation(ctx context.Context, order Order) error {
    // Primary: Send email
    err := s.emailService.Send(ctx, order.CustomerEmail, "order_confirmation", order)
    if err != nil {
        s.logger.Warn("email service unavailable", zap.Error(err))

        // Fallback: Send SMS
        err = s.smsService.Send(ctx, order.CustomerPhone, "Order confirmed")
        if err != nil {
            s.logger.Warn("SMS service also unavailable", zap.Error(err))

            // Final fallback: Queue for later
            return s.queueService.Enqueue(ctx, "notifications", order)
        }
    }

    return nil
}
```

---

## Error Logging

### Structured Logging

```go
type ErrorLogger struct {
    logger *zap.Logger
}

func (l *ErrorLogger) LogError(ctx context.Context, err error, fields ...zap.Field) {
    // Extract request ID from context
    requestID := ctx.Value("request_id")

    baseFields := []zap.Field{
        zap.String("request_id", requestID.(string)),
        zap.Error(err),
        zap.Time("timestamp", time.Now()),
    }

    baseFields = append(baseFields, fields...)

    // Log based on error type
    var appErr *AppError
    if errors.As(err, &appErr) {
        if appErr.HTTPStatus >= 500 {
            l.logger.Error(appErr.Message, baseFields...)
        } else {
            l.logger.Warn(appErr.Message, baseFields...)
        }
    } else {
        l.logger.Error("unexpected error", baseFields...)
    }
}
```

### Error Context

```go
func (s *InventoryService) ReserveStock(ctx context.Context, req ReserveStockRequest) error {
    // Add context to error
    err := s.repo.ReserveStock(ctx, req.ProductID, req.WarehouseID, req.Quantity)
    if err != nil {
        return fmt.Errorf("failed to reserve stock for product %s in warehouse %s: %w",
            req.ProductID, req.WarehouseID, err)
    }
    return nil
}
```

---

## Error Recovery

### Panic Recovery

```go
func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                // Log panic
                logger.Error("panic recovered",
                    zap.String("method", r.Method),
                    zap.String("path", r.URL.Path),
                    zap.Any("error", err),
                    zap.String("stack", string(debug.Stack())),
                )

                // Respond with error
                respondJSON(w, http.StatusInternalServerError, ErrorResponse{
                    Success: false,
                    Error: AppError{
                        Code:    "INTERNAL_ERROR",
                        Message: "An internal error occurred",
                    },
                })
            }
        }()

        next.ServeHTTP(w, r)
    })
}
```

### Transaction Rollback

```go
func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()

    // Create order
    order, err := s.repo.CreateOrderTx(ctx, tx, req)
    if err != nil {
        return nil, err
    }

    // Reserve inventory
    err = s.inventoryService.ReserveStockTx(ctx, tx, order.ID, req.Items)
    if err != nil {
        return nil, err
    }

    // Publish event
    err = s.publisher.Publish(ctx, "order.created", order)
    if err != nil {
        return nil, err
    }

    // Commit transaction
    if err := tx.Commit(); err != nil {
        return nil, err
    }

    return order, nil
}
```

---

## Dead Letter Queue

### DLQ Processing

```go
type DeadLetterHandler struct {
    kafka    KafkaClient
    logger   *zap.Logger
    alerts   AlertService
}

func (h *DeadLetterHandler) HandleDeadLetter(ctx context.Context, msg kafka.Message) {
    // Parse dead letter event
    var event DeadLetterEvent
    if err := json.Unmarshal(msg.Value, &event); err != nil {
        h.logger.Error("failed to parse dead letter event", zap.Error(err))
        return
    }

    // Log dead letter
    h.logger.Error("dead letter received",
        zap.String("original_topic", event.OriginalTopic),
        zap.String("event_type", event.OriginalEvent.EventType),
        zap.Int("retry_count", event.RetryCount),
        zap.Error(event.Error),
    )

    // Send alert for manual intervention
    h.alerts.Send(ctx, Alert{
        Severity: "high",
        Title:    "Dead Letter Event",
        Message:  fmt.Sprintf("Event %s failed after %d retries", event.OriginalEvent.EventID, event.RetryCount),
    })

    // Store for later processing
    h.storeDeadLetter(ctx, event)
}
```

---

## Error Monitoring

### Prometheus Metrics

```go
var (
    errorCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "app_errors_total",
            Help: "Total application errors",
        },
        []string{"service", "code", "http_status"},
    )

    errorDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "app_error_duration_seconds",
            Help:    "Duration of error recovery attempts",
            Buckets: []float64{0.1, 0.5, 1, 5, 10},
        },
        []string{"service", "operation"},
    )

    circuitBreakerState = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "circuit_breaker_state",
            Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
        },
        []string{"service", "circuit"},
    )

    retryCount = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "retry_attempts_total",
            Help: "Total retry attempts",
        },
        []string{"service", "operation"},
    )
)
```

### Grafana Dashboard Queries

```promql
# Error rate
sum(rate(app_errors_total[5m])) by (service, code)

# Error rate by HTTP status
sum(rate(app_errors_total{http_status=~"5.."}[5m])) by (service)

# Circuit breaker state
circuit_breaker_state

# Retry rate
sum(rate(retry_attempts_total[5m])) by (service, operation)

# P99 error duration
histogram_quantile(0.99, sum(rate(app_error_duration_seconds_bucket[5m])) by (le, service))
```

---

## Error Handling Checklist

| Category | Item | Status |
|----------|------|--------|
| **Validation** | Input validation on all endpoints | ☐ |
| **Validation** | Request ID tracking | ☐ |
| **Authentication** | JWT validation errors | ☐ |
| **Authorization** | Permission denied errors | ☐ |
| **Database** | Connection failures | ☐ |
| **Database** | Query timeouts | ☐ |
| **Cache** | Cache miss handling | ☐ |
| **Cache** | Cache unavailability fallback | ☐ |
| **Kafka** | Publish failures | ☐ |
| **Kafka** | Consumer errors | ☐ |
| **External** | API timeout handling | ☐ |
| **External** | Circuit breaker | ☐ |
| **System** | Panic recovery | ☐ |
| **System** | Graceful shutdown | ☐ |
| **Monitoring** | Error logging | ☐ |
| **Monitoring** | Error metrics | ☐ |
| **Monitoring** | Alerting | ☐ |
