# Caching Strategy

## Redis Caching Layer

**Package**: `pkg/cache/redis.go`, `pkg/cache/cache.go`

### Cache Helper (`pkg/cache/cache.go`)

Generic cache helper wrapping Redis operations:
- JSON serialization/deserialization
- TTL-based expiration
- Key prefix namespacing
- Fail-open pattern (cache errors don't break reads)

### Client Factory (`pkg/cache/redis.go`)

Shared Redis client factory using `go-redis/v9`:
- Connection pooling
- Configurable via environment variables
- Health check support

---

## Cache Decorators

### Order Cache Decorator

**File**: `internal/order/adapter/cache/order_cache.go`

Wraps `OrderRepository` with Redis caching.

| Key Pattern | TTL | Description |
|-------------|-----|-------------|
| `order:{id}` | 5min | Order details by ID |
| `order:items:{id}` | 5min | Order items by order ID |

**Behavior**:
- **Read-through**: On cache miss, queries inner repository, caches result
- **Write-through**: On write (Create, UpdateStatus), invalidates cache
- **Fail-open**: On Redis error, falls through to DB without failing

### Inventory Cache Decorator

**File**: `internal/inventory/adapter/cache/inventory_cache.go`

Wraps `InventoryRepository` with Redis caching.

| Key Pattern | TTL | Description |
|-------------|-----|-------------|
| `stock:{product_id}` | 30s | Product stock across warehouses |
| `warehouse:code:{code}` | 10min | Warehouse code → ID mapping |

**Behavior**:
- **Read-through**: On cache miss, queries inner repository, caches result
- **Write-through**: On write (UpdateStock, ReserveQuantity, ReleaseQuantity), invalidates cache
- **Fail-open**: On Redis error, falls through to DB

---

## Invalidation Strategy

| Operation | Cache Keys Invalidated |
|-----------|----------------------|
| CreateOrder | `order:{id}` (no existing key) |
| CancelOrder | `order:{id}` |
| UpdateOrderStatus | `order:{id}` |
| UpdateStock | `stock:{product_id}` |
| ReserveStock | `stock:{product_id}` |
| ReleaseReservation | `stock:{product_id}` |

---

## Cache Warming

Not implemented. Cache is populated on first read (read-through).

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | (empty) | Redis password |
| `REDIS_DB` | `0` | Redis database number |

---

## Implementation Details

```go
// pkg/cache/cache.go
type Cache struct {
    client *redis.Client
    prefix string
    ttl    time.Duration
}

func (c *Cache) Get(ctx context.Context, key string, dest interface{}) error
func (c *Cache) Set(ctx context.Context, key string, value interface{}) error
func (c *Cache) Delete(ctx context.Context, key string) error
```

**Fail-open pattern**: All cache operations catch errors and log warnings instead of failing the request.

---

## Trade-offs

| Decision | Rationale |
|----------|-----------|
| Short TTLs (30s-5min) | Balance freshness vs performance |
| Fail-open | Cache failure shouldn't break the app |
| Write-through invalidation | Ensures stale data isn't served after writes |
| No cache warming | Simplicity; first-read penalty acceptable |
| JSON serialization | Simple, human-readable, debuggable |

---

## What's Implemented

- [x] `pkg/cache/redis.go` — Redis client factory
- [x] `pkg/cache/cache.go` — Cache helper (JSON serialize, TTL, prefix)
- [x] `order_cache.go` — Order cache decorator
- [x] `inventory_cache.go` — Inventory cache decorator
- [x] `docker-compose.yml` — Redis service (redis:7-alpine, port 6379)
- [x] `docker-compose.yml` — Redis exporter (port 9121)

## Not Implemented

- [ ] Cache warming on startup
- [ ] Cache statistics dashboard in Grafana
- [ ] Distributed cache invalidation across service instances
- [ ] Cache-aside pattern (currently read-through only)
