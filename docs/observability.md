# Observability

## Overview

| Pillar | Tool | Purpose |
|--------|------|---------|
| Logging | Zap + Promtail + Loki | Structured logs → log aggregation → query |
| Metrics | Prometheus + Grafana + Exporters | Time-series metrics → dashboards |
| Tracing | OpenTelemetry + Jaeger | Distributed trace context propagation |

**Access Points:**

| Service | URL | Credentials |
|---------|-----|-------------|
| Grafana | http://localhost:3000 | admin / admin |
| Prometheus | http://localhost:9090 | — |
| Loki | http://localhost:3100 | — |
| Jaeger | http://localhost:16686 | — |
| OTel Collector | http://localhost:8888 | — |
| Promtail | http://localhost:9080 | — |

---

## Architecture

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                           OBSERVABILITY STACK                                         │
│                                                                                      │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌───────────┐ │
│  │  Order Svc   │ │Inventory Svc │ │ WebSocket Svc│ │  Auth Svc    │ │  Traefik  │ │
│  │  :8080       │ │  :8081       │ │  :8082       │ │  :8083       │ │  :8080    │ │
│  └──────┬───────┘ └──────┬───────┘ └──────┬───────┘ └──────┬───────┘ └─────┬─────┘ │
│         │ /metrics        │ /metrics        │ /metrics        │ /metrics      │/metrics│
│         └─────────────────┴────────────────┴─────────────────┴───────────────┘        │
│                                       │                                              │
│                                       ▼                                              │
│                           ┌─────────────────────┐                                    │
│                           │     Prometheus      │                                    │
│                           │   (Metrics Store)   │                                    │
│                           │     :9090           │                                    │
│                           └──────────┬──────────┘                                    │
│                                      │                                               │
│         ┌────────────────────────────┼──────────────────────────┐                    │
│         │                            │                          │                    │
│         ▼                            ▼                          ▼                    │
│  ┌─────────────┐            ┌──────────────┐           ┌──────────────┐             │
│  │   Grafana   │            │    Jaeger    │           │     Loki     │             │
│  │  Dashboards │            │   Traces     │           │    Logs      │             │
│  │   :3000     │            │  :16686      │           │   :3100      │             │
│  └──────┬──────┘            └──────────────┘           └──────┬───────┘             │
│         │                                                      │                     │
│         │  log queries                                         │                     │
│         └──────────────────────────────────────────────────────┘                     │
│                                      ▲                                               │
│                                      │                                               │
│                           ┌──────────┴──────────┐                                   │
│                           │      Promtail       │                                   │
│                           │   (Log Collector)   │                                   │
│                           │       :9080         │                                   │
│                           └─────────────────────┘                                   │
│                                                                                      │
│  ┌──────────────────────────────────────────────────────────────────────────────┐    │
│  │                        Exporters                                             │    │
│  │  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────────────┐  │    │
│  │  │  OTel Collector │    │  Kafka Exporter  │    │    Redis Exporter       │  │    │
│  │  │    :8888        │    │     :9308        │    │       :9121             │  │    │
│  │  └─────────────────┘    └─────────────────┘    └─────────────────────────┘  │    │
│  └──────────────────────────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Containers

### Observability Services

| Container | Image | Port | Purpose |
|-----------|-------|------|---------|
| prometheus | prom/prometheus:v2.53.0 | 9090 | Metrics TSDB + scrape + alerts |
| grafana | grafana/grafana:11.1.0 | 3000 | Dashboards + data source UI |
| loki | grafana/loki:3.1.0 | 3100 | Log aggregation |
| jaeger | jaegertracing/all-in-one:1.59 | 16686 | Distributed trace UI |
| otel-collector | otel/opentelemetry-collector-contrib:0.104.0 | 4317/4318/8888/8889 | Telemetry pipeline |
| promtail | grafana/promtail:3.1.0 | 9080 | Log shipping to Loki |

### Exporters

| Container | Image | Port | Purpose |
|-----------|-------|------|---------|
| kafka-exporter | danielqsj/kafka-exporter:v1.8.0 | 9308 | Kafka metrics for Prometheus |
| redis-exporter | oliver006/redis_exporter:v1.61.0 | 9121 | Redis metrics for Prometheus |

---

## Logging

### Flow

```
Go services (zap) → stdout → Docker/Podman logs → Promtail → Loki → Grafana
```

### Structured Log Format

```json
{
  "level": "info",
  "ts": "2026-09-12T10:30:00.123Z",
  "caller": "adapter/httpd/handler.go:42",
  "msg": "order created",
  "service": "order-service",
  "request_id": "req_abc123",
  "order_id": "ord_abc123",
  "customer_id": "usr_xyz789",
  "total": 118.77
}
```

### Log Levels

| Level | Usage |
|-------|-------|
| DEBUG | Development debugging |
| INFO | Normal operations |
| WARN | Recoverable issues |
| ERROR | Failures requiring attention |
| FATAL | Unrecoverable (process exits) |

### Implementation (`pkg/logger/logger.go`)

```go
func New(service string) (*zap.Logger, error) {
    config := zap.NewProductionConfig()
    config.OutputPaths = []string{"stdout"}
    config.EncoderConfig.TimeKey = "ts"
    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

    logger, err := config.Build(
        zap.AddCallerSkip(1),
        zap.Fields(zap.String("service", service)),
    )
    return logger, err
}
```

### Promtail Config

File: `config/promtail/promtail.yml`

Promtail scrapes Docker container logs from `/var/lib/docker/containers/` and ships them to Loki at `http://loki:3100/loki/api/v1/push`. Each service is labeled by job name for filtering in Grafana.

---

## Metrics

### Implementation (`pkg/metrics/`)

**Metrics definitions** — `pkg/metrics/metrics.go`:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_requests_total` | Counter | service, method, path, status | Total HTTP requests |
| `http_request_duration_seconds` | Histogram | service, method, path | Request latency |
| `orders_created_total` | Counter | service | Orders created |
| `orders_cancelled_total` | Counter | service | Orders cancelled |
| `orders_failed_total` | Counter | service, reason | Failed order ops |
| `inventory_reservations_total` | Counter | service, status | Inventory reservations |
| `inventory_releases_total` | Counter | service | Reservation releases |
| `inventory_updates_total` | Counter | service | Stock updates |
| `inventory_failed_total` | Counter | service, reason | Failed inventory ops |
| `ws_connections_active` | Gauge | service | Active WebSocket conns |
| `ws_connections_total` | Counter | service | Total WS connections |
| `ws_messages_sent_total` | Counter | service, channel | WS messages sent |
| `kafka_messages_produced_total` | Counter | service, topic | Kafka messages produced |
| `kafka_messages_consumed_total` | Counter | service, topic | Kafka messages consumed |
| `kafka_processing_duration_seconds` | Histogram | service, topic | Kafka processing latency |

**HTTP middleware** — `pkg/metrics/middleware.go`:

Automatically records `http_requests_total` and `http_request_duration_seconds` for all chi routes.

```go
router := chi.NewRouter()
router.Use(metrics.Middleware("order-service"))
```

### Where Metrics Are Recorded

| Service | File | Metrics |
|---------|------|---------|
| Order | `internal/order/adapter/httpd/handler.go` | orders_created, orders_cancelled, orders_failed |
| Inventory | `internal/inventory/adapter/httpd/handler.go` | inventory_reservations, inventory_releases, inventory_updates, inventory_failed |
| WebSocket | `internal/websocket/handler.go` + `pkg/websocket/hub.go` | ws_connections_active, ws_connections_total, ws_messages_sent |
| All | `pkg/metrics/middleware.go` | http_requests_total, http_request_duration_seconds |

### Prometheus Scraping

File: `config/prometheus/prometheus.yml`

| Job | Target | Metrics Path |
|-----|--------|-------------|
| order-service | order-service:8080 | /metrics |
| inventory-service | inventory-service:8081 | /metrics |
| websocket-service | websocket-service:8082 | /metrics |
| auth-service | auth-service:8083 | /metrics |
| traefik | traefik:8080 | /metrics |
| otel-collector | otel-collector:8888 | /metrics |
| kafka | kafka-exporter:9308 | /metrics |
| redis | redis-exporter:9121 | /metrics |

---

## Distributed Tracing

### Implementation (`pkg/tracing/tracing.go`)

```go
func InitTracer(ctx context.Context, serviceName, collectorURL string) (func(context.Context) error, error) {
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(collectorURL),
        otlptracegrpc.WithInsecure(),
    )

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(5*time.Second)),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sdktrace.AlwaysSample()),
    )

    otel.SetTracerProvider(tp)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))

    return tp.Shutdown, nil
}
```

### Trace Propagation Flow

```
Service A → OTLP gRPC → OTel Collector → Jaeger (storage + UI)
```

All 3 main services (order, inventory, websocket) initialize the tracer on startup. Trace context (`traceparent` header) is propagated via `propagation.TraceContext{}`.

---

## Dashboards

### Provisioned Dashboard

File: `config/grafana/dashboards/overview.json`

**Grafana Datasources** (provisioned):

| Name | Type | URL |
|------|------|-----|
| Prometheus | prometheus | http://prometheus:9090 |
| Loki | loki | http://loki:3100 |
| Jaeger | jaeger | http://jaeger:16686 |
| Tempo | tempo | http://tempo:3200 |

### Dashboard Panels

| Panel | Type | Query |
|-------|------|-------|
| Request Rate (req/s) | timeseries | `sum(rate(http_requests_total[5m])) by (service)` |
| Error Rate (%) | timeseries | `sum(rate(http_requests_total{status=~"5.."}[5m])) by (service) / sum(rate(http_requests_total[5m])) by (service) * 100` |
| Latency P95 (ms) | timeseries | `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service))` |
| Service Up | stat | `up` |
| Orders Created | stat | `sum(increase(orders_created_total[1h]))` |
| Orders Cancelled | stat | `sum(increase(orders_cancelled_total[1h]))` |
| Inventory Reservations | stat | `sum(increase(inventory_reservations_total{status="success"}[1h]))` |
| Inventory Updates | stat | `sum(increase(inventory_updates_total[1h]))` |
| Failed Operations | timeseries | `sum(rate(orders_failed_total[5m])) by (reason)` + `sum(rate(inventory_failed_total[5m])) by (reason)` |
| WebSocket Connections | timeseries | `ws_connections_active` + `rate(ws_connections_total[5m])` |
| Kafka Messages | timeseries | `sum(rate(kafka_messages_produced_total[5m])) by (topic)` + consumed |
| Kafka Processing Duration (P95) | timeseries | `histogram_quantile(0.95, sum(rate(kafka_processing_duration_seconds_bucket[5m])) by (le, topic))` |

---

## Alerting

### Prometheus Alert Rules

File: `config/prometheus/alerts/alerts.yml`

```yaml
groups:
  - name: service-alerts
    rules:
      - alert: ServiceDown
        expr: up == 0
        for: 1m

      - alert: HighErrorRate
        expr: sum(rate(http_requests_total{status=~"5.."}[5m])) by (service)
              / sum(rate(http_requests_total[5m])) by (service) > 0.05
        for: 5m

      - alert: HighLatency
        expr: histogram_quantile(0.95,
               sum(rate(http_request_duration_seconds_bucket[5m])) by (le, service)) > 1
        for: 5m

  - name: kafka-alerts
    rules:
      - alert: KafkaConsumerLagHigh
        expr: kafka_consumer_group_lag > 1000
        for: 10m

  - name: infrastructure-alerts
    rules:
      - alert: RedisDown
        expr: redis_up == 0
        for: 1m

      - alert: PostgresDown
        expr: pg_up == 0
        for: 1m
```

### Alertmanager (Future)

Not yet deployed. When added:

```yaml
# alertmanager.yml
global:
  slack_api_url: 'https://slack.com/api/chat.postMessage'

route:
  group_by: ['alertname', 'service']
  group_wait: 10s
  receiver: 'slack-notifications'

receivers:
  - name: 'slack-notifications'
    slack_configs:
      - channel: '#alerts'
        send_resolved: true
```

---

## Health Checks

All services expose a `/health` endpoint:

```
GET /health → {"status":"ok"}
```

| Service | Endpoint | Port |
|---------|----------|------|
| Order Service | http://localhost:8080/health | 8080 |
| Inventory Service | http://localhost:8081/health | 8081 |
| WebSocket Service | http://localhost:8082/health | 8082 |
| Auth Service | http://localhost:8083/health | 8083 |

---

## Configuration Files

```
config/
├── prometheus/
│   ├── prometheus.yml          # Scrape configs
│   └── alerts/
│       └── alerts.yml          # Alert rules
├── grafana/
│   ├── provisioning/
│   │   ├── datasources/
│   │   │   └── datasources.yml # Prometheus, Loki, Jaeger, Tempo
│   │   └── dashboards/
│   │       └── dashboards.yml  # Dashboard provider
│   └── dashboards/
│       └── overview.json       # Main dashboard (12 panels)
├── loki/
│   └── loki.yml                # Loki config (filesystem storage)
├── otel/
│   └── otel-collector.yml      # OTEL Collector config
└── promtail/
    └── promtail.yml            # Log collection config
```

---

## Future Improvements

- [ ] Add Alertmanager container for Slack/PagerDuty notifications
- [ ] Add kafka-exporter and redis-exporter to K8s manifests
- [ ] Add custom business metrics (per-warehouse, per-product)
- [ ] Add trace context propagation to Kafka producer/consumer
- [ ] Add Loki log-to-trace correlation in Grafana
- [ ] Add Tempo for trace storage (currently Jaeger in-memory)
- [ ] K8s HPA-aware alerts
- [ ] Persistent storage for Prometheus/Loki/Jaeger in production
