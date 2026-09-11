# Backlogs

## Progress

| Metric | Count |
|--------|-------|
| Completed | 66 |
| Remaining | 3 |
| Total | 69 |

---

## Completed

| ID | Category | Task | Notes |
|----|----------|------|-------|
| B-01 | Docs | Architecture documentation | 14 docs in `docs/` |
| B-02 | Core | Project folder structure (Clean Architecture) | `cmd/`, `internal/`, `pkg/`, `tests/`, `schema/` |
| B-03 | Core | Domain layer — Order | `order.go`, `repository.go`, `errors.go` |
| B-04 | Core | Domain layer — Inventory | `inventory.go`, `repository.go`, `errors.go` |
| B-05 | Core | Usecase layer — Order | CreateOrder, GetOrder, CancelOrder, ListOrders |
| B-06 | Core | Usecase layer — Inventory | GetStock, ReserveStock, ReleaseReservation, UpdateStock |
| B-07 | Adapter | Order PostgreSQL repository | Full CRUD, outbox events |
| B-08 | Adapter | Inventory PostgreSQL repository | ReserveQuantity/ReleaseQuantity |
| B-09 | Adapter | HTTP handlers — Order | Create, Get, List, Cancel |
| B-10 | Adapter | HTTP handlers — Inventory | GetStock, Reserve, Release, UpdateStock |
| B-11 | Data | Database schema (`schema/init.sql`) | 8 tables, uuidv7(), indexes |
| B-12 | Core | Shared packages (`pkg/`) | config, database, logger, response |
| B-13 | Infra | Service entry points (`cmd/`) | All build cleanly |
| B-14 | Tests | Integration tests — Order | 22 tests (repo + usecase + handler) |
| B-15 | Tests | Integration tests — Inventory | 19 tests (repo + usecase + handler) |
| B-16 | Tests | Test helpers (`tests/helpers/testdb.go`) | Auto table creation, cleanup, seed functions |
| B-17 | Kafka | Kafka producer (`pkg/kafka/producer.go`) | segmentio/kafka-go |
| B-18 | Kafka | Kafka consumer (`pkg/kafka/consumer.go`) | MessageHandler callback |
| B-19 | Kafka | Outbox polling publisher | Polls outbox, publishes to Kafka topics |
| B-20 | Kafka | Order event handler — Inventory Service | Handles `order.created`, `order.cancelled` |
| B-21 | Kafka | Inventory event handler — Order Service | Handles `inventory.reserved`, `inventory.reservation_failed` |
| B-22 | Kafka | ConfirmOrder usecase method | Updates status to processing, creates outbox event |
| B-23 | Kafka | Kafka topics in docker-compose | 13 topics with auto-creation init container |
| B-24 | WS | WebSocket service skeleton | gorilla/websocket |
| B-25 | WS | WebSocket connection manager (hub pattern) | `pkg/websocket/hub.go` |
| B-26 | WS | Redis Pub/Sub subscriber | `internal/websocket/subscriber.go` |
| B-27 | WS | Client subscription management | Channels: order_id, product_id, customer_id |
| B-28 | WS | WebSocket heartbeat / ping-pong | 60s pong wait, 54s ping interval |
| B-29 | Infra | Dockerfile — Order Service | Multi-stage: golang:1.22-alpine → alpine:3.19 |
| B-30 | Infra | Dockerfile — Inventory Service | Multi-stage build |
| B-31 | Infra | Dockerfile — WebSocket Service | Multi-stage build |
| B-32 | Infra | docker-compose.yml — Full stack (10 containers) | 2 Postgres, Redis, Kafka, kafka-init, auth-service, 3 services, Traefik |
| B-33 | Infra | docker-compose.yml — DB init via Dockerfile | Dedicated DB Dockerfiles with schema SQL baked in |
| B-34 | Infra | Kafka healthcheck fix | `nc -z` instead of bash `/dev/tcp`; advertised `kafka:9092` |
| B-35 | Infra | SQL init scripts — fix `#` comments | Changed to `--` for psql compatibility |
| B-36 | Infra | GitHub Actions CI/CD workflow | Test (2 Postgres services), build, push to GHCR |
| B-37 | Bugfix | API bug fix — ListOrders | Handle empty customer_id to show all orders |
| B-38 | Bugfix | API bug fix — CreateOrder items | Populate SKU, product_name, unit_price from request |
| B-39 | Bugfix | API bug fix — GetOrder | Load order items from DB when fetching single order |
| B-40 | Bugfix | API bug fix — ReserveStock | Resolve warehouse_code to warehouse_id via GetWarehouseByCode |
| B-41 | Bugfix | API bug fix — UpdateStock | Upsert: create inventory if not exists |
| B-42 | Redis | Redis caching — Shared client factory | `pkg/cache/redis.go` (go-redis/v9) |
| B-43 | Redis | Redis caching — Cache helper | `pkg/cache/cache.go` (JSON serialize, TTL, prefix keys) |
| B-44 | Redis | Redis caching — Order cache decorator | `order:{id}` 5min, `order:items:{id}` 5min, invalidates on write |
| B-45 | Redis | Redis caching — Inventory cache decorator | `stock:{product_id}` 30s, `warehouse:code:{code}` 10min, invalidates on write |
| B-50 | Gateway | Traefik API gateway | v2.11.57, file provider, custom Dockerfile |
| B-51 | Gateway | Routing: `/api/orders` → Order Service | PathPrefix match via file provider |
| B-52 | Gateway | Routing: `/api/inventory` → Inventory Service | PathPrefix match via file provider |
| B-53 | Gateway | Routing: `/ws` → WebSocket Service | PathPrefix match via file provider |
| B-54 | Gateway | TLS termination (Let's Encrypt ACME) | Traefik config ready, commented TLS options |
| B-55 | Gateway | Rate limiting middleware | 100 req/s, 50 burst, per-IP via Traefik |
| B-56 | Gateway | JWT ForwardAuth middleware | auth-service validates JWT, X-User-Id/Role headers |
| B-64 | Security | Request validation middleware spec | Covered in `security_updated.md` |
| B-65 | Security | CORS configuration spec | Covered in `security_updated.md` |
| B-66 | Security | SQL injection prevention audit | All queries use parameterized `$1`, `$2` — verified |
| B-67 | Security | Secrets management spec | Covered in `security_updated.md` |
| B-68 | Obs | Structured logging across all services | zap logger in all services + shared `pkg/logger` |
| B-71 | Obs | Health check endpoints (liveness + readiness) | `/health` via Traefik and direct |
| B-72 | Gateway | File provider (Podman compat) | Baked config in custom Dockerfile, watch=false |
| B-73 | Security | Security architecture document | Full threat model, defense-in-depth layers |
| B-74 | Security | JWT implementation spec | Token gen, validation, middleware, context keys |
| B-75 | Security | RBAC implementation spec | Permissions, roles, middleware, route authorization |
| B-76 | Security | Rate limiting spec (atomic) | Lua script, per-use-case separation |
| B-77 | Security | Idempotency schema spec | DDL, uniqueness constraint, request hash |
| B-78 | Security | Traefik gateway security spec | Middleware chain, gateway vs service responsibilities |
| B-79 | Security | Resource-level authorization spec | Order ownership, inventory invariants |
| B-80 | Docs | Auth Service docs (architecture, deployment, API spec) | Added Auth Service to 3 docs, fixed WebSocket port |

---

## Backlog

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

### Observability

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-69 | Prometheus metrics endpoint | Low | — |
| B-70 | OpenTelemetry tracing setup | Low | — |
