# Testing Strategy

## Overview

| Level | Coverage Target | Tools |
|-------|-----------------|-------|
| Unit Tests | 80%+ | Go testing, Testify |
| Integration Tests | 70%+ | Go testing, testcontainers |
| E2E Tests | Critical paths | Go testing, HTTP clients |
| Performance Tests | SLA validation | k6, Vegeta |

---

## Test Pyramid

```
                    ┌─────────┐
                    │   E2E   │
                    │  Tests  │
                   ┌┴─────────┴┐
                   │Integration │
                   │   Tests    │
                  ┌┴───────────┴┐
                  │    Unit      │
                  │    Tests     │
                  └──────────────┘

    Coverage:     80%+           70%+           Critical
    Speed:        Slow           Medium          Fast
    Cost:         High           Medium          Low
```

---

## Unit Tests

### Order Service Tests

```go
package order_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

type MockOrderRepo struct {
    mock.Mock
}

func (m *MockOrderRepo) Create(ctx context.Context, order *Order) error {
    args := m.Called(ctx, order)
    return args.Error(0)
}

func (m *MockOrderRepo) GetByID(ctx context.Context, id string) (*Order, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(*Order), args.Error(1)
}

func TestCreateOrder_Success(t *testing.T) {
    // Arrange
    mockRepo := new(MockOrderRepo)
    mockPublisher := new(MockPublisher)
    service := NewOrderService(mockRepo, mockPublisher)

    req := CreateOrderRequest{
        CustomerID: "usr_001",
        Items: []OrderItemRequest{
            {ProductID: "prod_001", Quantity: 2},
        },
    }

    mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
    mockPublisher.On("Publish", mock.Anything, "order.created", mock.Anything).Return(nil)

    // Act
    order, err := service.CreateOrder(context.Background(), req)

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, order)
    assert.Equal(t, "pending_payment", order.Status)
    mockRepo.AssertExpectations(t)
    mockPublisher.AssertExpectations(t)
}

func TestCreateOrder_EmptyCart(t *testing.T) {
    // Arrange
    service := NewOrderService(nil, nil)
    req := CreateOrderRequest{
        CustomerID: "usr_001",
        Items:      []OrderItemRequest{},
    }

    // Act
    _, err := service.CreateOrder(context.Background(), req)

    // Assert
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "EMPTY_CART")
}

func TestCreateOrder_InvalidQuantity(t *testing.T) {
    // Arrange
    service := NewOrderService(nil, nil)
    req := CreateOrderRequest{
        CustomerID: "usr_001",
        Items: []OrderItemRequest{
            {ProductID: "prod_001", Quantity: -1},
        },
    }

    // Act
    _, err := service.CreateOrder(context.Background(), req)

    // Assert
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "INVALID_QUANTITY")
}
```

### Inventory Service Tests

```go
func TestReserveStock_Success(t *testing.T) {
    // Arrange
    mockRepo := new(MockInventoryRepo)
    service := NewInventoryService(mockRepo)

    mockRepo.On("GetInventory", mock.Anything, "prod_001", "wh_nyc").
        Return(&Inventory{QuantityOnHand: 100, QuantityReserved: 10, Version: 1}, nil)
    mockRepo.On("ReserveStock", mock.Anything, "prod_001", "wh_nyc", 5, 90).
        Return(nil)
    mockRepo.On("CreateReservation", mock.Anything, mock.Anything).
        Return(&Reservation{ID: "rsv_001"}, nil)

    // Act
    reservation, err := service.ReserveStock(context.Background(), ReserveStockRequest{
        OrderID:     "ord_001",
        ProductID:   "prod_001",
        WarehouseID: "wh_nyc",
        Quantity:    5,
    })

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, reservation)
    assert.Equal(t, "rsv_001", reservation.ID)
}

func TestReserveStock_InsufficientStock(t *testing.T) {
    // Arrange
    mockRepo := new(MockInventoryRepo)
    service := NewInventoryService(mockRepo)

    mockRepo.On("GetInventory", mock.Anything, "prod_001", "wh_nyc").
        Return(&Inventory{QuantityOnHand: 10, QuantityReserved: 8, Version: 1}, nil)

    // Act
    _, err := service.ReserveStock(context.Background(), ReserveStockRequest{
        ProductID:   "prod_001",
        WarehouseID: "wh_nyc",
        Quantity:    5,
    })

    // Assert
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "INSUFFICIENT_STOCK")
}

func TestReserveStock_ConcurrentModification(t *testing.T) {
    // Arrange
    mockRepo := new(MockInventoryRepo)
    service := NewInventoryService(mockRepo)

    mockRepo.On("GetInventory", mock.Anything, "prod_001", "wh_nyc").
        Return(&Inventory{QuantityOnHand: 100, QuantityReserved: 10, Version: 1}, nil)
    mockRepo.On("ReserveStock", mock.Anything, "prod_001", "wh_nyc", 5, 90).
        Return(ErrConcurrentModification)

    // Act
    _, err := service.ReserveStock(context.Background(), ReserveStockRequest{
        ProductID:   "prod_001",
        WarehouseID: "wh_nyc",
        Quantity:    5,
    })

    // Assert
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "CONCURRENT_MODIFICATION")
}
```

### Table-Driven Tests

```go
func TestValidateOrderRequest(t *testing.T) {
    tests := []struct {
        name    string
        req     CreateOrderRequest
        wantErr string
    }{
        {
            name: "valid request",
            req: CreateOrderRequest{
                CustomerID: "usr_001",
                Items: []OrderItemRequest{
                    {ProductID: "prod_001", Quantity: 1},
                },
            },
            wantErr: "",
        },
        {
            name: "empty cart",
            req: CreateOrderRequest{
                CustomerID: "usr_001",
                Items:      []OrderItemRequest{},
            },
            wantErr: "EMPTY_CART",
        },
        {
            name: "invalid quantity",
            req: CreateOrderRequest{
                CustomerID: "usr_001",
                Items: []OrderItemRequest{
                    {ProductID: "prod_001", Quantity: 0},
                },
            },
            wantErr: "INVALID_QUANTITY",
        },
        {
            name: "missing product_id",
            req: CreateOrderRequest{
                CustomerID: "usr_001",
                Items: []OrderItemRequest{
                    {Quantity: 1},
                },
            },
            wantErr: "product_id is required",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateOrderRequest(tt.req)
            if tt.wantErr == "" {
                assert.NoError(t, err)
            } else {
                assert.Contains(t, err.Error(), tt.wantErr)
            }
        })
    }
}
```

---

## Integration Tests

### Database Tests with Testcontainers

```go
package integration_test

import (
    "testing"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
    ctx := context.Background()

    pgContainer, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:16-alpine"),
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
    )
    if err != nil {
        t.Fatal(err)
    }

    connStr, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        t.Fatal(err)
    }

    // Run migrations
    runMigrations(db)

    cleanup := func() {
        db.Close()
        pgContainer.Terminate(ctx)
    }

    return db, cleanup
}

func TestOrderRepository_Create(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()

    repo := NewOrderRepo(db)
    order := &Order{
        ID:         "ord_001",
        CustomerID: "usr_001",
        Status:     "pending_payment",
    }

    err := repo.Create(context.Background(), order)
    assert.NoError(t, err)

    // Verify
    fetched, err := repo.GetByID(context.Background(), "ord_001")
    assert.NoError(t, err)
    assert.Equal(t, order.ID, fetched.ID)
}
```

### Redis Integration Tests

```go
func setupTestRedis(t *testing.T) (*redis.Client, func()) {
    ctx := context.Background()

    redisContainer, err := testcontainers.GenericContainer(ctx,
        testcontainers.GenericContainerRequest{
            ContainerRequest: testcontainers.ContainerRequest{
                Image:        "redis:7-alpine",
                ExposedPorts: []string{"6379/tcp"},
            },
            Started: true,
        },
    )
    if err != nil {
        t.Fatal(err)
    }

    host, _ := redisContainer.Host(ctx)
    port, _ := redisContainer.MappedPort(ctx, "6379")

    client := redis.NewClient(&redis.Options{
        Addr: fmt.Sprintf("%s:%s", host, port.Port()),
    })

    cleanup := func() {
        client.Close()
        redisContainer.Terminate(ctx)
    }

    return client, cleanup
}

func TestCache_SetAndGet(t *testing.T) {
    client, cleanup := setupTestRedis(t)
    defer cleanup()

    cache := NewRedisCache(client, "test", 5*time.Minute)

    // Set
    err := cache.Set(context.Background(), "key1", "value1")
    assert.NoError(t, err)

    // Get
    var result string
    err = cache.Get(context.Background(), "key1", &result)
    assert.NoError(t, err)
    assert.Equal(t, "value1", result)
}
```

### Kafka Integration Tests

```go
func setupTestKafka(t *testing.T) (testcontainers.Container, func()) {
    ctx := context.Background()

    kafkaContainer, err := testcontainers.GenericContainer(ctx,
        testcontainers.GenericContainerRequest{
            ContainerRequest: testcontainers.ContainerRequest{
                Image:        "confluentinc/cp-kafka:7.6.0",
                ExposedPorts: []string{"9092/tcp"},
                Env: map[string]string{
                    "KAFKA_NODE_ID":               "1",
                    "KAFKA_LISTENER_SECURITY_PROTOCOL_MAP": "PLAINTEXT:PLAINTEXT",
                    "KAFKA_ADVERTISED_LISTENERS":  "PLAINTEXT://localhost:9092",
                },
            },
            Started: true,
        },
    )
    if err != nil {
        t.Fatal(err)
    }

    cleanup := func() {
        kafkaContainer.Terminate(ctx)
    }

    return kafkaContainer, cleanup
}

func TestKafka_PublishAndConsume(t *testing.T) {
    container, cleanup := setupTestKafka(t)
    defer cleanup()

    // Create producer and consumer
    producer := NewKafkaProducer(container.Endpoint())
    consumer := NewKafkaConsumer(container.Endpoint(), "test-group")

    // Publish
    err := producer.Publish(context.Background(), "test-topic", "test-key", []byte("test-value"))
    assert.NoError(t, err)

    // Consume
    msg, err := consumer.Consume(context.Background(), "test-topic", 5*time.Second)
    assert.NoError(t, err)
    assert.Equal(t, []byte("test-value"), msg.Value)
}
```

---

## End-to-End Tests

### HTTP API Tests

```go
func TestE2E_OrderCreation(t *testing.T) {
    // Setup
    server := setupTestServer(t)
    defer server.Close()

    client := &http.Client{Timeout: 10 * time.Second}

    // Create order
    orderReq := CreateOrderRequest{
        CustomerID: "usr_001",
        Items: []OrderItemRequest{
            {ProductID: "prod_001", Quantity: 2},
        },
    }
    body, _ := json.Marshal(orderReq)

    resp, err := client.Post(
        server.URL+"/api/orders",
        "application/json",
        bytes.NewReader(body),
    )
    assert.NoError(t, err)
    assert.Equal(t, http.StatusCreated, resp.StatusCode)

    var orderResp OrderResponse
    json.NewDecoder(resp.Body).Decode(&orderResp)
    assert.Equal(t, "pending_payment", orderResp.Data.Status)

    // Get order
    resp, err = client.Get(server.URL + "/api/orders/" + orderResp.Data.ID)
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)

    // Cancel order
    resp, err = client.Post(
        server.URL+"/api/orders/"+orderResp.Data.ID+"/cancel",
        "application/json",
        strings.NewReader(`{"reason": "test"}`),
    )
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

### WebSocket Tests

```go
func TestE2E_WebSocketNotifications(t *testing.T) {
    // Connect to WebSocket
    conn, _, err := websocket.DefaultDialer.Dial(
        "ws://localhost:8081/ws?token=test-token",
        nil,
    )
    assert.NoError(t, err)
    defer conn.Close()

    // Subscribe to order updates
    subscribeMsg := map[string]interface{}{
        "action":   "subscribe",
        "channels": []string{"order:ord_001"},
    }
    err = conn.WriteJSON(subscribeMsg)
    assert.NoError(t, err)

    // Simulate order status change (via API)
    go func() {
        time.Sleep(1 * time.Second)
        http.Post("http://localhost:8080/api/orders/ord_001/ship", "application/json", nil)
    }()

    // Read notification
    conn.SetReadDeadline(time.Now().Add(5 * time.Second))
    _, message, err := conn.ReadMessage()
    assert.NoError(t, err)

    var event Event
    json.Unmarshal(message, &event)
    assert.Equal(t, "order.status_changed", event.Type)
}
```

---

## Performance Tests

### k6 Load Test

```javascript
// load_test.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '30s', target: 20 },  // Ramp up
        { duration: '1m', target: 50 },   // Stay at 50
        { duration: '30s', target: 0 },   // Ramp down
    ],
    thresholds: {
        http_req_duration: ['p(95)<500'],  // 95% under 500ms
        http_req_failed: ['rate<0.01'],    // Less than 1% errors
    },
};

export default function () {
    const payload = JSON.stringify({
        customer_id: 'usr_001',
        items: [{ product_id: 'prod_001', quantity: 1 }],
    });

    const params = {
        headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer test-token',
        },
    };

    const res = http.post('http://localhost:8080/api/orders', payload, params);

    check(res, {
        'status is 201': (r) => r.status === 201,
        'response time < 500ms': (r) => r.timings.duration < 500,
    });

    sleep(1);
}
```

### Vegeta Load Test

```go
func TestLoad_OrderCreation(t *testing.T) {
    target := vegeta.Target{
        Method: "POST",
        URL:    "http://localhost:8080/api/orders",
        Header: http.Header{
            "Content-Type":  {"application/json"},
            "Authorization": {"Bearer test-token"},
        },
        Body: []byte(`{
            "customer_id": "usr_001",
            "items": [{"product_id": "prod_001", "quantity": 1}]
        }`),
    }

    attacker := vegeta.NewAttacker()
    rate := vegeta.Rate{Freq: 100, Per: time.Second}
    duration := 1 * time.Minute

    var metrics vegeta.Metrics
    for res := range attacker.Attack(target.Serve, rate, duration, "Load Test") {
        metrics.Add(res)
    }
    metrics.Close()

    // Check results
    if metrics.Success < 0.99 {
        t.Errorf("Success rate too low: %.2f%%", metrics.Success*100)
    }
    if metrics.Latencies.P95 > 500*time.Millisecond {
        t.Errorf("P95 latency too high: %v", metrics.Latencies.P95)
    }
}
```

---

## Test Organization

```
├── unit/
│   ├── order/
│   │   ├── service_test.go
│   │   └── repository_test.go
│   ├── inventory/
│   │   ├── service_test.go
│   │   └── repository_test.go
│   └── common/
│       └── validator_test.go
├── integration/
│   ├── database_test.go
│   ├── redis_test.go
│   └── kafka_test.go
├── e2e/
│   ├── order_flow_test.go
│   └── websocket_test.go
├── performance/
│   ├── load_test.go
│   └── stress_test.go
└── helpers/
    ├── testdb.go
    └── fixtures.go
```

---

## Test Helpers

```go
// testhelpers/testdb.go
func SetupTestDB(t *testing.T) *sql.DB {
    t.Helper()

    db, err := sql.Open("postgres", os.Getenv("TEST_DATABASE_URL"))
    if err != nil {
        t.Fatal(err)
    }

    // Clean database before test
    db.Exec("TRUNCATE orders, order_items, inventory CASCADE")

    t.Cleanup(func() {
        db.Close()
    })

    return db
}

// testhelpers/fixtures.go
func CreateTestOrder(db *sql.DB, id string) {
    db.Exec(`
        INSERT INTO orders (id, customer_id, status, currency, subtotal, total_amount, created_at, updated_at)
        VALUES ($1, 'usr_001', 'pending_payment', 'USD', 100.00, 100.00, NOW(), NOW())`,
        id,
    )
}

func CreateTestInventory(db *sql.DB, productID, warehouseID string, quantity int) {
    db.Exec(`
        INSERT INTO inventory (id, product_id, sku, warehouse_id, quantity_on_hand, quantity_reserved, version, updated_at)
        VALUES (uuid_generate_v4(), $1, 'SKU-001', $2, $3, 0, 0, NOW())`,
        productID, warehouseID, quantity,
    )
}
```

---

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/test.yml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:16
        env:
          POSTGRES_DB: testdb
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
        ports:
          - 5432:5432

      redis:
        image: redis:7
        ports:
          - 6379:6379

    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run Unit Tests
        run: go test -v ./unit/... -coverprofile=coverage.out

      - name: Run Integration Tests
        run: go test -v ./integration/... -tags=integration
        env:
          TEST_DATABASE_URL: postgres://test:test@localhost:5432/testdb?sslmode=disable
          TEST_REDIS_URL: localhost:6379

      - name: Upload Coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
```

---

## Coverage Reports

```bash
# Generate coverage report
go test ./... -coverprofile=coverage.out

# View coverage
go tool cover -html=coverage.out

# Coverage by package
go test ./... -cover | grep -v "no test files"
```

### Coverage Thresholds

| Package | Minimum Coverage |
|---------|------------------|
| order | 85% |
| inventory | 85% |
| websocket | 80% |
| common | 90% |
| Overall | 80% |
