# Realtime Order Inventory System

A distributed microservices system for order processing and inventory management with real-time synchronization.

## Architecture

- **Clean Architecture** - Domain, UseCase, Adapter layers
- **Microservices** - Order Service, Inventory Service
- **Event-Driven** - Kafka for async communication
- **CQRS** - Separate read/write models
- **Real-time** - WebSocket for live updates

## Tech Stack

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