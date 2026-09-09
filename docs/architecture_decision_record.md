# Architecture Decision Records

## Overview

This document records key architectural decisions for the Realtime Order Inventory System (ROIS). Each decision follows the ADR format: Title, Status, Context, Decision, Consequences.

---

## ADR-001: Microservices Architecture

**Status:** Accepted

**Context:**
We need to build a system that handles order processing and inventory management with real-time synchronization. The system must support independent scaling, deployment, and development of different components.

**Decision:**
Adopt a microservices architecture with separate services for Order, Inventory, and WebSocket functionality.

**Rationale:**
- Independent scaling: Order service may need more resources during peak hours than inventory
- Independent deployment: Changes to notification logic don't require redeploying order service
- Team autonomy: Different teams can own different services
- Technology flexibility: Each service can use optimized libraries

**Consequences:**

| Positive | Negative |
|----------|----------|
| Independent scaling per service | Increased operational complexity |
| Fault isolation | Network latency between services |
| Independent deployments | Distributed transaction challenges |
| Team autonomy | More infrastructure to maintain |
| Technology flexibility | Debugging complexity |

---

## ADR-002: Event-Driven Communication with Kafka

**Status:** Accepted

**Context:**
Services need to communicate asynchronously. We need reliable event delivery, event ordering, and the ability to handle high throughput.

**Decision:**
Use Apache Kafka as the primary event bus for inter-service communication.

**Alternatives Considered:**
- RabbitMQ: Simpler but lower throughput, no event replay
- NATS: Lightweight but less mature ecosystem
- Redis Pub/Sub: Simple but no persistence guarantee

**Rationale:**
- High throughput (millions of messages/second)
- Event persistence and replay capability
- Ordered events within partitions
- Mature ecosystem with excellent Go client libraries
- Built-in consumer groups for parallel processing

**Consequences:**

| Positive | Negative |
|----------|----------|
| High throughput | Operational complexity (Kafka cluster) |
| Event replay capability | Higher latency than in-process calls |
| Ordered events | Requires careful partition key design |
| Consumer groups | Message ordering only within partition |
| Mature ecosystem | Learning curve for team |

---

## ADR-003: PostgreSQL as Primary Database

**Status:** Accepted

**Context:**
We need a reliable, ACID-compliant database for storing orders and inventory with strong consistency requirements.

**Decision:**
Use PostgreSQL 16 as the primary database for all services.

**Alternatives Considered:**
- MySQL: Good but fewer advanced features
- MongoDB: Flexible schema but weaker consistency
- CockroachDB: Distributed but higher operational cost

**Rationale:**
- ACID compliance for financial transactions
- JSONB support for flexible data (shipping addresses)
- Strong consistency with row-level locking
- Mature and battle-tested
- Excellent Go driver support (pgx)
- Built-in UUID support

**Consequences:**

| Positive | Negative |
|----------|----------|
| Strong ACID guarantees | Vertical scaling limits |
| JSONB for flexible data | Requires careful connection pooling |
| Excellent Go support | Schema migrations required |
| Rich feature set | Storage cost for JSONB |
| Mature ecosystem | Read replicas needed for scaling reads |

---

## ADR-004: Redis for Caching and Distributed Locking

**Status:** Accepted

**Context:**
We need fast data access for frequently read data, session storage, and distributed locking for coordination across service instances.

**Decision:**
Use Redis 7 with Cluster mode for caching, distributed locking, and real-time features.

**Alternatives Considered:**
- Memcached: Simpler but no persistence or data structures
- Hazelcast: JVM-based, doesn't fit Go ecosystem
- etcd: Good for locking but not optimized for caching

**Rationale:**
- Sub-millisecond latency
- Rich data structures (strings, hashes, lists, sets)
- Built-in Pub/Sub for real-time features
- Atomic operations for distributed locking
- TTL support for automatic expiration
- Cluster mode for high availability

**Consequences:**

| Positive | Negative |
|----------|----------|
| Sub-millisecond latency | Memory cost |
| Rich data structures | Cache invalidation complexity |
| Built-in Pub/Sub | Eventual consistency for cached data |
| Atomic operations | Cache stampede risk |
| TTL support | Another infrastructure component |

---

## ADR-005: Traefik as API Gateway

**Status:** Accepted

**Context:**
We need a reverse proxy to handle routing, TLS termination, authentication, and rate limiting for external API traffic.

**Decision:**
Use Traefik as the API Gateway/edge router.

**Alternatives Considered:**
- Custom Go API Gateway: Full control but high development cost
- Kong: Feature-rich but complex configuration
- Nginx: Battle-tested but less Kubernetes-native
- Envoy: Powerful but steep learning curve

**Rationale:**
- Kubernetes-native with automatic service discovery
- Built-in Let's Encrypt ACME support
- Dynamic configuration via Kubernetes CRDs
- Middleware plugins for rate limiting, auth, CORS
- Built-in dashboard for monitoring
- Lower operational cost than custom solution

**Consequences:**

| Positive | Negative |
|----------|----------|
| Kubernetes-native | Less control than custom gateway |
| Auto TLS with Let's Encrypt | Vendor dependency on Traefik |
| Dynamic configuration | Debugging middleware issues |
| Built-in dashboard | Limited custom logic |
| Lower development cost | Learning curve for CRDs |

---

## ADR-006: WebSocket for Real-Time Updates

**Status:** Accepted

**Context:**
We need to push real-time order status updates and inventory changes to connected clients without polling.

**Decision:**
Implement a dedicated WebSocket service using Gorilla WebSocket library.

**Alternatives Considered- SSE (Server-Sent Events): Simpler but unidirectional
- Long Polling: Simple but inefficient
- gRPC Streaming: Powerful but complex client setup
- Socket.IO: Feature-rich but adds dependency

**Rationale:**
- Full-duplex communication
- Browser-native support
- Gorilla WebSocket is battle-tested
- Redis Pub/Sub for broadcasting to multiple instances
- No polling overhead

**Consequences:**

| Positive | Negative |
|----------|----------|
| Real-time bidirectional | Connection management complexity |
| Browser-native | Horizontal scaling requires sticky sessions or pub/sub |
| Efficient | Stateful connections |
| Low latency | Connection handling overhead |
| Battle-tested library | WebSocket load balancing challenges |

---

## ADR-007: Outbox Pattern for Event Publishing

**Status:** Accepted

**Context:**
We need to guarantee that events are published when database state changes. We cannot afford to lose events or publish events without corresponding database changes.

**Decision:**
Implement the Transactional Outbox Pattern for reliable event publishing.

**Alternatives Considered:**
- Direct Kafka publish: Risk of inconsistency
- CDC (Change Data Capture): Complex setup
- Two-phase commit: Performance and availability issues

**Rationale:**
- Guarantees atomicity between database write and event publication
- No distributed transactions required
- Polling publisher is simple to implement
- Proven pattern for event-driven architectures

**Consequences:**

| Positive | Negative |
|----------|----------|
| Guaranteed event publication | Added database writes |
| No distributed transactions | Polling overhead |
| Simple implementation | Eventual consistency |
| Reliable | Outbox table growth |
| battle-tested | Requires cleanup job |

---

## ADR-008: Saga Pattern for Distributed Transactions

**Status:** Accepted

**Context:**
Order creation spans multiple services (order, inventory, notification). We need to maintain consistency without distributed transactions.

**Decision:**
Implement Choreography-based Saga pattern with compensation logic.

**Alternatives Considered:**
- Orchestration Saga: Central coordinator but single point of failure
- Two-phase commit: Poor performance and availability
- Tight coupling: Simple but no fault tolerance

**Rationale:**
- No central coordinator (single point of failure)
- Loose coupling between services
- Compensation logic handles failures
- Each service is autonomous

**Consequences:**

| Positive | Negative |
|----------|----------|
| No single point of failure | Complex compensation logic |
| Loose coupling | Harder to track overall flow |
| Service autonomy | Eventual consistency |
| Fault tolerance | Debugging complexity |
| Scalability | Circular event dependencies possible |

---

## ADR-009: CQRS for Read/Write Separation

**Status:** Accepted

**Context:**
Read and write patterns have different performance requirements. Reads are frequent and need low latency; writes are less frequent but need strong consistency.

**Decision:**
Implement CQRS (Command Query Responsibility Segregation) with separate read and write models.

**Rationale:**
- Optimize read and write independently
- Read replicas for scaling reads
- Redis cache for hot data
- Write model optimized for consistency
- Read model optimized for query performance

**Consequences:**

| Positive | Negative |
|----------|----------|
| Independent optimization | Increased complexity |
| Better read performance | Eventual consistency |
| Scalable reads | Data synchronization challenges |
| Clear separation of concerns | More code to maintain |
| Flexible query optimization | Cache invalidation complexity |

---

## ADR-010: UUID v7 for Primary Keys

**Status:** Accepted

**Context:**
We need unique identifiers that are globally unique, time-sortable, and work well with databases and caches.

**Decision:**
Use UUID v7 (time-ordered) for all primary keys.

**Alternatives Considered:**
- Auto-increment: Not distributed-friendly
- UUID v4: Random, poor index performance
- ULID: Similar to UUID v7 but less standard
- NanoID: Short but not time-ordered

**Rationale:**
- Globally unique without coordination
- Time-ordered for better index performance
- Standard format (RFC 9562)
- Safe to expose externally
- No database sequence contention

**Consequences:**

| Positive | Negative |
|----------|----------|
| Globally unique | 16 bytes storage |
| Time-ordered | Less human-readable |
| No coordination | Requires UUID library |
| Index-friendly | Can't be manually generated |
| External-safe | Order reveals creation time (minor) |

---

## ADR-011: Structured Logging with Zap

**Status:** Accepted

**Context:**
We need high-performance, structured logging that integrates with log aggregation systems.

**Decision:**
Use Uber's Zap library for structured JSON logging.

**Alternatives Considered:**
- Logrus: Popular but slower
- zerolog: Similar performance but less mainstream
- Standard log: No structure

**Rationale:**
- High performance (zero allocation)
- Structured JSON output
- Context-aware logging
- Strong ecosystem support
- Battle-tested at Uber

**Consequences:**

| Positive | Negative |
|----------|----------|
| High performance | Slightly complex API |
| Structured logs | Different API than standard log |
| Context support | Requires initialization |
| JSON output | Learning curve |
| Battle-tested | |

---

## ADR-012: OpenTelemetry for Observability

**Status:** Accepted

**Context:**
We need unified observability across logging, metrics, and tracing in a distributed system.

**Decision:**
Use OpenTelemetry as the observability framework.

**Alternatives Considered:**
- Jaeger only: Tracing only
- Prometheus + Grafana only: Metrics only
- Datadog: Expensive, vendor lock-in
- Custom solution: High development cost

**Rationale:**
- Unified API for logs, metrics, traces
- Vendor-neutral (no lock-in)
- Automatic context propagation
- Growing industry standard
- Excellent Go SDK

**Consequences:**

| Positive | Negative |
|----------|----------|
| Unified observability | Additional complexity |
| Vendor-neutral | Learning curve |
| Automatic context propagation | Overhead of instrumentation |
| Industry standard | Still maturing |
| Go SDK support | |

---

## ADR-013: Database per Service

**Status:** Accepted

**Context:**
Each microservice should own its data to ensure loose coupling and independent deployment.

**Decision:**
Each service has its own database (order_db, inventory_db).

**Alternatives Considered:**
- Shared database: Tight coupling, deployment conflicts
- Schema per service: Partial isolation
- API-only access: Performance overhead

**Rationale:**
- True loose coupling
- Independent schema evolution
- No cross-service locks
- Clear data ownership
- Independent scaling

**Consequences:**

| Positive | Negative |
|----------|----------|
| True loose coupling | No joins across services |
| Independent schema evolution | Data consistency challenges |
| No cross-service locks | More databases to manage |
| Clear ownership | Increased storage cost |
| Independent scaling | Backup complexity |

---

## ADR-014: Kubernetes for Orchestration

**Status:** Accepted

**Context:**
We need a container orchestration platform for deploying, scaling, and managing services.

**Decision:**
Use Kubernetes for container orchestration.

**Alternatives Considered:**
- Docker Swarm: Simpler but less features
- ECS: AWS-specific, vendor lock-in
- Nomad: Simpler but smaller ecosystem
- Manual deployment: Not scalable

**Rationale:**
- Industry standard
- Rich ecosystem (Helm, operators)
- Auto-scaling (HPA, VPA)
- Self-healing capabilities
- Rolling updates and rollbacks
- Strong community support

**Consequences:**

| Positive | Negative |
|----------|----------|
| Industry standard | Steep learning curve |
| Rich ecosystem | Complex configuration |
| Auto-scaling | Resource overhead |
| Self-healing | Requires expertise |
| Rolling updates | YAML management |

---

## ADR-015: gRPC for Internal Communication

**Status:** Accepted

**Context:**
Services need efficient, type-safe internal communication.

**Decision:**
Use gRPC for synchronous service-to-service communication.

**Alternatives Considered:**
- REST: Simpler but less efficient
- Thrift: Similar but less ecosystem
- GraphQL: Flexible but complex

**Rationale:**
- Binary protocol (efficient)
- Strong typing with Protocol Buffers
- Code generation from .proto files
- Built-in streaming support
- Excellent performance

**Consequences:**

| Positive | Negative |
|----------|----------|
| High performance | Not browser-friendly |
| Strong typing | Requires .proto files |
| Code generation | Harder to debug |
| Streaming support | Learning curve |
| Efficient serialization | |

---

## Decision Summary

| Decision | Choice | Key Reason |
|----------|--------|------------|
| Architecture | Microservices | Independent scaling and deployment |
| Event Bus | Kafka | High throughput, persistence |
| Primary DB | PostgreSQL | ACID compliance |
| Cache | Redis | Sub-millisecond latency |
| API Gateway | Traefik | Kubernetes-native |
| Real-time | WebSocket | Bidirectional communication |
| Event Publishing | Outbox Pattern | Guaranteed delivery |
| Distributed TX | Saga Pattern | No 2PC |
| Read/Write | CQRS | Independent optimization |
| Primary Keys | UUID v7 | Time-sortable, unique |
| Logging | Zap | High performance |
| Observability | OpenTelemetry | Unified approach |
| Data Isolation | DB per service | Loose coupling |
| Orchestration | Kubernetes | Industry standard |
| Internal Comms | gRPC | Performance, typing |
