# Caching Strategy

## Overview

| Property | Value |
|----------|-------|
| Primary Cache | Redis 7 (Cluster) |
| Cache Layer | Application (per-service) |
| Invalidation | Event-driven + TTL |
| Consistency | Eventual (with strong where needed) |

---

## Cache Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              CACHING ARCHITECTURE                                │
│                                                                                 │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐                   │
│  │    Client     │     │    Client     │     │    Client     │                   │
│  └──────┬───────┘     └──────┬───────┘     └──────┬───────┘                   │
│         │                     │                     │                           │
│         ▼                     ▼                     ▼                           │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                           TRAEFIK                                       │   │
│  └─────────────────────────────┬───────────────────────────────────────────┘   │
│                                │                                               │
│         ┌──────────────────────┼──────────────────────┐                       │
│         ▼                      ▼                      ▼                       │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐                   │
│  │  Order Svc   │     │Inventory Svc │     │ WebSocket Svc│                   │
│  └──────┬───────┘     └──────┬───────┘     └──────┬───────┘                   │
│         │                     │                     │                           │
│         └──────────────────────┼──────────────────────┘                       │
│                                │                                               │
│                                ▼                                               │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                           REDIS CLUSTER                                 │   │
│  │                                                                         │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                 │   │
│  │  │  L1 Cache    │  │  L2 Cache    │  │  Pub/Sub     │                 │   │
│  │  │  (Local)     │  │  (Redis)     │  │  (Events)    │                 │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘                 │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                │                                               │
│                                ▼                                               │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                         POSTGRESQL                                      │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Cache Layers

### Layer 1: Application Cache (In-Process)

Local in-memory cache for hot data.

```go
type LocalCache struct {
    items   sync.Map
    maxSize int
    ttl     time.Duration
}

type CacheItem struct {
    Value     interface{}
    ExpiresAt time.Time
}

func NewLocalCache(maxSize int, ttl time.Duration) *LocalCache {
    c := &LocalCache{
        maxSize: maxSize,
        ttl:     ttl,
    }
    go c.cleanup()
    return c
}

func (c *LocalCache) Get(key string) (interface{}, bool) {
    item, ok := c.items.Load(key)
    if !ok {
        return nil, false
    }

    cacheItem := item.(*CacheItem)
    if time.Now().After(cacheItem.ExpiresAt) {
        c.items.Delete(key)
        return nil, false
    }

    return cacheItem.Value, true
}

func (c *LocalCache) Set(key string, value interface{}) {
    if c.size() >= c.maxSize {
        c.evict()
    }

    c.items.Store(key, &CacheItem{
        Value:     value,
        ExpiresAt: time.Now().Add(c.ttl),
    })
}

func (c *LocalCache) Invalidate(key string) {
    c.items.Delete(key)
}
```

### Layer 2: Distributed Cache (Redis)

Shared cache across service instances.

```go
type RedisCache struct {
    client *redis.ClusterClient
    prefix string
    defaultTTL time.Duration
}

func NewRedisCache(client *redis.ClusterClient, prefix string, defaultTTL time.Duration) *RedisCache {
    return &RedisCache{
        client:     client,
        prefix:     prefix,
        defaultTTL: defaultTTL,
    }
}

func (r *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
    data, err := r.client.Get(ctx, r.key(key)).Bytes()
    if err == redis.Nil {
        return ErrCacheMiss
    }
    if err != nil {
        return err
    }
    return json.Unmarshal(data, dest)
}

func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl ...time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }

    expire := r.defaultTTL
    if len(ttl) > 0 {
        expire = ttl[0]
    }

    return r.client.Set(ctx, r.key(key), data, expire).Err()
}

func (r *RedisCache) Invalidate(ctx context.Context, keys ...string) error {
    pipe := r.client.Pipeline()
    for _, key := range keys {
        pipe.Del(ctx, r.key(key))
    }
    _, err := pipe.Exec(ctx)
    return err
}

func (r *RedisCache) key(key string) string {
    return fmt.Sprintf("%s:%s", r.prefix, key)
}
```

---

## Cache Key Design

### Naming Convention

```
{service}:{entity}:{identifier}
```

### Key Patterns

| Pattern | Example | TTL | Description |
|---------|---------|-----|-------------|
| `inventory:{product_id}` | `inventory:prod_001` | 30s | Product stock across warehouses |
| `inventory:{product_id}:{warehouse_id}` | `inventory:prod_001:wh_nyc` | 30s | Product stock in specific warehouse |
| `order:{order_id}` | `order:ord_abc123` | 60s | Order details |
| `user:{user_id}:orders` | `user:usr_xyz789:orders` | 30s | User's recent orders |
| `product:{product_id}` | `product:prod_001` | 5min | Product details |
| `warehouse:{warehouse_id}` | `warehouse:wh_nyc` | 5min | Warehouse details |
| `session:{session_id}` | `session:ses_abc123` | 24h | User session |
| `rate_limit:{identifier}` | `rate_limit:192.168.1.1` | 60s | Rate limiting |

### Key Structure Examples

```go
type CacheKeyBuilder struct{}

func (b CacheKeyBuilder) InventoryKey(productID, warehouseID string) string {
    if warehouseID == "" {
        return fmt.Sprintf("inventory:%s", productID)
    }
    return fmt.Sprintf("inventory:%s:%s", productID, warehouseID)
}

func (b CacheKeyBuilder) OrderKey(orderID string) string {
    return fmt.Sprintf("order:%s", orderID)
}

func (b CacheKeyBuilder) UserOrdersKey(userID string) string {
    return fmt.Sprintf("user:%s:orders", userID)
}

func (b CacheKeyBuilder) ProductKey(productID string) string {
    return fmt.Sprintf("product:%s", productID)
}

func (b CacheKeyBuilder) RateLimitKey(identifier string) string {
    return fmt.Sprintf("rate_limit:%s", identifier)
}
```

---

## TTL Policies

### By Data Type

| Data Type | TTL | Rationale |
|-----------|-----|-----------|
| Inventory stock | 30s | Real-time accuracy critical |
| Order details | 60s | Moderate freshness needed |
| User orders | 30s | Recent data |
| Product catalog | 5min | Rarely changes |
| Warehouse info | 5min | Static data |
| User session | 24h | Authentication |
| Rate limit | 60s | Sliding window |

### TTL Variation (Jitter)

Prevent cache stampede by adding random jitter:

```go
func (c *CacheService) SetWithJitter(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error {
    // Add 10% jitter
    jitter := time.Duration(rand.Int63n(int64(baseTTL / 10)))
    ttl := baseTTL + jitter

    return c.redis.Set(ctx, key, value, ttl).Err()
}
```

---

## Cache Patterns

### Pattern 1: Cache-Aside (Lazy Loading)

Application manages cache explicitly.

```go
type InventoryCache struct {
    redis  *RedisCache
    repo   InventoryRepository
}

func (c *InventoryCache) GetInventory(ctx context.Context, productID, warehouseID string) (*Inventory, error) {
    key := CacheKeyBuilder{}.InventoryKey(productID, warehouseID)

    // Try cache first
    var inventory Inventory
    err := c.redis.Get(ctx, key, &inventory)
    if err == nil {
        return &inventory, nil
    }

    // Cache miss - fetch from database
    inventory, err = c.repo.GetInventory(ctx, productID, warehouseID)
    if err != nil {
        return nil, err
    }

    // Populate cache
    c.redis.Set(ctx, key, inventory, 30*time.Second)

    return &inventory, nil
}
```

### Pattern 2: Write-Through

Write to cache and database simultaneously.

```go
func (c *InventoryCache) UpdateStock(ctx context.Context, req UpdateStockRequest) error {
    // Update database
    inventory, err := c.repo.UpdateStock(ctx, req)
    if err != nil {
        return err
    }

    // Update cache immediately
    key := CacheKeyBuilder{}.InventoryKey(req.ProductID, req.WarehouseID)
    c.redis.Set(ctx, key, inventory, 30*time.Second)

    return nil
}
```

### Pattern 3: Write-Behind (Write-Back)

Write to cache first, async to database.

```go
type WriteBehindCache struct {
    redis    *RedisCache
    repo     InventoryRepository
    writeQueue chan WriteOperation
}

func (c *WriteBehindCache) UpdateStock(ctx context.Context, req UpdateStockRequest) error {
    // Update cache immediately
    key := CacheKeyBuilder{}.InventoryKey(req.ProductID, req.WarehouseID)
    current, _ := c.GetFromCache(ctx, key)

    updated := applyUpdate(current, req)
    c.redis.Set(ctx, key, updated, 30*time.Second)

    // Queue database write
    c.writeQueue <- WriteOperation{
        Type:      "update_stock",
        Request:   req,
        Timestamp: time.Now(),
    }

    return nil
}

func (c *WriteBehindCache) processWriteQueue() {
    for op := range c.writeQueue {
        // Batch writes for efficiency
        c.repo.BulkUpdate([]UpdateStockRequest{op.Request})
    }
}
```

### Pattern 4: Read-Through

Cache layer handles data fetching.

```go
type ReadThroughCache struct {
    redis     *RedisCache
    loader    DataLoader
}

func (c *ReadThroughCache) Get(ctx context.Context, key string) (interface{}, error) {
    var value interface{}
    err := c.redis.Get(ctx, key, &value)
    if err == nil {
        return value, nil
    }

    // Cache miss - loader fetches from source
    value, err = c.loader.Load(ctx, key)
    if err != nil {
        return nil, err
    }

    c.redis.Set(ctx, key, value, 30*time.Second)
    return value, nil
}
```

---

## Cache Invalidation

### Strategy 1: Event-Driven Invalidation

Kafka events trigger cache invalidation.

```go
type CacheInvalidationConsumer struct {
    redis  *RedisCache
    topics []string
}

func (c *CacheInvalidationConsumer) HandleEvent(ctx context.Context, event Event) error {
    switch event.Type {
    case "inventory.updated":
        var data InventoryUpdatedEvent
        json.Unmarshal(event.Payload, &data)

        // Invalidate inventory cache
        keys := []string{
            CacheKeyBuilder{}.InventoryKey(data.ProductID, ""),
            CacheKeyBuilder{}.InventoryKey(data.ProductID, data.WarehouseID),
        }
        c.redis.Invalidate(ctx, keys...)

    case "order.status_changed":
        var data OrderStatusChangedEvent
        json.Unmarshal(event.Payload, &data)

        // Invalidate order cache
        c.redis.Invalidate(ctx, CacheKeyBuilder{}.OrderKey(data.OrderID))

    case "product.updated":
        var data ProductUpdatedEvent
        json.Unmarshal(event.Payload, &data)

        // Invalidate product cache
        c.redis.Invalidate(ctx, CacheKeyBuilder{}.ProductKey(data.ProductID))
    }

    return nil
}
```

### Strategy 2: Time-Based Expiration (TTL)

Automatic expiration based on TTL.

```go
// Cache entries automatically expire
c.redis.Set(ctx, key, value, 30*time.Second)
```

### Strategy 3: Manual Invalidation

Explicit invalidation on data change.

```go
func (s *InventoryService) UpdateStock(ctx context.Context, req UpdateStockRequest) error {
    // Update database
    if err := s.repo.UpdateStock(ctx, req); err != nil {
        return err
    }

    // Invalidate cache
    keys := []string{
        CacheKeyBuilder{}.InventoryKey(req.ProductID, ""),
        CacheKeyBuilder{}.InventoryKey(req.ProductID, req.WarehouseID),
    }
    s.cache.Invalidate(ctx, keys...)

    // Publish event for other services
    s.publisher.Publish(ctx, "inventory.updated", InventoryUpdatedEvent{
        ProductID:   req.ProductID,
        WarehouseID: req.WarehouseID,
    })

    return nil
}
```

---

## Cache Warming

### Pre-load Critical Data

```go
func (s *InventoryService) WarmCache(ctx context.Context) error {
    // Load low-stock products (most frequently accessed)
    products, err := s.repo.GetLowStockProducts(ctx)
    if err != nil {
        return err
    }

    for _, product := range products {
        inventory, err := s.repo.GetInventory(ctx, product.ID, "")
        if err != nil {
            continue
        }

        key := CacheKeyBuilder{}.InventoryKey(product.ID, "")
        s.cache.Set(ctx, key, inventory, 30*time.Second)
    }

    s.logger.Info("cache warmed", zap.Int("products", len(products)))
    return nil
}
```

### Scheduled Warming

```go
func (s *InventoryService) StartCacheWarming(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := s.WarmCache(ctx); err != nil {
                s.logger.Error("cache warming failed", zap.Error(err))
            }
        }
    }
}
```

---

## Cache Stampede Prevention

### Problem

Multiple requests try to rebuild expired cache simultaneously.

```
Request 1 ──▶ Cache Miss ──▶ DB Query ──▶ Set Cache
Request 2 ──▶ Cache Miss ──▶ DB Query ──▶ Set Cache
Request 3 ──▶ Cache Miss ──▶ DB Query ──▶ Set Cache

All 3 hit DB simultaneously!
```

### Solution 1: Singleflight

Ensures only one goroutine fetches data.

```go
import "golang.org/x/sync/singleflight"

type InventoryService struct {
    cache  *RedisCache
    repo   InventoryRepository
    group  singleflight.Group
}

func (s *InventoryService) GetInventory(ctx context.Context, productID, warehouseID string) (*Inventory, error) {
    key := CacheKeyBuilder{}.InventoryKey(productID, warehouseID)

    // Try cache
    var inventory Inventory
    err := s.cache.Get(ctx, key, &inventory)
    if err == nil {
        return &inventory, nil
    }

    // Use singleflight to prevent stampede
    result, err, _ := s.group.Do(key, func() (interface{}, error) {
        inv, err := s.repo.GetInventory(ctx, productID, warehouseID)
        if err != nil {
            return nil, err
        }

        s.cache.Set(ctx, key, inv, 30*time.Second)
        return inv, nil
    })

    if err != nil {
        return nil, err
    }

    return result.(*Inventory), nil
}
```

### Solution 2: Mutex Lock

Distributed lock for cache rebuild.

```go
func (s *InventoryService) GetInventoryWithLock(ctx context.Context, productID, warehouseID string) (*Inventory, error) {
    key := CacheKeyBuilder{}.InventoryKey(productID, warehouseID)

    // Try cache
    var inventory Inventory
    err := s.cache.Get(ctx, key, &inventory)
    if err == nil {
        return &inventory, nil
    }

    // Acquire lock
    lockKey := fmt.Sprintf("lock:cache:%s", key)
    lock := NewDistributedLock(s.redis, lockKey, 5*time.Second)
    acquired, _ := lock.Acquire(ctx)

    if acquired {
        defer lock.Release(ctx)

        // Double-check cache (might have been set by another process)
        err = s.cache.Get(ctx, key, &inventory)
        if err == nil {
            return &inventory, nil
        }

        // Fetch and cache
        inventory, err = s.repo.GetInventory(ctx, productID, warehouseID)
        if err != nil {
            return nil, err
        }

        s.cache.Set(ctx, key, inventory, 30*time.Second)
        return &inventory, nil
    }

    // Another process is rebuilding, wait and retry
    time.Sleep(100 * time.Millisecond)
    return s.GetInventoryWithLock(ctx, productID, warehouseID)
}
```

---

## Cache Monitoring

### Metrics

```go
var (
    cacheHits = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_hits_total",
            Help: "Total cache hits",
        },
        []string{"service", "cache_type"},
    )

    cacheMisses = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_misses_total",
            Help: "Total cache misses",
        },
        []string{"service", "cache_type"},
    )

    cacheOperationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "cache_operation_duration_ms",
            Help:    "Cache operation duration",
            Buckets: []float64{1, 5, 10, 50, 100},
        },
        []string{"operation"},
    )
)
```

### Hit Rate Calculation

```go
func (c *CacheMetrics) HitRate() float64 {
    hits := float64(c.hits)
    misses := float64(c.misses)
    total := hits + misses

    if total == 0 {
        return 0
    }
    return hits / total
}
```

### Dashboard Queries (Prometheus)

```promql
# Cache hit rate
rate(cache_hits_total[5m]) / (rate(cache_hits_total[5m]) + rate(cache_misses_total[5m]))

# Cache operations per second
sum(rate(cache_operation_duration_count[5m])) by (operation)

# Cache latency p99
histogram_quantile(0.99, rate(cache_operation_duration_bucket[5m]))
```

---

## Error Handling

### Cache Failure Fallback

```go
func (s *InventoryService) GetInventory(ctx context.Context, productID, warehouseID string) (*Inventory, error) {
    key := CacheKeyBuilder{}.InventoryKey(productID, warehouseID)

    // Try cache (fail open)
    var inventory Inventory
    err := s.cache.Get(ctx, key, &inventory)
    if err == nil {
        return &inventory, nil
    }

    // Cache miss or error - fetch from database
    s.metrics.CacheMiss("inventory")

    inventory, err = s.repo.GetInventory(ctx, productID, warehouseID)
    if err != nil {
        return nil, err
    }

    // Try to populate cache (don't fail if cache write fails)
    if err := s.cache.Set(ctx, key, inventory, 30*time.Second); err != nil {
        s.logger.Warn("failed to set cache", zap.Error(err))
    }

    return &inventory, nil
}
```

### Circuit Breaker for Cache

```go
type CircuitBreakerCache struct {
    redis   *RedisCache
    breaker *CircuitBreaker
}

func (c *CircuitBreakerCache) Get(ctx context.Context, key string, dest interface{}) error {
    if !c.breaker.Allow() {
        return ErrCircuitOpen
    }

    err := c.redis.Get(ctx, key, dest)
    if err != nil {
        c.breaker.RecordFailure()
        return err
    }

    c.breaker.RecordSuccess()
    return nil
}
```

---

## Redis Configuration

### Cluster Setup

```yaml
# redis-cluster.yml
cluster:
  nodes:
    - redis-node-1:6379
    - redis-node-2:6379
    - redis-node-3:6379
  pool_size: 10
  min_idle_conns: 5
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s
  max_retries: 3
```

### Memory Management

```yaml
# redis.conf
maxmemory 1gb
maxmemory-policy allkeys-lru
```

### Eviction Policies

| Policy | Description | Use Case |
|--------|-------------|----------|
| `noeviction` | Return errors when memory full | Critical data |
| `allkeys-lru` | Evict any key using LLRU | General caching |
| `volatile-lru` | Evict keys with TTL using LRU | Mixed workload |
| `allkeys-lfu` | Evict any key using LFU | Hot key caching |

---

## Summary

| Pattern | When to Use | Consistency |
|---------|-------------|-------------|
| Cache-Aside | General purpose | Eventual |
| Write-Through | Strong consistency needed | Strong |
| Write-Behind | High write throughput | Eventual |
| Read-Through | Transparent caching | Eventual |

| Strategy | Pros | Cons |
|----------|------|------|
| TTL-based | Simple, automatic | Stale data possible |
| Event-driven | Fresh data | Complex implementation |
| Manual | Precise control | Developer burden |
| Singleflight | Prevents stampede | Added complexity |
