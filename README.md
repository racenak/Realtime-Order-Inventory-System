# Realtime Order Inventory System

A distributed microservices system for order processing and inventory management with real-time synchronization.

## Architecture

- **Clean Architecture** - Domain, UseCase, Adapter layers
- **Microservices** - Order Service, Inventory Service
- **Event-Driven** - Kafka for async communication
- **CQRS** - Separate read/write models
- **Real-time** - WebSocket for live updates

### Verified Runtime Architecture

**Diagram artifacts (verified from repo sources)**
- [Architecture (runtime overview)](https://racenak.github.io/Realtime-Order-Inventory-System/docs/architecture/architecture-runtime.html)
- [Sequence (request flow)](https://racenak.github.io/Realtime-Order-Inventory-System/docs/architecture/sequence-request-flow.html)
- [Workflow (order creation)](https://racenak.github.io/Realtime-Order-Inventory-System/docs/architecture/workflow-order-flow.html)
- [Lifecycle (status)](https://racenak.github.io/Realtime-Order-Inventory-System/docs/architecture/lifecycle-order-status.html)

- **Language:** Go 1.22
- **Database:** PostgreSQL 16
- **Cache:** Redis 7
- **Message Broker:** Apache Kafka
- **API Gateway:** Traefik
- **Orchestration:** Kubernetes

## Project Structure

```
├── cmd/                        # Application entry points
│   ├── order-service/
│   └── inventory-service/
├── internal/                   # Private business logic
│   ├── order/
│   │   ├── domain/             # Entities + Repository interfaces
│   │   ├── usecase/            # Business rules
│   │   └── adapter/            # Postgres, HTTP implementations
│   └── inventory/
│       ├── domain/
│       ├── usecase/
│       └── adapter/
├── pkg/                        # Shared utilities
├── schema/                     # Database schema
│   └── init.sql               # Unified schema (all tables)
└── deployments/                # Kubernetes, Traefik configs
```

## Quick Start

### 1. Start Infrastructure

```bash
docker compose up -d
```

This starts:
- PostgreSQL (port 5432) - auto-creates tables via `schema/init.sql`
- Redis (port 6379)
- Kafka (port 9092)

### 2. Run Services

```bash
# Terminal 1
go run ./cmd/order-service

# Terminal 2
go run ./cmd/inventory-service
```

### 3. Test APIs

**Create Order:**
```bash
curl -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "550e8400-e29b-41d4-a716-446655440000",
    "items": [
      {"product_id": "550e8400-e29b-41d4-a716-446655440001", "quantity": 2}
    ],
    "shipping_address": {
      "street": "123 Main St",
      "city": "New York",
      "state": "NY",
      "zip": "10001",
      "country": "US"
    }
  }'
```

**Get Stock:**
```bash
curl http://localhost:8081/api/inventory/stock/550e8400-e29b-41d4-a716-446655440001
```

## Database Schema

All tables are created in a single PostgreSQL database: `order_inventory`

| Table | Description |
|-------|-------------|
| orders | Order records |
| order_items | Order line items |
| order_status_history | Status change audit |
| outbox_events | Event publishing queue |
| warehouses | Warehouse locations |
| inventory | Stock levels per product/warehouse |
| inventory_reservations | Reserved stock for orders |
| inventory_movements | Stock movement audit trail |

## Clean Architecture

```
Request → Handler → UseCase → Repository → Database
   │          │          │          │
   │          │          │          └─ Implements interface
   │          │          └─ Uses interface (port)
   │          └─ Calls use case
   └─ HTTP handling
```

**Dependency Rule:** Dependencies point inward only.

## Development

```bash
# Build
make build

# Run tests
make test

# Lint
make lint

# Reset database
make db-reset
```

## API Endpoints

### Order Service (port 8080)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /api/orders | Create order |
| GET | /api/orders/{id} | Get order |
| GET | /api/orders | List orders |
| POST | /api/orders/{id}/cancel | Cancel order |

### Inventory Service (port 8081)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | /api/inventory/stock/{product_id} | Get stock |
| POST | /api/inventory/reserve | Reserve stock |
| POST | /api/inventory/release/{id} | Release reservation |
| PUT | /api/inventory/stock | Update stock |

## License

MIT

## Knowledge Graph (graphify)

Graph rebuilt at `5a5688e` (includes new `docs/architecture/` artifacts + `.archify/` candidates + updated README).

- **Graph**: [`graphify-out/graph.html`](graphify-out/graph.html) | [`graph.json`](graphify-out/graph.json) | [`GRAPH_REPORT.md`](graphify-out/GRAPH_REPORT.md)
- **Scale**: 1147 nodes · 2940 edges · 78 communities · 94% EXTRACTED / 5% INFERRED (avg conf 0.9) · 0 import cycles · 155 inferred edges
- **Core abstractions (god nodes)**: `Order` (35 edges), `setupOrderUsecase()` / `setupInventoryUsecase()`, `main()`, `OutboxEvent`, `Inventory`, `Hub`, `NewCreateOrderUseCase()`, `NewOrderRepository()` — confirmed by `cmd/order-service/main.go`, `internal/order/usecase/`, `internal/inventory/adapter/`
- **Verified architecture connections** (from graph + source): `traefik-dynamic.yml` ↔ `auth-service:8083/verify`; `order-service` ↔ `order-db` (SQL + outbox); `kafka` topics (`order.*`, `inventory.reservation_failed`, `dead_letter`) ↔ consumer groups (`inventory-service-orders`, `order-service-inventory`); `redis` (cache/pubsub) ↔ `websocket-service` subscriber (`internal/websocket/subscriber.go:35` — subscribe-only, zero `.Publish()`); `otel-collector` ↔ `prometheus` (`:8889`) / `loki` / `tempo` ↔ `grafana`
- **Surprising graph connections** (verified by source inspection): `jwt-auth ForwardAuth Middleware (K8s CRD)` semantically similar to `jwt-auth ForwardAuth Middleware (file provider)` (`traefik-dynamic.yml` ↔ `deployments/traefik/ingress-routes.yml`); `inventory-service Container (:8081)` ↔ `Inventory Service` (`docker-compose.yml` ↔ `README.md`); `order-service Container (:8080)` ↔ `Order Service`
- **Key communities**: System Architecture & ADRs; Kubernetes Observability Storage; Traefik Routing & Middleware; Auth Deployment & Monitoring; Event-Driven Design Docs; Cache Abstraction / Read-Through / Write-Through; Outbox Publisher / DLQ; WebSocket / Redis PubSub; OpenTelemetry Tracing; Grafana Dashboards; Transaction Boundary Patterns; E2E / Integration / Unit Tests
- **Data-quality note**: `graphify-out/` was rebuilt at `ddddab94` with `python3` interpreter fixed (prior failure at `sql/sqlite` extraction), SQL nodes restored (99 SQL nodes), old labels recovered (67 + 11 SQL labels), interpreter sidecar corrected to `/usr/bin/python3`