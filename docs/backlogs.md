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

## In Progress

| ID | Task | Status | Notes |
|----|------|--------|-------|
| — | — | — | — |

## Backlogs

### Redis Caching

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-30 | Redis client setup (`pkg/cache/redis.go`) | Medium | — |
| B-31 | Cache inventory stock (`inventory:{product_id}`, TTL 30s) | Medium | B-30 |
| B-32 | Cache order details (`order:{order_id}`, TTL 60s) | Medium | B-30 |
| B-33 | Cache invalidation on event consumption | Medium | B-31, B-32 |

### API Gateway (Traefik)

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-34 | Traefik static/dynamic config | Medium | — |
| B-35 | Routing: `/api/orders` → Order Service | Medium | B-34 |
| B-36 | Routing: `/api/inventory` → Inventory Service | Medium | B-34 |
| B-37 | Routing: `/ws` → WebSocket Service | Medium | B-34 |
| B-38 | TLS termination (Let's Encrypt ACME) | Low | B-34 |
| B-39 | Rate limiting middleware | Low | B-34 |
| B-40 | JWT ForwardAuth middleware | Low | B-34 |

### Infrastructure

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-41 | `docker-compose.yml` — Kafka + Zookeeper services | High | — |
| B-42 | `docker-compose.yml` — Redis service | Medium | — |
| B-43 | `docker-compose.yml` — Traefik service | Medium | B-34 |
| B-44 | `Dockerfile` — Order Service | Medium | — |
| B-45 | `Dockerfile` — Inventory Service | Medium | — |
| B-46 | `Dockerfile` — WebSocket Service | Medium | B-24 |
| B-47 | Kubernetes manifests (Deployment, Service, Ingress) | Low | B-44, B-45, B-46 |

### Testing & Quality

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-48 | Unit tests — Order domain/usecase | Medium | — |
| B-49 | Unit tests — Inventory domain/usecase | Medium | — |
| B-50 | Integration tests — Kafka producer/consumer | Medium | B-18, B-19 |
| B-51 | Integration tests — WebSocket service | Medium | B-24 |
| B-52 | E2E tests (full order lifecycle) | Low | B-18, B-19, B-20, B-24 |
| B-53 | `go test -p 1` → full parallel test support | Low | — |

### Security

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-54 | Request validation middleware (input sanitization) | Medium | — |
| B-55 | CORS configuration | Medium | — |
| B-56 | SQL injection prevention audit | Medium | — |
| B-57 | Secrets management (env-based, no hardcoded) | Medium | — |

### Observability

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-58 | Structured logging across all services | Low | — |
| B-59 | Prometheus metrics endpoint | Low | — |
| B-60 | OpenTelemetry tracing setup | Low | — |
| B-61 | Health check endpoints (liveness + readiness) | Low | — |

---

## Summary

| Category | Completed | In Progress | Backlog |
|----------|-----------|-------------|---------|
| Core Domain & Usecase | 8 | 0 | 0 |
| Adapters (HTTP + DB) | 4 | 0 | 0 |
| Tests | 4 | 0 | 5 |
| Kafka | 7 | 0 | 0 |
| WebSocket | 5 | 0 | 1 |
| Redis | 0 | 0 | 4 |
| Traefik | 0 | 0 | 7 |
| Infrastructure | 1 | 0 | 6 |
| Security | 0 | 0 | 4 |
| Observability | 0 | 0 | 4 |
| **Total** | **29** | **0** | **31** |
