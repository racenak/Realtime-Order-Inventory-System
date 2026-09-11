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
│                          TRAEFIK                                    │
│   (Reverse Proxy, TLS, Rate Limiting, JWT Auth, Routing)           │
└──────┬──────────────┬──────────────────────┬───────────────────────┘
       │              │                      │
       │              │                      │ WebSocket
┌──────▼──────┐ ┌─────▼──────┐ ┌────────────▼────────────┐
│   Order     │ │ Inventory  │ │    WebSocket Service    │
│   Service   │ │  Service   │ │   (Real-time Updates)   │
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
│              PostgreSQL (Primary) + Redis (Cache)                   │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                      SUPPORT SERVICES                               │
│  ┌──────────────┐                                                  │
│  │ Auth Service │  (JWT validation, ForwardAuth for Traefik)       │
│  │ :8083        │                                                  │
│  └──────────────┘                                                  │
└─────────────────────────────────────────────────────────────────────┘
```

## Services

### 1. Traefik (API Gateway)

| Property     | Description                          |
| ------------ | ------------------------------------ |
| Protocol     | HTTP/2, HTTPS, gRPC                  |
| Port         | 80 (HTTP), 443 (HTTPS)              |
| Responsibilities | Reverse Proxy, TLS Termination, Routing, Rate Limiting, Auth |

**Middleware:**
- Rate Limiting (per IP / per user)
- JWT Authentication
- Circuit Breaker
- Retry
- Compress
- CORS
- Strip Prefix

**Why Traefik:**
- Kubernetes-native with automatic service discovery
- Built-in Let's Encrypt ACME support
- Dynamic configuration via Kubernetes IngressRoutes
- Dashboard for monitoring
- Middleware plugins for extensibility

### 2. Order Service

| Property     | Description                          |
| ------------ | ------------------------------------ |
| Protocol     | gRPC (internal) / REST (gateway)     |
| Port         | 9001                                 |
| Responsibilities | Order CRUD, Order Lifecycle, Payment Integration |

**Patterns:** CQRS, Outbox Pattern, Event Publishing

### 3. Inventory Service

| Property     | Description                          |
| ------------ | ------------------------------------ |
| Protocol     | gRPC (internal) / REST (gateway)     |
| Port         | 9002                                 |
| Responsibilities | Stock Management, Reservations, Warehouse Operations |

**Patterns:** Optimistic Locking, Event Sourcing, CQRS

### 4. WebSocket Service

| Property     | Description                          |
| ------------ | ------------------------------------ |
| Protocol     | WebSocket                            |
| Port         | 8082                                |
| Responsibilities | Real-time Order Status, Inventory Updates, Notifications |

**Technologies:** Gorilla WebSocket, Redis Pub/Sub

### 5. Auth Service

| Property     | Description                          |
| ------------ | ------------------------------------ |
| Protocol     | HTTP                                |
| Port         | 8083                                |
| Responsibilities | JWT validation, User identity resolution |

**Technologies:** golang-jwt/jwt/v5

**Role in architecture:** Traefik forwards authentication requests to this service via ForwardAuth middleware. On valid JWT, it returns `X-User-Id` and `X-User-Role` headers that downstream services can trust.

## Data Flow

### Order Creation Flow

```
Client ──POST /orders──▶ Traefik ──ForwardAuth──▶ Auth Service
                                    │              (validate JWT)
                                    │              (return X-User-Id, X-User-Role)
                                    ▼
                              Order Service
                                    │
                                    ├──▶ Validate Request
                                    ├──▶ Create Order (DB)
                                    ├──▶ Save Outbox Event
                                    └──▶ Publish to Kafka
                                             │
                             ┌───────────────┘
                             ▼
             Inventory Service ◄── Consume Event
                             │
                             ├──▶ Reserve Stock
                             ├──▶ Update Inventory
                             └──▶ Publish Inventory Updated Event
                                      │
                                      ▼
                               WebSocket Service
                                 │
                                 └──▶ Notify Client (Real-time)
```

### Inventory Update Flow

```
Inventory Service ──Update Stock──▶ PostgreSQL
                                      │
                                      └──▶ Publish InventoryUpdated Event
                                              │
                              ┌────────────────┤
                              ▼                ▼
                      WebSocket Service   Order Service
                      (Notify Clients)    (Check Reservations)
```

## Event Bus (Kafka Topics)

| Topic                      | Producer         | Consumer         | Description                     |
| -------------------------- | ---------------- | ---------------- | ------------------------------- |
| `order.created`            | Order Service    | Inventory Service | Trigger stock reservation      |
| `order.confirmed`          | Order Service    | Inventory Service | Confirm stock deduction        |
| `order.cancelled`          | Order Service    | Inventory Service | Release reserved stock         |
| `inventory.reserved`       | Inventory Service| Order Service     | Confirm reservation success    |
| `inventory.updated`        | Inventory Service| WebSocket Service | Broadcast stock changes        |
| `order.status.changed`     | Order Service    | WebSocket Service | Notify order progress          |

## Database Schema

### Order Service Database

```sql
-- orders
id, user_id, status, total_amount, created_at, updated_at

-- order_items
id, order_id, product_id, quantity, unit_price, total_price

-- order_status_history
id, order_id, old_status, new_status, changed_at

-- outbox_events
id, aggregate_type, aggregate_id, event_type, payload, published, created_at
```

### Inventory Service Database

```sql
-- warehouses
id, name, location, is_active

-- inventory
id, product_id, warehouse_id, quantity, reserved_quantity, version

-- inventory_reservations
id, order_id, product_id, warehouse_id, quantity, status, expires_at

-- inventory_movements
id, product_id, warehouse_id, movement_type, quantity, reference_id, created_at
```

## Caching Strategy (Redis)

| Key Pattern              | TTL   | Description                    |
| ------------------------ | ----- | ------------------------------ |
| `inventory:{product_id}` | 30s   | Product stock across warehouses|
| `order:{order_id}`       | 60s   | Order details cache            |
| `user:{user_id}:orders`  | 30s   | User recent orders             |
| `rate_limit:{ip}`        | 60s   | API rate limiting              |

## Deployment

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Kubernetes Cluster                           │
│                                                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                         │
│  │ Traefik  │  │ Traefik  │  │ Traefik  │   (Edge Router)         │
│  │ Pod (x2) │  │ Pod (x2) │  │ Pod (x2) │                         │
│  └──────────┘  └──────────┘  └──────────┘                         │
│                                                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │ Order    │  │Inventory │  │ WebSocket│  │   Auth   │             │
│  │ Svc (x3) │  │Svc (x3) │  │ Svc (x3) │  │ Svc (x2) │             │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘             │
│                                                                     │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                         │
│  │PostgreSQL│  │  Redis   │  │  Kafka   │                         │
│  │ (HA)     │  │ (Cluster)│  │ (Cluster)│                         │
│  └──────────┘  └──────────┘  └──────────┘                         │
└─────────────────────────────────────────────────────────────────────┘
```

## Technology Stack

| Layer          | Technology                          |
| -------------- | ----------------------------------- |
| Language       | Go 1.22                             |
| API Gateway    | Traefik                             |
| HTTP Router    | Chi (per-service)                   |
| gRPC           | Google gRPC                         |
| Database       | PostgreSQL 16                       |
| Cache          | Redis 7                             |
| Message Broker | Apache Kafka                        |
| ORM            | sqlx / pgx                          |
| Config         | Viper                               |
| Logging        | Zap                                 |
| Testing        | Go testing + Testify                |
| Container      | Docker + Docker Compose             |
| Orchestration  | Kubernetes                          |

## Security

- JWT-based authentication via Traefik ForwardAuth middleware
- TLS termination at Traefik (Let's Encrypt ACME)
- Rate limiting at Traefik edge
- Service-to-service mTLS (internal gRPC)
- Row-level security in PostgreSQL
- Redis AUTH for cache layer
- Environment-based secret management
