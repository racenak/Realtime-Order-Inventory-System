# Architecture Decision Records (ADR)

## ADR-01: Event-Driven Architecture

**Status**: Accepted

**Decision**: Use event-driven architecture with Kafka as the event bus.

**Rationale**:
- Loose coupling between Order and Inventory services
- Natural fit for inventory reservation/release patterns
- Enables real-time updates via WebSocket
- Kafka provides durability, ordering, and replay

**Consequences**:
- Eventual consistency between services
- Need for idempotent consumers
- Outbox pattern for reliable event publishing

---

## ADR-02: CQRS Pattern

**Status**: Accepted

**Decision**: Separate read and write models for Order and Inventory.

**Rationale**:
- Order reads (list, detail) need different views than writes (create, cancel)
- Inventory reads (stock check) optimized differently than writes (reserve, release)
- Enables independent scaling of read/write paths

**Consequences**:
- More complex code structure
- Potential for stale reads (mitigated by Redis caching)
- Clear separation of concerns

---

## ADR-03: Two Separate Databases

**Status**: Accepted

**Decision**: Order Service and Inventory Service each own their own PostgreSQL database.

**Rationale**:
- Strong data isolation per bounded context
- Independent schema evolution
- Prevents cross-service direct DB access
- Aligns with microservice ownership model

**Consequences**:
- Cross-service queries require API calls
- No JOIN across services (by design)
- Each service responsible for its own migrations

---

## ADR-04: Outbox Pattern for Event Publishing

**Status**: Accepted

**Decision**: Use Outbox Pattern with polling publisher for reliable event delivery.

**Implementation**:
- Write use cases insert outbox event in same DB transaction as business data
- Outbox publisher polls `outbox_events` table for PENDING events
- Publisher uses `ClaimBatch()` with `FOR UPDATE SKIP LOCKED` to prevent duplicate processing
- Status flow: `PENDING → CLAIMED → PUBLISHED/FAILED`
- Published messages include `event_type` header for consumer routing

**Rationale**:
- Guarantees at-least-once delivery without distributed transactions
- atomic business write + event write eliminates lost events
- `FOR UPDATE SKIP LOCKED` prevents race condition when multiple publisher instances compete
- DLQ (dead letter queue) captures events that exhaust retries

**Consequences**:
- Polling latency (configurable interval)
- Slight delay between write and event publication
- Failed events stored for retry/DLQ

---

## ADR-05: Idempotency Key (Client-Sends-Key)

**Status**: Accepted

**Decision**: Clients generate and send `Idempotency-Key` header on `POST /api/orders/`.

**Implementation**:
- Client generates unique UUID and sends as `Idempotency-Key` header
- Order use case calls `GetByIDempotencyKey(key)` before insert
- If key exists: returns `409 Conflict` with existing order in error body
- If key is new: inserts order, creates outbox event in same DB transaction

**Rationale**:
- Server does not need to track or store idempotency keys separately
- Existing unique index on `orders.idempotency_key` provides constraint
- Simpler than server-generated tokens
- Natural client-side retry with same key

**Consequences**:
- Client must store and reuse idempotency key on retries
- Duplicate submissions return 409 with existing order

---

## ADR-06: JWT-Based Authentication via Traefik ForwardAuth

**Status**: Accepted

**Decision**: Traefik validates JWTs via ForwardAuth middleware pointing to Auth Service.

**Implementation**:
- Traefik intercepts all `/api/*` requests
- ForwardAuth sends request to `http://auth-service:8083/validate`
- Auth Service decodes JWT, validates signature (HMAC-SHA256), issuer (`order-inventory-system`), audience (`order-inventory-api`), expiry
- On valid JWT: returns `200 OK` with `X-User-Id` and `X-User-Role` headers
- On invalid JWT: returns `401 Unauthorized`, Traefik blocks request

**Rationale**:
- Single point of authentication, services trust Traefik-injected headers
- Auth Service is stateless, no session storage
- JWT validation at edge prevents unauthenticated traffic from reaching services

**Consequences**:
- Services must not expose endpoints without Traefik (internal ports)
- Token refresh must be handled by client
- Secrets (JWT signing key) managed via environment variables

---

## ADR-07: File Provider for Traefik on Podman

**Status**: Accepted

**Decision**: Use Traefik file provider (not Docker provider) for Podman compatibility.

**Implementation**:
- Custom Traefik Dockerfile copies baked-in `traefik.yml` and `dynamic.yml`
- `--providers.file.watch=false` avoids file watching issues in containers
- All routes defined as static files in `dynamic.yml`

**Rationale**:
- Traefik Docker provider requires Docker socket, which Podman doesn't expose the same way
- File provider is runtime-agnostic
- Config baked into image ensures immutability

**Consequences**:
- Config changes require image rebuild (not runtime reload)
- `watch=false` avoids file watcher issues in containers

---

## ADR-08: MessageWriter Interface for Kafka

**Status**: Accepted

**Decision**: Define `MessageWriter` interface to decouple Kafka components from concrete `*kafka.Writer`.

**Implementation**:
```go
type MessageWriter interface {
    WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}
```

- Both `*kafka.Writer` and `MockMessageWriter` implement this interface
- `OutboxPublisher`, `InventoryEventHandler`, and `OrderEventHandler` depend on `MessageWriter`, not `*kafka.Writer`
- `OutboxPublisher` has `NewOutboxPublisherWithWriter()` constructor for DI
- Event handlers have `producer` field typed as `MessageWriter`

**Rationale**:
- Enables unit testing without Kafka infrastructure
- Decouples event handlers from Kafka client library details
- Prevents data race from mutating `writer.Topic` on shared writer instances

**Consequences**:
- Topic is set on each `kafka.Message` (not on the writer) to avoid concurrent mutation
- Tests inject `MockMessageWriter` instead of real Kafka

---

## ADR-09: ClaimBatch for Outbox Polling

**Status**: Accepted

**Decision**: Use `SELECT FOR UPDATE SKIP LOCKED` for atomic batch claiming of outbox events.

**Implementation**:
- `ClaimBatch(batchSize)` selects up to `batchSize` PENDING events
- Row-level lock with `SKIP LOCKED` allows concurrent publishers to claim different batches
- Status updated to CLAIMED, `published_at` set
- Returns claimed events for publishing

**Rationale**:
- `FOR UPDATE SKIP LOCKED` prevents duplicate processing across publisher instances
- No distributed lock manager needed
- Atomic claim + status update in single query
- `SKIP LOCKED` avoids blocking when another publisher claims rows

**Consequences**:
- PostgreSQL row-level locking
- Multiple publisher instances can safely process concurrently
- Missed claims (if publisher crashes after claim) are handled by re-claiming CLAIMED events

---

## ADR-10: Concurrency Control

**Status**: Accepted

**Decision**: Use optimistic locking for inventory updates, row-level locks for outbox processing.

**Implementation**:

**Optimistic Locking (Inventory)**:
- `UPDATE ... SET quantity_on_hand = $1 WHERE id = $2 AND version = $3`
- Check `rows affected == 1`, retry on conflict

**Row-Level Locking (Outbox)**:
- `SELECT ... FOR UPDATE SKIP LOCKED` for batch claiming
- Prevents duplicate processing across publisher instances

**Rationale**:
- Optimistic locking suitable for low-contention inventory updates
- Row-level locking for outbox prevents duplicate processing
- No distributed locks needed

**Consequences**:
- Need retry logic for optimistic lock failures
- `SKIP LOCKED` prevents blocking on lock contention

---

## ADR-11: Real-Time Updates via WebSocket + Redis Pub/Sub

**Status**: Accepted

**Decision**: Use Redis Pub/Sub as the message broker for WebSocket service fan-out.

**Implementation**:
- WebSocket service subscribes to Redis channels
- Order/Inventory services publish events to Redis after Kafka processing
- WebSocket hub maintains connected clients and broadcasts messages

**Rationale**:
- Redis Pub/Sub provides simple pub/sub without additional infrastructure
- WebSocket service is stateless (can scale horizontally)
- Fan-out pattern: one message reaches all subscribers

**Consequences**:
- Messages lost if no subscribers connected (acceptable for notifications)
- Redis becomes critical infrastructure for real-time features

---

## ADR-12: Podman as Container Runtime

**Status**: Accepted

**Decision**: Use Podman (rootless) instead of Docker for container management.

**Rationale**:
- Rootless containers for security
- daemonless architecture
- OCI-compatible
- Docker Compose compatibility via `podman compose`

**Consequences**:
- Must use `--userns=keep-id` for volume permissions
- SELinux labels (`:z`) needed for volume mounts
- File provider for Traefik (not Docker provider)

---

## ADR-13: Observability Stack

**Status**: Accepted

**Decision**: Full observability stack with Prometheus, Grafana, Loki, Jaeger, and OpenTelemetry Collector.

**Implementation**:
- **Prometheus**: Metrics TSDB, scrapes all services
- **Grafana**: 9 dashboards (Overview, Order, Inventory, WebSocket, Auth, Kafka, Redis, Infrastructure, API)
- **Loki**: Log aggregation, receives from Promtail
- **Promtail**: Log shipping from all containers
- **Jaeger**: Distributed trace UI
- **OTel Collector**: Receives OTLP gRPC from services, exports to Jaeger/Prometheus

**Custom Business Metrics (16)**:
- Order lifecycle: created, completed, cancelled, processing duration
- Inventory: reservations, releases, movements, stock level
- WebSocket: connections, messages sent
- Kafka: published, consumed, errors, DLQ
- HTTP: requests, duration

**Rationale**:
- Unified observability across all services
- Business metrics (not just infra) for domain visibility
- Distributed tracing for request flow across services

**Consequences**:
- All services must expose `/metrics` endpoint
- All services must instrument OpenTelemetry tracer
- Structured JSON logging via Zap for Loki ingestion

---

## ADR-14: Testing Strategy

**Status**: Accepted

**Decision**: Three-layer testing: Unit → Integration → E2E, with function-field mocks.

**Implementation**:

**Unit Tests (58)**:
- Domain logic, use cases, HTTP handlers, WebSocket hub, metrics, Kafka event handlers, outbox publisher, consumer
- Function-field mocks in `tests/mocks/` (no code generation)
- No infrastructure dependencies

**Integration Tests (43)**:
- Real PostgreSQL database
- `testcontainers-go` for DB lifecycle
- Covers repository layer, use case layer, HTTP handler layer
- Atomicity and concurrency tests

**E2E Tests (8)**:
- Full HTTP lifecycle through chi router
- Idempotency, error handling, edge cases

**Makefile Targets**:
- `make test-unit`: Unit tests only (no infra)
- `make test-integration`: Integration tests (starts Postgres)
- `make test-all`: Unit + Integration
- `make test`: Runs everything with `-p 1`

**Rationale**:
- Function-field mocks are simple, no external dependencies
- Integration tests catch real SQL/schema issues
- E2E tests verify full request lifecycle
- `-p 1` prevents race conditions between parallel test packages sharing DB

**Consequences**:
- Integration tests slower than unit tests (need DB)
- Mock maintenance is manual (no code generation)

---

## ADR-15: Database Transaction Pattern

**Status**: Accepted

**Decision**: All write operations use explicit DB transactions with `defer tx.Rollback()`.

**Implementation**:
- `BeginTxx(ctx, nil)` starts transaction
- `defer tx.Rollback()` ensures cleanup on error
- `tx.Commit()` on success (Rollback is no-op after commit)
- Business logic + outbox event write in same transaction

**Rationale**:
- `defer tx.Rollback()` is safe (no-op after commit)
- Prevents abandoned transactions on error paths
- Ensures atomicity: business data + outbox event written together

**Consequences**:
- All write use cases depend on `DBExecutor` interface
- `*sqlx.DB` and `*sqlx.Tx` both implement `DBExecutor`

---

## ADR-16: Consumer Retry + DLQ Pattern

**Status**: Accepted

**Decision**: Kafka consumers use manual offset commit, configurable retry, and DLQ publishing on final failure.

**Implementation**:
- `CommitInterval: 0` — manual offset commit (offsets committed only after successful processing)
- `MaxRetries: 3` (configurable), `RetryDelay: 1s` (configurable)
- On exhaustion: publish to DLQ topic (`<topic>.dlq`) with error metadata headers
- Error metadata headers: `error-message`, `retry-count`, `original-topic`, `original-partition`, `original-offset`, `timestamp`

**Rationale**:
- Manual offset commit prevents message loss on processing failure
- DLQ captures failed messages for later inspection
- Error metadata headers enable debugging without parsing message body

**Consequences**:
- DLQ topics must be created manually or via kafka-init
- Failed messages require manual reprocessing or alerting
