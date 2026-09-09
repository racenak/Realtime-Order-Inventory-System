# Concurrency Control

## Overview

| Concern | Solution |
|---------|----------|
| Overselling prevention | Optimistic locking + Atomic SQL |
| Concurrent reservations | Database transactions + Row-level locks |
| Distributed coordination | Redis distributed locks |
| Event processing | Idempotent consumers + Deduplication |
| Race conditions | Compare-and-swap operations |

---

## Concurrency Challenges

### The Overselling Problem

```
Timeline:
───────────────────────────────────────────────────────────────────────────

T1: User A reads stock = 10
T2: User B reads stock = 10
T3: User A orders 8, writes stock = 2
T4: User B orders 8, writes stock = 2  ← ERROR: Should fail!

Result: Stock oversold (ordered 16 from 10 available)
```

### Race Condition Scenarios

| Scenario | Risk | Impact |
|----------|------|--------|
| Two users order same product simultaneously | Overselling | Negative inventory |
| Concurrent stock updates | Lost updates | Incorrect counts |
| Order + cancellation at same time | Inconsistent state | Money/stock mismatch |
| Reservation expiry during order | Zombie reservations | Stock leakage |

---

## Locking Strategies

### 1. Pessimistic Locking

Locks row before update, blocks other transactions.

```sql
-- Lock inventory row for update
BEGIN;

SELECT quantity_on_hand, quantity_reserved
FROM inventory
WHERE product_id = 'prod_001' AND warehouse_id = 'wh_nyc'
FOR UPDATE;  -- Blocks other transactions

-- Now safe to update
UPDATE inventory
SET quantity_reserved = quantity_reserved + 2
WHERE product_id = 'prod_001' AND warehouse_id = 'wh_nyc';

COMMIT;
```

**When to Use:**
- High contention scenarios
- Critical stock operations
- When retry cost is high

**Drawbacks:**
- Reduces concurrency
- Potential deadlocks
- Holding locks = reduced throughput

---

### 2. Optimistic Locking

Detects conflicts at update time, requires retry.

```sql
-- Update with version check
UPDATE inventory
SET quantity_reserved = quantity_reserved + 2,
    version = version + 1,
    updated_at = NOW()
WHERE product_id = 'prod_001'
  AND warehouse_id = 'wh_nyc'
  AND version = 5  -- Expected version
  AND (quantity_on_hand - quantity_reserved) >= 2;  -- Sufficient stock

-- Check rows affected
-- 0 rows = conflict, retry
-- 1 row = success
```

**When to Use:**
- Low contention scenarios
- Read-heavy workloads
- When retries are acceptable

---

### 3. Compare-and-Swap (CAS)

Atomic operation combining read and conditional write.

```go
// CAS implementation
func (r *InventoryRepo) ReserveStock(
    ctx context.Context,
    productID, warehouseID string,
    quantity int,
    expectedAvailable int,
) error {
    result, err := r.db.ExecContext(ctx, `
        UPDATE inventory
        SET quantity_reserved = quantity_reserved + $1,
            version = version + 1,
            updated_at = NOW()
        WHERE product_id = $2
          AND warehouse_id = $3
          AND (quantity_on_hand - quantity_reserved) = $4
          AND (quantity_on_hand - quantity_reserved) >= $1`,
        quantity, productID, warehouseID, expectedAvailable,
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

---

## Atomic SQL Operations

### Reserve Stock (Atomic)

Single SQL statement prevents race conditions:

```sql
UPDATE inventory
SET quantity_reserved = quantity_reserved + $1,
    version = version + 1,
    updated_at = NOW()
WHERE product_id = $2
  AND warehouse_id = $3
  AND (quantity_on_hand - quantity_reserved) >= $1;  -- Atomic check
```

### Release Reservation (Atomic)

```sql
UPDATE inventory
SET quantity_reserved = quantity_reserved - $1,
    version = version + 1,
    updated_at = NOW()
WHERE product_id = $2
  AND warehouse_id = $3
  AND quantity_reserved >= $1;  -- Prevent negative
```

### Confirm Order (Atomic Deduct)

```sql
UPDATE inventory
SET quantity_on_hand = quantity_on_hand - $1,
    quantity_reserved = quantity_reserved - $1,
    version = version + 1,
    updated_at = NOW()
WHERE product_id = $2
  AND warehouse_id = $3
  AND quantity_reserved >= $1;
```

---

## Transaction Patterns

### Pattern 1: Reserve with Retry

```go
type ConcurrencyConfig struct {
    MaxRetries     int
    RetryDelay     time.Duration
    BackoffFactor  float64
}

func (s *InventoryService) ReserveStock(
    ctx context.Context,
    orderID string,
    items []OrderItem,
) ([]Reservation, error) {
    config := ConcurrencyConfig{
        MaxRetries:    3,
        RetryDelay:    100 * time.Millisecond,
        BackoffFactor: 2.0,
    }

    var reservations []Reservation

    for _, item := range items {
        var reservation *Reservation
        var err error

        for attempt := 0; attempt <= config.MaxRetries; attempt++ {
            reservation, err = s.tryReserve(ctx, orderID, item)
            if err == nil {
                break
            }

            if err != ErrConcurrentModification {
                return nil, err
            }

            // Exponential backoff
            delay := config RetryDelay * time.Duration(math.Pow(config.BackoffFactor, float64(attempt)))
            time.Sleep(delay)
        }

        if err != nil {
            // Compensate: release already reserved items
            s.releaseReservations(ctx, reservations)
            return nil, fmt.Errorf("failed to reserve %s after %d attempts: %w",
                item.ProductID, config.MaxRetries, err)
        }

        reservations = append(reservations, *reservation)
    }

    return reservations, nil
}

func (s *InventoryService) tryReserve(
    ctx context.Context,
    orderID string,
    item OrderItem,
) (*Reservation, error) {
    // Get current available stock
    inventory, err := s.repo.GetInventory(ctx, item.ProductID, item.WarehouseID)
    if err != nil {
        return nil, err
    }

    available := inventory.QuantityOnHand - inventory.QuantityReserved
    if available < item.Quantity {
        return nil, ErrInsufficientStock
    }

    // Attempt atomic update
    err = s.repo.ReserveStock(ctx, item.ProductID, item.WarehouseID, item.Quantity, available)
    if err != nil {
        return nil, err
    }

    // Create reservation record
    reservation, err := s.repo.CreateReservation(ctx, CreateReservationRequest{
        OrderID:     orderID,
        ProductID:   item.ProductID,
        WarehouseID: item.WarehouseID,
        Quantity:    item.Quantity,
    })
    if err != nil {
        return nil, err
    }

    return reservation, nil
}
```

---

### Pattern 2: Optimistic Lock with Version Check

```go
func (s *InventoryService) UpdateStock(
    ctx context.Context,
    productID, warehouseID string,
    newQuantity int,
) error {
    maxRetries := 3

    for attempt := 0; attempt < maxRetries; attempt++ {
        // Get current state
        inventory, err := s.repo.GetInventory(ctx, productID, warehouseID)
        if err != nil {
            return err
        }

        // Attempt update with version check
        err = s.repo.UpdateInventory(ctx, UpdateInventoryRequest{
            ProductID:      productID,
            WarehouseID:    warehouseID,
            QuantityOnHand: newQuantity,
            ExpectedVersion: inventory.Version,
        })

        if err == nil {
            return nil
        }

        if err != ErrVersionConflict {
            return err
        }

        // Log retry
        s.logger.Warn("optimistic lock conflict, retrying",
            zap.String("product_id", productID),
            zap.Int("attempt", attempt+1),
        )

        // Brief delay before retry
        time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
    }

    return ErrMaxRetriesExceeded
}
```

---

### Pattern 3: Select for Update Skip Locked

For queue-like processing without blocking:

```sql
-- Get next batch of reservations to process
SELECT *
FROM inventory_reservations
WHERE status = 'reserved'
  AND expires_at < NOW()
ORDER BY created_at
LIMIT 10
FOR UPDATE SKIP LOCKED;  -- Skip rows locked by other transactions
```

---

## Distributed Locking (Redis)

### Lock Implementation

```go
type DistributedLock struct {
    client     *redis.Client
    lockKey    string
    lockValue  string
    ttl        time.Duration
}

func NewDistributedLock(client *redis.Client, key string, ttl time.Duration) *DistributedLock {
    return &DistributedLock{
        client:    client,
        lockKey:   key,
        lockValue: uuid.New().String(),
        ttl:       ttl,
    }
}

// Acquire tries to acquire the lock
func (l *DistributedLock) Acquire(ctx context.Context) (bool, error) {
    result, err := l.client.SetNX(ctx, l.lockKey, l.lockValue, l.ttl).Result()
    if err != nil {
        return false, err
    }
    return result, nil
}

// Release releases the lock (only if we own it)
func (l *DistributedLock) Release(ctx context.Context) error {
    script := `
        if redis.call("get", KEYS[1]) == ARGV[1] then
            return redis.call("del", KEYS[1])
        else
            return 0
        end
    `
    _, err := l.client.Eval(ctx, script, []string{l.lockKey}, l.lockValue).Result()
    return err
}

// Extend extends the lock TTL
func (l *DistributedLock) Extend(ctx context.Context) error {
    result, err := l.client.Expire(ctx, l.lockKey, l.ttl).Result()
    if err != nil {
        return err
    }
    if !result {
        return ErrLockNotFound
    }
    return nil
}
```

### Usage: Reservation Expiry Processing

```go
func (s *InventoryService) ProcessExpiredReservations(ctx context.Context) error {
    lock := NewDistributedLock(s.redis, "lock:reservation:expiry", 30*time.Second)

    acquired, err := lock.Acquire(ctx)
    if err != nil {
        return err
    }
    if !acquired {
        s.logger.Info("another instance is processing expired reservations")
        return nil
    }
    defer lock.Release(ctx)

    // Extend lock periodically for long-running operations
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            if err := lock.Extend(ctx); err != nil {
                return
            }
        }
    }()

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
        }
    }

    return nil
}
```

---

## Deadlock Prevention

### Strategy 1: Consistent Lock Order

Always acquire locks in the same order:

```go
// CORRECT: Always lock by product_id then warehouse_id
func (s *InventoryService) TransferStock(
    ctx context.Context,
    productID string,
    fromWarehouse, toWarehouse string,
    quantity int,
) error {
    // Sort warehouse IDs
    warehouses := []string{fromWarehouse, toWarehouse}
    sort.Strings(warehouses)

    // Acquire locks in consistent order
    for _, wh := range warehouses {
        lock := NewDistributedLock(s.redis, fmt.Sprintf("lock:inventory:%s:%s", productID, wh), 5*time.Second)
        acquired, err := lock.Acquire(ctx)
        if err != nil || !acquired {
            return ErrCouldNotAcquireLock
        }
        defer lock.Release(ctx)
    }

    // Now safe to transfer
    // ...
}
```

### Strategy 2: Lock Timeout

```sql
-- Set lock timeout to prevent indefinite waiting
SET lock_timeout = '5s';

BEGIN;
SELECT * FROM inventory
WHERE product_id = 'prod_001'
FOR UPDATE;
-- If lock not acquired in 5s, returns error
```

### Strategy 3: Try-Lock Pattern

```go
func (s *InventoryService) TryLockAndUpdate(
    ctx context.Context,
    productID, warehouseID string,
    updateFunc func(ctx context.Context) error,
) error {
    lockKey := fmt.Sprintf("lock:inventory:%s:%s", productID, warehouseID)
    lock := NewDistributedLock(s.redis, lockKey, 5*time.Second)

    acquired, err := lock.Acquire(ctx)
    if err != nil {
        return err
    }

    if !acquired {
        return ErrLockBusy
    }
    defer lock.Release(ctx)

    return updateFunc(ctx)
}
```

---

## Concurrent Order Processing

### Order Queue with Partitioning

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    KAFKA PARTITIONING                                    │
│                                                                         │
│  Topic: order.created                                                  │
│  Partition Key: customer_id (or order_id)                              │
│                                                                         │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐        │
│  │  Partition 0    │  │  Partition 1    │  │  Partition 2    │        │
│  │                 │  │                 │  │                 │        │
│  │  Order A        │  │  Order B        │  │  Order C        │        │
│  │  Order D        │  │  Order E        │  │  Order F        │        │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘        │
│           │                     │                     │                 │
│           ▼                     ▼                     ▼                 │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐        │
│  │  Consumer 0     │  │  Consumer 1     │  │  Consumer 2     │        │
│  │  (Worker Pool)  │  │  (Worker Pool)  │  │  (Worker Pool)  │        │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘        │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Worker Pool for Concurrent Processing

```go
type WorkerPool struct {
    numWorkers int
    jobQueue   chan Job
    resultChan chan Result
}

func NewWorkerPool(numWorkers, queueSize int) *WorkerPool {
    return &WorkerPool{
        numWorkers: numWorkers,
        jobQueue:   make(chan Job, queueSize),
        resultChan: make(chan Result, queueSize),
    }
}

func (wp *WorkerPool) Start(ctx context.Context) {
    var wg sync.WaitGroup

    for i := 0; i < wp.numWorkers; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            wp.worker(ctx, workerID)
        }(i)
    }

    wg.Wait()
    close(wp.resultChan)
}

func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
    for job := range wp.jobQueue {
        select {
        case <-ctx.Done():
            return
        default:
            result := wp.processJob(ctx, job)
            wp.resultChan <- result
        }
    }
}
```

---

## Read-Write Separation

### Read Replica Pattern

```go
type DatabasePool struct {
    writeDB *sql.DB  // Primary
    readDB  *sql.DB  // Read replica
}

func (dp *DatabasePool) GetInventory(ctx context.Context, productID, warehouseID string) (*Inventory, error) {
    // Read from replica (eventual consistency OK)
    return dp.readDB.QueryContext(ctx, `
        SELECT * FROM inventory
        WHERE product_id = $1 AND warehouse_id = $2`,
        productID, warehouseID,
    )
}

func (dp *DatabasePool) UpdateInventory(ctx context.Context, req UpdateInventoryRequest) error {
    // Write to primary (strong consistency required)
    _, err := dp.writeDB.ExecContext(ctx, `
        UPDATE inventory
        SET quantity_on_hand = $1, version = version + 1
        WHERE product_id = $2 AND warehouse_id = $3 AND version = $4`,
        req.QuantityOnHand, req.ProductID, req.WarehouseID, req.ExpectedVersion,
    )
    return err
}
```

---

## Concurrency Metrics

### Key Metrics to Monitor

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `concurrent_lock_wait_ms` | Time waiting for locks | > 1000ms |
| `optimistic_lock_retry_count` | CAS retry attempts | > 10% of operations |
| `deadlock_count` | Deadlock occurrences | > 0 |
| `lock_timeout_count` | Lock acquisition timeouts | > 5/min |
| `concurrent_order_processing` | Orders processed in parallel | Monitor trend |

### Prometheus Metrics

```go
var (
    lockWaitDuration = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "inventory_lock_wait_duration_ms",
            Help:    "Time spent waiting for inventory locks",
            Buckets: []float64{10, 50, 100, 500, 1000, 5000},
        },
    )

    optimisticLockRetries = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "inventory_optimistic_lock_retries_total",
            Help: "Total optimistic lock retry attempts",
        },
    )

    deadlocks = prometheus.NewCounter(
        prometheus.CounterOpts{
            Name: "inventory_deadlocks_total",
            Help: "Total deadlock occurrences",
        },
    )
)
```

---

## Best Practices Summary

| Practice | Implementation |
|----------|----------------|
| Use atomic SQL | Single UPDATE with WHERE condition |
| Implement retry logic | Exponential backoff for CAS conflicts |
| Consistent lock ordering | Sort resources before acquiring locks |
| Set lock timeouts | Prevent indefinite blocking |
| Idempotent operations | Safe to retry any operation |
| Monitor lock contention | Track wait times and retries |
| Use distributed locks | Redis for cross-service coordination |
| Separate read/write | Replicas for reads, primary for writes |
| Avoid long transactions | Keep transactions short |
| Handle deadlocks gracefully | Detect and retry on deadlock |

---

## Comparison Table

| Strategy | Pros | Cons | Use Case |
|----------|------|------|----------|
| Pessimistic Lock | Guaranteed consistency | Reduced concurrency | High contention |
| Optimistic Lock | High concurrency | Retry overhead | Low contention |
| Atomic SQL | Simple, fast | Limited complexity | Single row updates |
| Distributed Lock | Cross-service coordination | Network overhead | Coordinated operations |
| Queue + Workers | Scalable processing | Eventual consistency | High throughput |
