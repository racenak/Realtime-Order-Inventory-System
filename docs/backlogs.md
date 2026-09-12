# Backlogs

## Progress

| Metric | Count |
|--------|-------|
| Completed | 95 |
| Remaining | 6 |
| Total | 101 |

---

## Completed

| ID | Category | Task | Notes |
|----|----------|------|-------|
| B-01 | Docs | Architecture documentation | 16 docs in `docs/` |
| B-02 | Core | Project folder structure (Clean Architecture) | `cmd/`, `internal/`, `pkg/`, `tests/`, `migrations/` |
| B-03 | Core | Domain layer — Order | `order.go`, `repository.go`, `errors.go` |
| B-04 | Core | Domain layer — Inventory | `inventory.go`, `repository.go`, `errors.go` |
| B-05 | Core | Usecase layer — Order | CreateOrder, GetOrder, CancelOrder, ListOrders, ConfirmOrder |
| B-06 | Core | Usecase layer — Inventory | GetStock, ReserveStock, ReleaseReservation, UpdateStock |
| B-07 | Adapter | Order PostgreSQL repository | Full CRUD, outbox events, `InTx` methods, `GetByIDempotencyKey` |
| B-08 | Adapter | Inventory PostgreSQL repository | ReserveQuantity/ReleaseQuantity, `InTx` methods |
| B-09 | Adapter | HTTP handlers — Order | Create, Get, List, Cancel, Idempotency-Key header |
| B-10 | Adapter | HTTP handlers — Inventory | GetStock, Reserve, Release, UpdateStock |
| B-11 | Data | Database schema (`migrations/`) | 8 tables, uuidv7(), indexes, 2 DBs |
| B-12 | Core | Shared packages (`pkg/`) | config, database, logger, response |
| B-13 | Infra | Service entry points (`cmd/`) | All build cleanly |
| B-14 | Tests | Integration tests — Order | 12 tests (repo + usecase + handler) |
| B-15 | Tests | Integration tests — Inventory | 14 tests (repo + usecase + handler) |
| B-16 | Tests | Test helpers (`tests/helpers/testdb.go`) | Auto table creation, cleanup, seed functions |
| B-17 | Kafka | Kafka producer (`pkg/kafka/producer.go`) | segmentio/kafka-go, `MessageWriter` interface |
| B-18 | Kafka | Kafka consumer (`pkg/kafka/consumer.go`) | Manual offset commit, DLQ, retry logic |
| B-19 | Kafka | Outbox polling publisher | ClaimBatch (FOR UPDATE SKIP LOCKED), `MessageWriter` interface |
| B-20 | Kafka | Order event handler — Inventory Service | Handles `order.created`, `order.cancelled` |
| B-21 | Kafka | Inventory event handler — Order Service | Handles `inventory.reserved`, `inventory.reservation_failed` |
| B-22 | Kafka | ConfirmOrder usecase method | Updates status to processing, creates outbox event |
| B-23 | Kafka | Kafka topics in docker-compose | 13 topics with auto-creation init container |
| B-24 | WS | WebSocket service skeleton | gorilla/websocket |
| B-25 | WS | WebSocket connection manager (hub pattern) | `pkg/websocket/hub.go`, data race fixed |
| B-26 | WS | Redis Pub/Sub subscriber | `internal/websocket/subscriber.go` |
| B-27 | WS | Client subscription management | Channels: order_id, product_id, customer_id |
| B-28 | WS | WebSocket heartbeat / ping-pong | 60s pong wait, 54s ping interval |
| B-29 | Infra | Dockerfile — Order Service | Multi-stage: golang:1.22-alpine → alpine:3.19 |
| B-30 | Infra | Dockerfile — Inventory Service | Multi-stage build |
| B-31 | Infra | Dockerfile — WebSocket Service | Multi-stage build |
| B-32 | Infra | docker-compose.yml — Full stack (15+ containers) | 2 Postgres, Redis, Kafka, kafka-init, auth, 3 services, Traefik, observability |
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
| B-50 | Gateway | Traefik API gateway | v2.11.57, file provider, custom Dockerfile, port 8088 |
| B-51 | Gateway | Routing: `/api/orders` → Order Service | PathPrefix match via file provider |
| B-52 | Gateway | Routing: `/api/inventory` → Inventory Service | PathPrefix match via file provider |
| B-53 | Gateway | Routing: `/ws` → WebSocket Service | PathPrefix match via file provider |
| B-54 | Gateway | TLS termination (Let's Encrypt ACME) | Traefik config ready, commented TLS options |
| B-55 | Gateway | Rate limiting middleware | 100 req/s, 50 burst, per-IP via Traefik |
| B-56 | Gateway | JWT ForwardAuth middleware | auth-service validates JWT, X-User-Id/Role headers |
| B-64 | Security | Request validation middleware spec | Covered in `security.md` |
| B-65 | Security | CORS configuration spec | Covered in `security.md` |
| B-66 | Security | SQL injection prevention audit | All queries use parameterized `$1`, `$2` — verified |
| B-67 | Security | Secrets management spec | Environment variables |
| B-68 | Obs | Structured logging across all services | zap logger in all services + shared `pkg/logger` |
| B-71 | Obs | Health check endpoints | `/health` via Traefik and direct |
| B-72 | Gateway | File provider (Podman compat) | Baked config in custom Dockerfile, watch=false |
| B-73 | Security | Security architecture document | Full threat model, defense-in-depth layers |
| B-74 | Security | JWT implementation spec | Token validation, HMAC-SHA256, issuer/audience/expiry |
| B-75 | Security | RBAC implementation spec | Permissions, roles, middleware, route authorization |
| B-76 | Security | Rate limiting spec (atomic) | Lua script, per-use-case separation |
| B-77 | Security | Idempotency schema spec | Client-sends-key approach, 409 Conflict on duplicate |
| B-78 | Security | Traefik gateway security spec | Middleware chain, gateway vs service responsibilities |
| B-79 | Security | Resource-level authorization spec | Order ownership, inventory invariants |
| B-80 | Docs | Auth Service docs (architecture, deployment, API spec) | Added Auth Service to 3 docs, fixed WebSocket port |
| B-69 | Obs | Prometheus + Grafana + Loki + Jaeger + OTEL Collector | Full observability stack, 9 Grafana dashboards |
| B-70 | Obs | /metrics endpoint in all Go services | promhttp.Handler() added to order, inventory, websocket, auth services |
| B-81 | Tests | Unit tests — Order domain/usecase | 5 domain + 7 usecase tests |
| B-82 | Tests | Unit tests — Inventory domain/usecase | 2 domain + 5 usecase tests |
| B-83 | Tests | Unit tests — Response/WebSocket/Metrics | 5 + 6 + 1 pkg tests |
| B-84 | Tests | Unit tests — Kafka event handlers | 13 tests (order + inventory handlers) |
| B-85 | Tests | Unit tests — Outbox publisher | 27 subtests (topic routing, batch, headers) |
| B-86 | Tests | Unit tests — Consumer retry/DLQ | 7 tests (retry, context cancel, defaults) |
| B-87 | Tests | E2E tests — Full HTTP lifecycle | 8 tests (CRUD, idempotency, errors) |
| B-88 | Tests | Mock infrastructure | 8 mock files (repos + use cases + kafka writer) |
| B-89 | Fix | Outbox publisher race condition | ClaimBatch with FOR UPDATE SKIP LOCKED |
| B-90 | Fix | DB transactions for all write use cases | BeginTx/Commit for CreateOrder, CancelOrder, ConfirmOrder, ReserveStock, ReleaseReservation |
| B-91 | Fix | Kafka consumer DLQ + retry | Manual offset commit, configurable retries, DLQ with error metadata headers |
| B-92 | Fix | WebSocket hub data race | Slow clients sent to unregister channel instead of delete() under RLock |
| B-93 | Fix | Idempotency key (client-sends-key) | GetByIDempotencyKey, 409 Conflict, separate endpoint |
| B-94 | Refactor | MessageWriter interface | Decouples Kafka components from concrete *kafka.Writer |
| B-95 | Fix | Broken &kafka.Writer{} in main.go | Properly configured writers passed to event handlers |

---

## Backlog

### Infrastructure

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-57 | Kubernetes manifests (Deployment, Service, Ingress) | Low | — |

### Testing & Quality

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-96 | ConfirmOrder integration test (happy path with DB) | Medium | — |
| B-97 | Benchmark tests for CalculateTotal, outbox polling | Low | — |
| B-98 | `go test -p 1` → full parallel test support | Low | — |

### Security

| ID | Task | Priority | Depends On |
|----|------|----------|------------|
| B-99 | Refresh token rotation + revocation | Medium | — |
| B-100 | Resource-level authorization (order ownership check) | Medium | — |
| B-101 | Database TLS encryption | Low | — |
