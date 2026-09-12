# Testing Strategy

## Overview

Three-layer testing approach: Unit → Integration → E2E.

| Layer | Count | Infrastructure | Speed |
|-------|-------|----------------|-------|
| Unit | 58 | None | Fast |
| Integration | 43 | PostgreSQL | Medium |
| E2E | 8 | PostgreSQL + HTTP | Slow |
| **Total** | **109** | | |

---

## Test Structure

```
tests/
├── unit/
│   ├── domain/          # Domain logic (pure functions)
│   ├── usecase/         # Business logic (mock repos)
│   └── pkg/             # Shared packages
├── integration/
│   ├── order/           # Repository + usecase + handler
│   ├── inventory/       # Repository + usecase + handler
│   └── http/            # HTTP handler integration
├── e2e/                 # Full HTTP lifecycle
└── mocks/               # Function-field mocks
```

---

## Unit Tests

### Domain Logic

```go
// tests/unit/domain/order_test.go
func TestCalculateTotal(t *testing.T) {
    order := &Order{Currency: "USD"}
    order.AddItem(OrderItem{Quantity: 2, UnitPrice: 10.00})
    order.AddItem(OrderItem{Quantity: 1, UnitPrice: 5.00})
    order.CalculateTotal()

    if order.Subtotal != 25.00 {
        t.Errorf("expected 25.00, got %f", order.Subtotal)
    }
}
```

**Coverage:**
- `Order.CalculateTotal()` — 5 tests (multi-item, single item, zero quantity, negative price, discounts)
- `Order.CanCancel()` — 7 statuses tested
- `Inventory.Available()` — 6 cases (normal, zero stock, all reserved, etc.)
- Domain error sentinels — via usecase tests

### Use Cases

```go
// tests/unit/usecase/order_test.go
func TestCreateOrder_EmptyCart(t *testing.T) {
    orderRepo := &mocks.MockOrderRepository{}
    usecase := NewOrderUseCase(orderRepo, nil, nil)

    _, err := usecase.CreateOrder(context.Background(), domain.CreateOrderRequest{
        CustomerID: "cust_123",
        Items:      []domain.CreateOrderItemRequest{},
    })

    if !errors.Is(err, domain.ErrInvalidRequest) {
        t.Errorf("expected ErrInvalidRequest, got %v", err)
    }
}
```

**Coverage:**
- Order: empty cart, invalid quantity, duplicate idempotency, not found, cancel guards, confirm guards
- Inventory: invalid quantity, insufficient stock, already released, not found

### Kafka Event Handlers

```go
// tests/unit/kafka_order_event_handler_test.go
func TestOrderEventHandler_ReserveStock(t *testing.T) {
    invUC := &mocks.MockInventoryUseCase{
        ReserveStockFn: func(ctx context.Context, req domain.ReserveStockRequest) (*domain.Reservation, error) {
            return &domain.Reservation{ID: "res_123"}, nil
        },
    }
    writer := &mocks.MockMessageWriter{}
    handler := NewOrderEventHandler(invUC, writer)

    msg := kafka.Message{
        Topic: "order.created",
        Value: eventJSON,
    }

    err := handler.Handle(context.Background(), msg)
    if err != nil {
        t.Errorf("unexpected error: %v", err)
    }
}
```

**Coverage:**
- Order events: order.created → reserve, order.cancelled → release, unknown event, bad JSON, failure event publishing
- Inventory events: inventory.reserved → confirm, inventory.reservation_failed → cancel, inventory.released (log only)
- Outbox publisher: topic routing (9 event types), batch publish, partial failure, ClaimBatch error
- Consumer: success, retry, exhaust retries, context cancelled, defaults, DLQ

### Mock Infrastructure

Function-field mocks (not code-generated):

```go
type MockOrderRepository struct {
    CreateFn         func(ctx context.Context, order *domain.Order) error
    GetByIDFn        func(ctx context.Context, id string) (*domain.Order, error)
    // ...
}

func (m *MockOrderRepository) Create(ctx context.Context, order *domain.Order) error {
    if m.CreateFn != nil {
        return m.CreateFn(ctx, order)
    }
    return nil
}
```

**8 mock files:**
- `order_repo.go` — OrderRepository
- `order_item_repo.go` — OrderItemRepository
- `outbox_repo.go` — OutboxRepository
- `inventory_repo.go` — InventoryRepository
- `reservation_repo.go` — ReservationRepository
- `movement_repo.go` — MovementRepository
- `usecases.go` — OrderUseCase + InventoryUseCase (for kafka)
- `kafka.go` — MockMessageWriter

---

## Integration Tests

### Testcontainers

```go
// tests/helpers/testdb.go
func SetupTestDB(t *testing.T) *sqlx.DB {
    ctx := context.Background()
    req := testcontainers.ContainerRequest{
        Image:        "postgres:18-alpine",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_DB":       "order_db",
            "POSTGRES_USER":     "postgres",
            "POSTGRES_PASSWORD": "postgres",
        },
    }
    // ...
}
```

### Repository Tests

```go
// tests/integration/order/order_repo_test.go
func TestOrderRepository_Create(t *testing.T) {
    db := testdb.SetupTestDB(t)
    repo := repository.NewOrderRepository(db)

    order := &domain.Order{
        ID:         uuid.New().String(),
        CustomerID: "cust_123",
        Status:     "pending",
    }

    err := repo.Create(context.Background(), order)
    if err != nil {
        t.Fatalf("failed to create order: %v", err)
    }

    // Verify
    fetched, err := repo.GetByID(context.Background(), order.ID)
    if err != nil {
        t.Fatalf("failed to get order: %v", err)
    }
    if fetched.CustomerID != "cust_123" {
        t.Errorf("expected cust_123, got %s", fetched.CustomerID)
    }
}
```

**Coverage:**
- Order CRUD, outbox events (ClaimBatch, MarkPublished, MarkFailed), idempotency key lookup
- Inventory CRUD, reservations, movements, concurrent access
- Atomicity tests (order + items in same transaction)
- Optimistic locking tests

### HTTP Handler Tests

```go
// tests/integration/http/order_handler_test.go
func TestCreateOrder(t *testing.T) {
    db := testdb.SetupTestDB(t)
    handler := handler.NewOrderHandler(usecase)

    body := `{"customer_id":"cust_123","items":[...]}`
    req := httptest.NewRequest("POST", "/api/orders", strings.NewReader(body))
    w := httptest.NewRecorder()

    handler.CreateOrder(w, req)

    if w.Code != http.StatusCreated {
        t.Errorf("expected 201, got %d", w.Code)
    }
}
```

---

## E2E Tests

```go
// tests/e2e/order_e2e_test.go
func TestCreateOrder(t *testing.T) {
    db := testdb.SetupTestDB(t)
    router := setupRouter(db)

    body := `{"customer_id":"cust_123","items":[...]}`
    req := httptest.NewRequest("POST", "/api/orders", strings.NewReader(body))
    w := httptest.NewRecorder()

    router.ServeHTTP(w, req)

    if w.Code != http.StatusCreated {
        t.Errorf("expected 201, got %d", w.Code)
    }

    var resp map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &resp)
    if resp["status"] != "pending" {
        t.Errorf("expected pending, got %s", resp["status"])
    }
}
```

**Coverage:**
- Full CRUD lifecycle
- Idempotency (duplicate key → 409)
- Error handling (invalid JSON, empty cart, not found)
- Total calculation verification

---

## Test Commands

```bash
# Unit tests only (no infrastructure)
make test-unit
go test ./tests/unit/...

# Integration tests (starts PostgreSQL)
make test-integration
go test ./tests/integration/...

# All tests
make test-all
make test

# Specific test
go test ./tests/unit/domain/ -run TestCalculateTotal

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Race detector
go test -race ./tests/unit/...
```

---

## CI/CD Integration

### GitHub Actions Workflow

```yaml
jobs:
  test:
    services:
      postgres:
        image: postgres:18-alpine
        env:
          POSTGRES_DB: order_db
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
        ports:
          - 5432:5432
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test -v -p 1 ./...
```

---

## Test Helpers

### Table Creation

```go
func createTables(db *sqlx.DB) {
    db.Exec(`
        CREATE TABLE IF NOT EXISTS orders (
            id VARCHAR(26) PRIMARY KEY,
            customer_id VARCHAR(26) NOT NULL,
            status VARCHAR(20) NOT NULL DEFAULT 'pending',
            ...
        )
    `)
}
```

### Cleanup

```go
func cleanupTables(db *sqlx.DB) {
    db.Exec("TRUNCATE order_status_history, order_items, outbox_events, orders CASCADE")
}
```

### Seed Data

```go
func seedOrder(db *sqlx.DB, order *domain.Order) {
    repo := repository.NewOrderRepository(db)
    repo.Create(context.Background(), order)
}
```

---

## What's Implemented

- [x] 58 unit tests (domain, usecase, kafka, pkg)
- [x] 43 integration tests (repository, usecase, handler)
- [x] 8 E2E tests (full HTTP lifecycle)
- [x] 8 mock files (repos, usecases, kafka writer)
- [x] Testcontainers for PostgreSQL
- [x] Test helpers (table creation, cleanup, seed)
- [x] GitHub Actions CI/CD
- [x] Makefile targets (test-unit, test-integration, test-all)
- [x] `-p 1` flag for parallel test safety

## Not Implemented

- [ ] Benchmark tests (testing.B)
- [ ] Fuzz tests (testing.F)
- [ ] Test coverage reporting in CI
- [ ] Mock generation (mockgen/counterfeiter)
- [ ] Redis integration tests
- [ ] WebSocket integration tests
