# Observability

## Stack

| Component | Port | Purpose |
|-----------|------|---------|
| Prometheus | 9090 | Metrics TSDB + scraping |
| Grafana | 3000 | 9 dashboards |
| Loki | 3100 | Log aggregation |
| Tempo | 3200 | Distributed trace UI |
| OTel Collector | 8888 | Telemetry pipeline |
| Promtail | 9080 | Log shipping |
| kafka-exporter | 9308 | Kafka metrics |
| redis-exporter | 9121 | Redis metrics |
| postgres-exporter | 9187 | Order DB metrics |
| postgres-exporter | 9188 | Inventory DB metrics |

---

## Metrics

### Custom Business Metrics (16)

**Order Metrics:**

| Metric | Type | Description |
|--------|------|-------------|
| `orders_created_total` | Counter | Total orders created |
| `orders_completed_total` | Counter | Total orders completed |
| `orders_cancelled_total` | Counter | Total orders cancelled |
| `order_processing_duration_seconds` | Histogram | Order processing duration |

**Inventory Metrics:**

| Metric | Type | Description |
|--------|------|-------------|
| `inventory_reservations_total` | Counter | Total reservations |
| `inventory_releases_total` | Counter | Total releases |
| `inventory_movements_total` | Counter | Total movements |
| `inventory_stock_level` | Gauge | Current stock level |

**WebSocket Metrics:**

| Metric | Type | Description |
|--------|------|-------------|
| `websocket_connections_active` | Gauge | Active WS connections |
| `websocket_messages_sent_total` | Counter | Total WS messages sent |

**Kafka Metrics:**

| Metric | Type | Description |
|--------|------|-------------|
| `kafka_messages_published_total` | Counter | Total messages published |
| `kafka_messages_consumed_total` | Counter | Total messages consumed |
| `kafka_publish_errors_total` | Counter | Total publish errors |
| `kafka_dlq_messages_total` | Counter | Total DLQ messages |

**HTTP Metrics:**

| Metric | Type | Description |
|--------|------|-------------|
| `http_requests_total` | Counter | Total HTTP requests |
| `http_request_duration_seconds` | Histogram | HTTP request duration |

### Infrastructure Metrics

| Exporter | Metrics |
|----------|---------|
| postgres-exporter | `pg_stat_activity`, `pg_stat_database`, `pg_stat_user_tables` |
| redis-exporter | `redis_connected_clients`, `redis_used_memory`, `redis_commands_processed` |
| kafka-exporter | `kafka_brokers`, `kafka_topic_partitions`, `kafka_consumer_group_lag` |
| Prometheus self | `prometheus_tsdb_head_samples_appended_total`, `prometheus_config_last_reload_success_timestamp_seconds` |

---

## Tracing

### OpenTelemetry

All Go services instrument OTEL tracer:

```go
// pkg/tracing/tracing.go
func Init(serviceName, endpoint string) (*sdktrace.TracerProvider, error) {
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(endpoint),
        otlptracegrpc.WithInsecure(),
    )
    // ...
}
```

**Flow**:
```
Service → OTLP gRPC → OTel Collector → Tempo (traces)
                                       → Prometheus (metrics)
```

### Span Naming

| Operation | Span Name |
|-----------|-----------|
| HTTP request | `HTTP {METHOD} {PATH}` |
| DB query | `SQL {TABLE}` |
| Kafka publish | `Kafka publish {TOPIC}` |
| Kafka consume | `Kafka consume {TOPIC}` |
| Redis get | `Redis GET {KEY}` |
| Redis set | `Redis SET {KEY}` |

---

## Logging

### Structured JSON Logging (Zap)

```go
logger := zap.New(zapcore.NewCore(
    zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
    os.Stdout,
    zapcore.InfoLevel,
))
```

**Log Format:**
```json
{
  "level": "info",
  "ts": "2026-09-09T10:30:00Z",
  "caller": "handler/handler.go:42",
  "msg": "order created",
  "order_id": "01HXYZ...",
  "customer_id": "cust_01HXYZ...",
  "total": 129.99
}
```

### Log Pipeline

```
Container stdout → Promtail → Loki → Grafana (Explore)
```

### Log Levels

| Level | Usage |
|-------|-------|
| ERROR | Unexpected failures, DLQ publishes |
| WARN | Cache misses, retryable errors |
| INFO | Request processing, event publishing |
| DEBUG | SQL queries, cache operations |

---

## Dashboards

### Grafana (9 dashboards)

| Dashboard | Datasource | Focus |
|-----------|------------|-------|
| System Overview | Prometheus | Requests, errors, latency p50/p95/p99 |
| Order Service | Prometheus | Orders created/completed/cancelled, processing duration |
| Inventory Service | Prometheus | Reservations, releases, stock levels |
| WebSocket Service | Prometheus | Connections, messages sent |
| Auth Service | Prometheus | JWT validations, errors |
| Kafka | Prometheus | Consumer group lag, throughput, partitions |
| Redis | Prometheus | Memory, connections, hit rate |
| Infrastructure | Prometheus | CPU, memory, network across all services |
| API Performance | Prometheus | Endpoint-level latency, error rates |

---

## Health Checks

### Traefik

```yaml
healthcheck:
  test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/ping"]
  interval: 10s
  timeout: 5s
  retries: 3
```

### Application Services

```yaml
healthcheck:
  test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/health"]
  interval: 10s
  timeout: 5s
  retries: 3
```

### PostgreSQL

```yaml
healthcheck:
  test: ["CMD-SHELL", "pg_isready -U postgres"]
  interval: 10s
  timeout: 5s
  retries: 5
```

### Redis

```yaml
healthcheck:
  test: ["CMD", "redis-cli", "ping"]
  interval: 10s
  timeout: 5s
  retries: 5
```

### Kafka

```yaml
healthcheck:
  test: ["CMD-SHELL", "nc -z localhost 9092"]
  interval: 10s
  timeout: 5s
  retries: 5
```

---

## What's Implemented

- [x] Full observability stack (Prometheus, Grafana, Loki, Tempo, OTel Collector, Promtail)
- [x] 9 Grafana dashboards
- [x] 16 custom business metrics (pkg/metrics/)
- [x] OTEL tracing with OTLP gRPC exporter
- [x] Structured JSON logging (Zap)
- [x] Log shipping via Promtail → Loki
- [x] Per-service exporters (kafka, redis, postgres)
- [x] Health checks for all services
- [x] HTTP metrics middleware (chi)
