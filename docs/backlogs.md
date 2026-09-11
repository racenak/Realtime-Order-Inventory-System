# Backlogs

## Completed

| ID | Task | Status | Notes |
|----|------|--------|-------|
| B-01 | Architecture documentation | Done | 14 docs in `docs/` |
| B-02 | Project folder structure (Clean Architecture) | Done | `cmd/`, `internal/`, `pkg/`, `tests/`, `schema/` |
| B-03 | Domain layer — Order | Done | `order.go`, `repository.go`, `errors.go` |
| B-04 | Domain layer — Inventory | Done | `inventory.go`, `repository.go`, `errors.go` |
| B-05 | Usecase layer — Order (CreateOrder, GetOrder, CancelOrder, ListOrders) | Done | With total calculation, status validation |
| B-06 | Usecase layer — Inventory (GetStock, ReserveStock, ReleaseReservation, UpdateStock) | Done | With optimistic locking, quantity reservation |
| B-07 | Adapter layer — Order PostgreSQL repository | Done | Full CRUD, outbox events |
| B-08 | Adapter layer — Inventory PostgreSQL repository | Done | With ReserveQuantity/ReleaseQuantity |
| B-09 | HTTP handlers — Order | Done | Create, Get, List, Cancel |
| B-10 | HTTP handlers — Inventory | Done | GetStock, Reserve, Release, UpdateStock |
| B-11 | Database schema (`schema/init.sql`) | Done | 8 tables, uuidv7(), indexes |
| B-12 | Shared packages (`pkg/`) | Done | config, database, logger, response |
| B-13 | Service entry points (`cmd/`) | Done | Both build cleanly |
| B-14 | Integration tests — Order (repo + usecase + handler) | Done | 22 tests |
| B-15 | Integration tests — Inventory (repo + usecase + handler) | Done | 19 tests |
| B-16 | Test helpers (`tests/helpers/testdb.go`) | Done | Auto table creation, cleanup, seed functions |
| B-17 | Kafka producer (`pkg/kafka/producer.go`) | Done | segmentio/kafka-go |
| B-18 | Kafka consumer (`pkg/kafka/consumer.go`) | Done | MessageHandler callback |
| B-19 | Outbox polling publisher (`internal/order/adapter/kafka/outbox_publisher.go`) | Done | Polls outbox, publishes to Kafka topics |
| B-20 | Order event handler — Inventory Service | Done | Handles `order.created`, `order.cancelled` |
| B-21 | Inventory event handler — Order Service | Done | Handles `inventory.reserved`, `inventory.reservation_failed` |
| B-22 | ConfirmOrder usecase method | Done | Updates status to processing, creates outbox event |
| B-23 | Kafka topics in docker-compose | Done | 13 topics with auto-creation init container |
| B-24 | WebSocket service skeleton (`cmd/websocket-service/main.go`) | Done | gorilla/websocket |
| B-25 | WebSocket connection manager (hub pattern) | Done | `pkg/websocket/hub.go` |
| B-26 | Redis Pub/Sub subscriber | Done | `internal/websocket/subscriber.go` |
| B-27 | Client subscription management | Done | Channels: order_id, product_id, customer_id |
| B-28 | WebSocket heartbeat / ping-pong | Done | 60s pong wait, 54s ping interval |
| B-29 | Dockerfile — Order Service | Done | Multi-stage: golang:1.22-alpine → alpine:3.19 |
| B-30 | Dockerfile — Inventory Service | Done | Multi-stage build |
| B-31 | Dockerfile — WebSocket Service | Done | Multi-stage build |
| B-32 | docker-compose.yml — Full stack (8 containers) | Done | 2 Postgres, Redis, Kafka, kafka-init, 3 services |
| B-33 | docker-compose.yml — DB init via Dockerfile | Done | Dedicated DB Dockerfiles with schema SQL baked in |
| B-34 | Kafka healthcheck fix | Done | `nc -z` instead of bash `/dev/tcp`; advertised `kafka:9092` for inter-container |
| B-35 | SQL init scripts — fix `#` comments | Done | Changed to `--` for psql compatibility |
| B-36 | GitHub Actions CI/CD workflow | Done | Test (2 Postgres services), build, push to GHCR |
| B-37 | API bug fixes — ListOrders | Done | Handle empty customer_id to show all orders |
| B-38 | API bug fixes — CreateOrder items | Done | Populate SKU, product_name, unit_price from request |
| B-39 | API bug fixes — GetOrder | Done | Load order items from DB when fetching single order |
| B-40 | API bug fixes — ReserveStock | Done | Resolve warehouse_code to warehouse_id via GetWarehouseByCode |
| B-41 | API bug fixes — UpdateStock | Done | Upsert: create inventory if not exists |
| B-42 | Redis caching — Shared client factory | Done | `pkg/cache/redis.go` (go-redis/v9) |
| B-43 | Redis caching — Cache helper | Done | `pkg/cache/cache.go` (JSON serialize, TTL, prefix keys) |
| B-44 | Redis caching — Order cache decorator | Done | `order:{id}` 5min, `order:items:{id}` 5min, invalidates on write |
| B-45 | Redis caching — Inventory cache decorator | Done | `stock:{product_id}` 30s, `warehouse:code:{code}` 10min, invalidates on write |
| B-50 | Traefik API gateway | Done | v2.11.57, file provider, custom Dockerfile |
| B-51 | Routing: `/api/orders` → Order Service | Done | PathPrefix match via file provider |
| B-52 | Routing: `/api/inventory` → Inventory Service | Done | PathPrefix match via file provider |
| B-53 | Routing: `/ws` → WebSocket Service | Done | PathPrefix match via file provider |
| B-72 | File provider (Podman compat) | Done | Baked config in custom Dockerfile, watch=false |

## In Progress

| ID | Task | Status | Notes |
|----|------|--------|-------|
| — | — | — | — |

## Backlogs

### API Gateway (Traefik)

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-50 | Traefik static/dynamic config | Medium | — |
| B-51 | Routing: `/api/orders` → Order Service | Medium | B-50 |
| B-52 | Routing: `/api/inventory` → Inventory Service | Medium | B-50 |
| B-53 | Routing: `/ws` → WebSocket Service | Medium | B-50 |
| B-54 | TLS termination (Let's Encrypt ACME) | Low | B-50 |
| B-55 | Rate limiting middleware | Low | B-50 |
| B-56 | JWT ForwardAuth middleware | Low | B-50 |
| B-72 | File provider (Podman compat — no Docker socket) | Medium | B-50 |

### Infrastructure

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-57 | Kubernetes manifests (Deployment, Service, Ingress) | Low | — |

### Testing & Quality

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-58 | Unit tests — Order domain/usecase | Medium | — |
| B-59 | Unit tests — Inventory domain/usecase | Medium | — |
| B-60 | Integration tests — Kafka producer/consumer | Medium | B-18, B-19 |
| B-61 | Integration tests — WebSocket service | Medium | B-24 |
| B-62 | E2E tests (full order lifecycle) | Low | B-18, B-19, B-20, B-24 |
| B-63 | `go test -p 1` → full parallel test support | Low | — |

### Security

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-64 | Request validation middleware (input sanitization) | Medium | — |
| B-65 | CORS configuration | Medium | — |
| B-66 | SQL injection prevention audit | Medium | — |
| B-67 | Secrets management (env-based, no hardcoded) | Medium | — |

### Observability

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-68 | Structured logging across all services | Low | — |
| B-69 | Prometheus metrics endpoint | Low | — |
| B-70 | OpenTelemetry tracing setup | Low | — |
| B-71 | Health check endpoints (liveness + readiness) | Low | — |

---

## Summary

| Category | Completed | In Progress | Backlog |
|----------|-----------|-------------|---------|
| Core Domain & Usecase | 8 | 0 | 0 |
| Adapters (HTTP + DB) | 4 | 0 | 0 |
| Tests | 4 | 0 | 6 |
| Kafka | 7 | 0 | 0 |
| WebSocket | 5 | 0 | 0 |
| Redis | 4 | 0 | 0 |
| Infrastructure (Docker/CI) | 8 | 0 | 1 |
| API Gateway (Traefik) | 5 | 0 | 3 |
| Security | 0 | 0 | 4 |
| Observability | 0 | 0 | 4 |
| **Total** | **50** | **0** | **18** |
