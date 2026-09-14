# System Architecture

## Overview

Realtime Order Inventory System is a distributed microservices-based application that handles order processing and inventory management with real-time synchronization.

## Architecture Style

**Event-Driven Microservices** with CQRS (Command Query Responsibility Segregation) and Event Sourcing for critical flows.

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                          CLIENT LAYER                               │
│                    (Web / Mobile / Third-party)                     │
└─────────────────────────────┬───────────────────────────────────────┘
                              │ HTTP / HTTPS
┌─────────────────────────────▼───────────────────────────────────────┐
│                          TRAEFIK (:8088)                            │
│   (Reverse Proxy, Rate Limiting, JWT ForwardAuth, Routing)          │
│   File provider config, Traefik Dashboard :8090                     │
└──────┬──────────────┬──────────────────────┬───────────────────────┘
       │              │                      │ WebSocket
┌──────▼──────┐ ┌─────▼──────┐ ┌────────────▼────────────┐
│   Order     │ │ Inventory  │ │    WebSocket Service    │
│  Service    │ │  Service   │ │   (Real-time Updates)   │
│  (:8080)   │ │  (:8081)   │ │      (:8082)            │
└──────┬──────┘ └─────┬──────┘ └────────────┬────────────┘
       │              │                      │
       │         ┌────▼────┐                 │
       │         │  Kafka  │◄────────────────┘
       │         │ (Event  │
       │         │  Bus)   │
       │         └────┬────┘
       │              │
┌──────▼──────────────▼───────────────────────────────────────────────┐
│                         DATA LAYER                                  │
│     order_db (:5432)  inventory_db (:5433)   Redis (:6379)        │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                      SUPPORT SERVICES                               │
│  ┌──────────────┐  ┌────────────────────────────────────────────┐  │
│  │ Auth Service │  │ Observability Stack                         │  │
│  │   (:8083)    │  │ Prometheus :9090, Grafana :3000            │  │
│  │ JWT via      │  │ Loki :3100, Tempo :3200                   │  │
│  │ ForwardAuth  │  │ OTel Collector :8888, Promtail :9080        │  │
│  └──────────────┘  └────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

## Services

### 1. Traefik (API Gateway)

| Property     | Description                          |
| ------------ | ------------------------------------ |
| Protocol     | HTTP, HTTPS                          |
| Port         | 8088 (HTTP), 8090 (Dashboard)        |
| Config       | File provider (Podman compatible)    |
| Responsibilities | Reverse Proxy, Routing, Rate Limiting, JWT Auth |

**Middleware:**
- Rate Limiting (per IP, 100 req/s, 50 burst)
- JWT ForwardAuth (auth-service validates, returns X-User-Id/Role)
- Security Headers (HSTS, CSP, X-Frame-Options, etc.)
- CORS
- Strip Prefix

### 2. Order Service

| Property     | Description                          |
| ------------ | ------------------------------------ |
| Port         | 8080                                 |
| Responsibilities | Order CRUD, Order Lifecycle, Outbox Event Publishing |

**Patterns:** CQRS, Outbox Pattern (ClaimBatch with FOR UPDATE SKIP LOCKED), DB Transactions

### 3. Inventory Service

| Property     | Description                          |
| ------------ | ------------------------------------ |
| Port         | 8081                                 |
| Responsibilities | Stock Management, Reservations, Warehouse Operations |

**Patterns:** Optimistic Locking, Event Sourcing, DB Transactions

### 4. WebSocket Service

| Property     | Description                          |
| ------------ | ------------------------------------ |
| Port         | 8082                                 |
| Responsibilities | Real-time Order Status, Inventory Updates, Notifications |

**Technologies:** gorilla/websocket, Redis Pub/Sub, Hub pattern with unregister channel (data race fixed)

### 5. Auth Service

| Property     | Description                          |
| ------------ | ------------------------------------ |
| Port         | 8083                                 |
| Responsibilities | JWT validation, User identity resolution |

**Technologies:** golang-jwt/jwt/v5, HMAC-SHA256

**Role in architecture:** Traefik forwards authentication requests to this service via ForwardAuth middleware. On valid JWT, it returns `X-User-Id` and `X-User-Role` headers that downstream services can trust.

### 6. Observability Stack

| Service | Port | Purpose |
|---------|------|---------|
| Prometheus | 9090 | Metrics TSDB + scraping |
| Grafana | 3000 | 9 dashboards (Overview, Order, Inventory, WebSocket, Auth, Kafka, Redis, Infra, API) |
| Loki | 3100 | Log aggregation |
| Tempo | 3200 | Distributed trace UI |
| OTel Collector | 8888 | Telemetry pipeline |
| Promtail | 9080 | Log shipping |
| kafka-exporter | 9308 | Kafka metrics |
| redis-exporter | 9121 | Redis metrics |
| postgres-exporter | 9187/9188 | Per-DB metrics |

## Data Flow

### Order Creation Flow

```
Client ──POST /api/orders──▶ Traefik ──ForwardAuth──▶ Auth Service
                                    │              (validate JWT)
                                    │              (return X-User-Id, X-User-Role)
                                    ▼
                              Order Service
                                    │
                                    ├──▶ Validate Request
                                    ├──▶ BeginTx
                                    ├──▶ Create Order (DB)
                                    ├──▶ Create Order Items (DB)
                                    ├──▶ Create Outbox Event (DB)
                                    ├──▶ Commit Tx
                                    └──▶ Outbox Publisher polls
                                             │
                             ┌───────────────┘
                             ▼
              Inventory Service ◄── Consume Event
                             │
                             ├──▶ BeginTx
                             ├──▶ Reserve Stock
                             ├──▶ Create Reservation
                             ├──▶ Create Movement
                             ├──▶ Commit Tx
                             └──▶ Publish inventory.reserved Event
                                      │
                                      ▼
                               WebSocket Service
                                 │
                                 └──▶ Notify Client (Real-time)
```

### Order Cancellation Flow

```
Client ──POST /api/orders/:id/cancel──▶ Traefik ──▶ Order Service
                                                        │
                                                        ├──▶ BeginTx
                                                        ├──▶ Update Status
                                                        ├──▶ Create History
                                                        ├──▶ Create Outbox Event
                                                        ├──▶ Commit Tx
                                                        └──▶ order.cancelled Event
                                                                  │
                                                                  ▼
                                                          Inventory Service
                                                                  │
                                                                  ├──▶ BeginTx
                                                                  ├──▶ Release Reservation
                                                                  ├──▶ Create Movement
                                                                  ├──▶ Commit Tx
                                                                  └──▶ inventory.released Event
```

## Event Bus (Kafka Topics)

| Topic                      | Producer         | Consumer         | Description                     |
| -------------------------- | ---------------- | ---------------- | ------------------------------- |
| `order.created`            | Order Service    | Inventory Service | Trigger stock reservation      |
| `order.confirmed`          | Order Service    | Inventory Service | Confirm stock deduction        |
| `order.cancelled`          | Order Service    | Inventory Service | Release reserved stock         |
| `order.paid`              | Order Service    | —                 | Payment confirmed              |
| `order.shipped`           | Order Service    | —                 | Order shipped                  |
| `order.delivered`         | Order Service    | —                 | Order delivered                |
| `order.status.changed`    | Order Service    | WebSocket Service | Notify order progress          |
| `inventory.reserved`      | Inventory Service| Order Service     | Confirm reservation success    |
| `inventory.released`      | Inventory Service| Order Service     | Reservation released           |
| `inventory.reservation_failed` | Inventory Service | Order Service | Reservation failed             |
| `inventory.updated`       | Inventory Service| WebSocket Service | Broadcast stock changes        |
| `inventory.low_stock`     | Inventory Service| —                 | Low stock alert                |
| `default`                 | —                | —                 | Fallback for unknown events    |

**Consumer config:** Manual offset commit (`CommitInterval: 0`), configurable retry (`MaxRetries: 3`, `RetryDelay: 1s`), DLQ publishing on final failure.

## Database Schema

### Order Database (order_db :5432)

```sql
-- orders
id (UUID v7), customer_id, status, currency, subtotal, discount_amount,
shipping_fee, tax_amount, total_amount, shipping_address (JSONB),
idempotency_key, created_at, updated_at

-- order_items
id (UUID v7), order_id, product_id, sku, product_name, quantity,
unit_price, discount_amount, total_amount, created_at

-- order_status_history
id (UUID v7), order_id, old_status, new_status, reason, created_at

-- outbox_events
id (UUID v7), aggregate_type, aggregate_id, event_type, payload,
status (PENDING/CLAIMED/PUBLISHED/FAILED), created_at, published_at
```

### Inventory Database (inventory_db :5433)

```sql
-- warehouses
id (UUID v7), code, name, status, created_at

-- inventory
id (UUID v7), product_id, sku, warehouse_id, quantity_on_hand,
quantity_reserved, version, updated_at

-- inventory_reservations
id (UUID v7), order_id, order_item_id, product_id, sku, warehouse_id,
quantity, status, expires_at, created_at, updated_at

-- inventory_movements
id (UUID v7), product_id, sku, warehouse_id, movement_type, quantity,
reference_type, reference_id, notes, created_by, created_at
```

## Caching Strategy (Redis)

| Key Pattern              | TTL   | Description                    |
| ------------------------ | ----- | ------------------------------ |
| `order:{id}`             | 5min  | Order details cache            |
| `order:items:{id}`       | 5min  | Order items cache              |
| `stock:{product_id}`     | 30s   | Product stock across warehouses|
| `warehouse:code:{code}`  | 10min | Warehouse code → ID mapping    |

Cache decorators for Order and Inventory repos. Writes invalidate cache. Read-through pattern with fail-open on cache errors.

## Technology Stack

| Layer          | Technology                          |
| -------------- | ----------------------------------- |
| Language       | Go 1.25                             |
| API Gateway    | Traefik v2.11.57 (file provider)   |
| HTTP Router    | Chi (per-service)                   |
| Database       | PostgreSQL 18 (2 instances)         |
| Cache          | Redis 7                             |
| Message Broker | Apache Kafka (apache/kafka:4.3.1)   |
| ORM            | sqlx                                |
| Config         | envconfig                           |
| Logging        | Zap (structured JSON)               |
| Metrics        | Prometheus + custom (16 metrics)    |
| Tracing        | OpenTelemetry (OTLP gRPC → Tempo)  |
| Log Aggregation| Loki + Promtail                     |
| Testing        | Go testing + function-field mocks   |
| Container      | Podman + Podman Compose             |
| CI/CD          | GitHub Actions (lint, security, tests, build) |

## Security

- JWT-based authentication via Traefik ForwardAuth middleware
- Auth Service validates JWT (HMAC-SHA256, issuer, audience, expiration)
- Rate limiting at Traefik edge (100 req/s, 50 burst)
- SQL injection prevention (parameterized queries throughout)
- Parameterized queries with `$1`, `$2` placeholders
- Environment-based secret management
- Structured logging without sensitive data
- Security headers (HSTS, CSP, X-Frame-Options, Referrer-Policy, Permissions-Policy)
