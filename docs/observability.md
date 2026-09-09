# Observability

## Overview

| Pillar | Tool | Purpose |
|--------|------|---------|
| Logging | Zap + Loki | Structured logs |
| Metrics | Prometheus + Grafana | Time-series data |
| Tracing | OpenTelemetry + Jaeger | Distributed traces |
| Alerting | Alertmanager + PagerDuty | Incident response |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                          OBSERVABILITY STACK                                     │
│                                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │  Order Svc   │  │Inventory Svc │  │ WebSocket Svc│  │   Traefik    │       │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘       │
│         │                  │                  │                  │               │
│         └──────────────────┼──────────────────┼──────────────────┘               │
│                            │                  │                                  │
│         ┌──────────────────┼──────────────────┼──────────────────┐               │
│         ▼                  ▼                  ▼                  ▼               │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                         OpenTelemetry Collector                         │   │
│  └─────────┬────────────────────┬────────────────────┬─────────────────────┘   │
│            │                    │                    │                          │
│            ▼                    ▼                    ▼                          │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐                     │
│  │     Loki     │    │  Prometheus  │    │    Jaeger    │                     │
│  │   (Logs)     │    │  (Metrics)   │    │  (Traces)    │                     │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘                     │
│         │                   │                   │                              │
│         ▼                   ▼                   ▼                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐                     │
│  │    Grafana   │◀──▶│ Alertmanager │    │              │                     │
│  │  (Dashboard) │    │  (Alerts)    │    │              │                     │
│  └──────────────┘    └──────────────┘    └──────────────┘                     │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Logging

### Structured Log Format

```json
{
  "level": "info",
  "ts": "2024-01-15T10:30:00.123Z",
  "caller": "order/service.go:42",
  "msg": "order created",
  "service": "order-service",
  "request_id": "req_abc123",
  "trace_id": "trace_xyz789",
  "span_id": "span_001",
  "order_id": "ord_abc123",
  "customer_id": "usr_xyz789",
  "total": 118.77,
  "duration_ms": 45
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

### Logger Implementation

```go
func NewLogger(service string) (*zap.Logger, error) {
    config := zap.NewProductionConfig()
    config.OutputPaths = []string{"stdout"}
    config.EncoderConfig.TimeKey = "ts"
    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

    // Add service name to all logs
    logger, err := config.Build(
        zap.AddCallerSkip(1),
        zap.Fields(zap.String("service", service)),
    )
    if err != nil {
        return nil, err
    }

    return logger, nil
}

// WithContext extracts trace info from context
func WithContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
    span := trace.SpanFromContext(ctx)
    if span.SpanContext().IsValid() {
        logger = logger.With(
            zap.String("trace_id", span.SpanContext().TraceID().String()),
            zap.String("span_id", span.SpanContext().SpanID().String()),
        )
    }

    if requestID, ok := ctx.Value("request_id").(string); ok {
        logger = logger.With(zap.String("request_id", requestID))
    }

    return logger
}
```

### Log Shipping

```
App → stdout → Fluentd → Loki → Grafana
```

---

## Metrics

### Key Metrics by Service

#### Order Service

| Metric | Type | Description |
|--------|------|-------------|
| `orders_created_total` | Counter | Total orders created |
| `orders_created_amount` | Histogram | Order amounts |
| `orders_duration_ms` | Histogram | Order creation latency |
| `orders_failed_total` | Counter | Failed order creations |
| `orders_by_status` | Gauge | Orders per status |

#### Inventory Service

| Metric | Type | Description |
|--------|------|-------------|
| `inventory_reservations_total` | Counter | Total reservations |
| `inventory_available` | Gauge | Available stock |
| `inventory_reserved` | Gauge | Reserved stock |
| `inventory_operations_duration_ms` | Histogram | Operation latency |
| `inventory_low_stock` | Gauge | Low stock products |

#### System Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `http_requests_total` | Counter | Total HTTP requests |
| `http_request_duration_ms` | Histogram | Request latency |
| `http_requests_in_flight` | Gauge | Concurrent requests |
| `kafka_messages_produced_total` | Counter | Messages produced |
| `kafka_messages_consumed_total` | Counter | Messages consumed |
| `kafka_consumer_lag` | Gauge | Consumer lag |
| `redis_operations_total` | Counter | Redis operations |
| `redis_hit_ratio` | Gauge | Cache hit ratio |
| `db_connections_active` | Gauge | Active DB connections |
| `db_query_duration_ms` | Histogram | Query latency |

### Prometheus Configuration

```yaml
# prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'order-service'
    static_configs:
      - targets: ['order-service:9090']
    metrics_path: '/metrics'

  - job_name: 'inventory-service'
    static_configs:
      - targets: ['inventory-service:9091']
    metrics_path: '/metrics'

  - job_name: 'websocket-service'
    static_configs:
      - targets: ['websocket-service:9092']
    metrics_path: '/metrics'

  - job_name: 'traefik'
    static_configs:
      - targets: ['traefik:8082']
    metrics_path: '/metrics'
```

### Custom Metrics Registration

```go
func RegisterMetrics(service string) {
    prometheus.MustRegister(
        prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name:        fmt.Sprintf("%s_requests_total", service),
                Help:        "Total requests",
                ConstLabels: prometheus.Labels{"service": service},
            },
            []string{"method", "path", "status"},
        ),
        prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:        fmt.Sprintf("%s_request_duration_ms", service),
                Help:        "Request duration",
                Buckets:     []float64{10, 50, 100, 250, 500, 1000, 2500, 5000},
                ConstLabels: prometheus.Labels{"service": service},
            },
            []string{"method", "path"},
        ),
    )
}
```

---

## Distributed Tracing

### Trace Propagation

```go
func InitTracer(serviceName string) (*sdktrace.TracerProvider, error) {
    exporter, err := otlptracehttp.New(ctx,
        otlptracehttp.WithEndpoint("jaeger:4318"),
        otlptracehttp.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName(serviceName),
            semconv.ServiceVersion("1.0.0"),
        )),
        sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1)), // 10% sampling
    )

    otel.SetTracerProvider(tp)
    return tp, nil
}
```

### Span Creation

```go
func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*Order, error) {
    ctx, span := tracer.Start(ctx, "OrderService.CreateOrder",
        trace.WithAttributes(
            attribute.Int("order.items_count", len(req.Items)),
        ),
    )
    defer span.End()

    // Create order
    order, err := s.repo.CreateOrder(ctx, req)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, err
    }

    span.SetAttributes(
        attribute.String("order.id", order.ID),
        attribute.Float64("order.total", order.TotalAmount),
    )

    return order, nil
}
```

### Trace Context Headers

```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
tracestate: congo=t61rcWkgMzE
```

---

## Dashboards

### Order Service Dashboard

```json
{
  "panels": [
    {
      "title": "Orders Per Minute",
      "type": "graph",
      "query": "sum(rate(orders_created_total[5m])) * 60"
    },
    {
      "title": "Order Creation Latency",
      "type": "heatmap",
      "query": "histogram_quantile(0.99, rate(orders_duration_ms_bucket[5m]))"
    },
    {
      "title": "Orders by Status",
      "type": "pie",
      "query": "orders_by_status"
    },
    {
      "title": "Failed Orders",
      "type": "stat",
      "query": "sum(rate(orders_failed_total[5m])) * 60"
    }
  ]
}
```

### Inventory Service Dashboard

```json
{
  "panels": [
    {
      "title": "Available Stock",
      "type": "graph",
      "query": "sum(inventory_available) by (product_id)"
    },
    {
      "title": "Reservation Rate",
      "type": "graph",
      "query": "sum(rate(inventory_reservations_total[5m])) * 60"
    },
    {
      "title": "Low Stock Products",
      "type": "table",
      "query": "inventory_low_stock"
    },
    {
      "title": "Stock Movements",
      "type": "graph",
      "query": "sum(rate(inventory_movements_total[5m])) by (movement_type)"
    }
  ]
}
```

---

## Alerting

### Alert Rules

```yaml
# alerts.yml
groups:
  - name: order-service
    rules:
      - alert: HighErrorRate
        expr: sum(rate(app_errors_total{service="order-service"}[5m])) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate in Order Service"

      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(http_request_duration_ms_bucket{service="order-service"}[5m])) > 1000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency in Order Service"

  - name: inventory-service
    rules:
      - alert: LowStock
        expr: inventory_available < 10
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Low stock for product {{ $labels.product_id }}"

      - alert: HighConsumerLag
        expr: kafka_consumer_lag{service="inventory-service"} > 1000
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High Kafka consumer lag"
```

### Alertmanager Configuration

```yaml
# alertmanager.yml
global:
  slack_api_url: 'https://slack.com/api/chat.postMessage'

route:
  group_by: ['alertname', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'slack-notifications'

receivers:
  - name: 'slack-notifications'
    slack_configs:
      - channel: '#alerts'
        send_resolved: true
        title: '{{ .GroupLabels.alertname }}'
        text: '{{ .CommonAnnotations.summary }}'
```

---

## Health Checks

### Health Endpoint

```go
type HealthStatus struct {
    Status    string            `json:"status"`
    Version   string            `json:"version"`
    Uptime    string            `json:"uptime"`
    Checks    map[string]string `json:"checks"`
}

func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
    status := HealthStatus{
        Status:  "healthy",
        Version: version,
        Uptime:  time.Since(startTime).String(),
        Checks:  make(map[string]string),
    }

    // Check database
    if err := h.db.PingContext(r.Context()); err != nil {
        status.Status = "degraded"
        status.Checks["database"] = err.Error()
    } else {
        status.Checks["database"] = "ok"
    }

    // Check Redis
    if err := h.redis.Ping(r.Context()).Err(); err != nil {
        status.Status = "degraded"
        status.Checks["redis"] = err.Error()
    } else {
        status.Checks["redis"] = "ok"
    }

    // Check Kafka
    if err := h.kafka.Ping(r.Context()); err != nil {
        status.Status = "degraded"
        status.Checks["kafka"] = err.Error()
    } else {
        status.Checks["kafka"] = "ok"
    }

    httpStatus := http.StatusOK
    if status.Status != "healthy" {
        httpStatus = http.StatusServiceUnavailable
    }

    respondJSON(w, httpStatus, status)
}
```

### Readiness vs Liveness

```
/health/live   → Is process alive? (Kubernetes liveness)
/health/ready  → Is service ready? (Kubernetes readiness)
/health/start  → Has service started? (Kubernetes startup)
```
