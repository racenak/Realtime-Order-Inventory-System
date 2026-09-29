# Realtime Order Inventory System

A distributed microservices system for order processing and inventory management with real-time synchronization.

## Architecture (verified from source at `ddddab94`)

The system is a distributed Go 1.22 service mesh behind a Traefik gateway (`:8088`).

**Request flow (verified)**: Client → Traefik → `jwt-auth` ForwardAuth (`auth-service:8083/verify`) → protected-chain (`jwt-auth` + `rate-limit` + `security-headers`) → `order-service` (`:8080`) or `inventory-service` (`:8081`) or `websocket-service` (`:8082`).

**Services** (entry points in `cmd/*/main.go`):
- `order-service`: order domain + outbox publisher (`internal/order/adapter/kafka/outbox_publisher.go:59`) → Kafka topics `order.created`..`delivered`; consumes `inventory.reserved` (`cmd/order-service/main.go:101`)
- `inventory-service`: reserves stock; consumes `order.created` (`cmd/inventory-service/main.go:116`); writes `inventory.reservation_failed` (`internal/inventory/adapter/kafka/order_event_handler.go:181`)
- `websocket-service`: Redis pub/sub subscriber only (`internal/websocket/subscriber.go:35` — channels `order.events`, `inventory.events`); **verified gap: no `.Publish()` match**
- `auth-service`: JWT verification (`/verify`) + `/metrics`

**State & messaging** (verified from `docker-compose.yml`, `traefik-dynamic.yml`):
- PostgreSQL: `order-db` (`:5432`), `inventory-db` (`:5433`)
- Redis: 7-alpine (`:6379`) — 5min TTL (order), 30s TTL (inventory)
- Kafka: `apache/kafka:4.3.1`; topics `order.*`, `inventory.reserved`, `inventory.reservation_failed`, `dead_letter` (`pkg/kafka/consumer.go:196` DLQ)

**Observability** (verified from `config/otel/otel-collector.yml`, `config/prometheus/prometheus.yml`):
- OTLP receivers `:4317`/`:4318`; `prometheus/app-services` scrapes `order-service:8080`, `inventory-service:8081`, `websocket-service:8082`, `auth-service:8083`, `traefik:8080`
- Pipelines: traces → Tempo (`:4318`); metrics → Prometheus (`:8889`); logs → Loki (`:3100`)
- Grafana (`:3000`) queries Prometheus (`:9090`), Loki (`:3100`), Tempo (`:3200`)

**Verified gaps** (documented in architecture cards + sequence notes):
- Kafka `inventory.reserved` is consumed (`cmd/order-service/main.go:101`) but has **no in-repo producer**
- Redis `order.events` / `inventory.events` are subscribed (`subscriber.go:35`) but have **zero `.Publish()` calls** in repo
- `inventory.reservation_failed` is produced (`internal/inventory/adapter/kafka/order_event_handler.go:181`) and consumed correctly

**Visual diagrams (all verified against source)**
- [Architecture — runtime overview](docs/architecture/architecture-runtime.html) | [SVG](docs/architecture/architecture-runtime.svg)
- [Sequence — request flow](docs/architecture/sequence-request-flow.html)
- [Workflow — order creation](docs/architecture/workflow-order-flow.html)
- [Lifecycle — order status](docs/architecture/lifecycle-order-status.html)
- [Data Flow — pipeline](.archify/dataflow-order-pipeline-20250930/candidate.json) — validated; needs `fromSide`/`toSide` fix on stage-flow `f1`/`f4` for HTML render
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

## Evidence & Source Verification

All architecture claims above are tied to inspected source at commit `ddddab94` (`local-only` links; dirty worktree excludes uncommitted `AGENTS.md` / `graphify-out/`). Key proofs: `traefik-dynamic.yml` (routing, ForwardAuth, chains); `cmd/*-service/main.go` (service wiring, OTLP, Kafka consumer groups, Redis clients); `internal/order/adapter/kafka/*.go` (outbox topics, DLQ, consumer); `internal/websocket/subscriber.go:35` (Redis pub/sub subscribe — gap noted); `config/otel/otel-collector.yml` + `prometheus/prometheus.yml` + `grafana/provisioning/datasources/datasources.yml` (observability pipeline); `docker-compose.yml` (service topology, Kafka init topics, `k6` profile). The interactive knowledge graph (`graphify-out/graph.html`) and `GRAPH_REPORT.md` document 1147 nodes / 2940 edges / 78 communities; core abstractions (`Order`, `OutboxEvent`, `Inventory`, `Hub`) are confirmed by code structure rather than inference.