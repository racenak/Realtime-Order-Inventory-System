# Deployment Guide

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Podman | 4.x+ | Container runtime (rootless) |
| Podman Compose | 1.x+ | Multi-container orchestration |
| Go | 1.25+ | Building services |

---

## Quick Start

```bash
# Start full stack
podman compose up -d --build

# Verify
podman compose ps

# Access
# Traefik:    http://localhost:8088
# Dashboard:  http://localhost:8090
# Grafana:    http://localhost:3000 (admin/admin)
# Tempo:      http://localhost:3200
# Prometheus: http://localhost:9090
```

---

## Services

| Service | Container | Port | Health Check |
|---------|-----------|------|-------------|
| Traefik | traefik | 8088 (HTTP), 8090 (Dashboard) | `http://traefik:8080/ping` |
| Order Service | order-service | 8080 | `http://order-service:8080/health` |
| Inventory Service | inventory-service | 8081 | `http://inventory-service:8081/health` |
| WebSocket Service | websocket-service | 8082 | `http://websocket-service:8082/health` |
| Auth Service | auth-service | 8083 | `http://auth-service:8083/validate` |
| PostgreSQL (Order) | order-db | 5432 | `pg_isready -U postgres` |
| PostgreSQL (Inventory) | inventory-db | 5433 | `pg_isready -U postgres` |
| Redis | redis | 6379 | `redis-cli ping` |
| Kafka | kafka | 9092 | `nc -z kafka 9092` |
| Kafka Init | kafka-init | — | Creates topics on startup |
| Prometheus | prometheus | 9090 | `http://prometheus:9090/-/healthy` |
| Grafana | grafana | 3000 | `http://grafana:3000/api/health` |
| Loki | loki | 3100 | `http://loki:3100/ready` |
| Tempo | tempo | 3200 | `http://tempo:3200/` |
| OTel Collector | otel-collector | 8888 | `http://otel-collector:8888/metrics` |
| Promtail | promtail | 9080 | `http://promtail:9080/` |
| kafka-exporter | kafka-exporter | 9308 | `http://kafka-exporter:9308/metrics` |
| redis-exporter | redis-exporter | 9121 | `http://redis-exporter:9121/metrics` |
| postgres-exporter (order) | postgres-exporter | 9187 | `http://postgres-exporter:9187/metrics` |
| postgres-exporter (inventory) | postgres-exporter | 9188 | `http://postgres-exporter:9188/metrics` |

---

## Environment Variables

### Order Service

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | localhost | PostgreSQL host |
| `DB_PORT` | 5432 | PostgreSQL port |
| `DB_USER` | postgres | PostgreSQL user |
| `DB_PASSWORD` | postgres | PostgreSQL password |
| `DB_NAME` | order_db | Database name |
| `KAFKA_BROKER` | localhost:9092 | Kafka broker address |
| `REDIS_ADDR` | localhost:6379 | Redis address |
| `OTEL_ENDPOINT` | localhost:4317 | OTLP gRPC endpoint |

### Inventory Service

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | localhost | PostgreSQL host |
| `DB_PORT` | 5433 | PostgreSQL port |
| `DB_USER` | postgres | PostgreSQL user |
| `DB_PASSWORD` | postgres | PostgreSQL password |
| `DB_NAME` | inventory_db | Database name |
| `KAFKA_BROKER` | localhost:9092 | Kafka broker address |
| `REDIS_ADDR` | localhost:6379 | Redis address |
| `OTEL_ENDPOINT` | localhost:4317 | OTLP gRPC endpoint |

### WebSocket Service

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDR` | localhost:6379 | Redis address |
| `OTEL_ENDPOINT` | localhost:4317 | OTLP gRPC endpoint |

### Auth Service

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | changeme | JWT signing secret |
| `JWT_ISSUER` | order-inventory-system | JWT issuer claim |
| `JWT_AUDIENCE` | order-inventory-api | JWT audience claim |
| `OTEL_ENDPOINT` | localhost:4317 | OTLP gRPC endpoint |

---

## Podman-Specific Requirements

### Volume Permissions

Rootless Podman runs as uid `1000` inside containers. For writable volumes:

```yaml
volumes:
  - grafana-data:/var/lib/grafana:z  # SELinux label
```

Grafana and other services must use `user: root` + `:z` labels for volume mounts.

### PostgreSQL 18

PostgreSQL 18 uses `/var/lib/postgresql` as data directory (not `/var/lib/postgresql/data`):

```yaml
volumes:
  - order-data:/var/lib/postgresql
```

### Kafka Advertised Listener

Kafka must advertise `PLAINTEXT://kafka:9092` for inter-container connectivity:

```yaml
KAFKA_LISTENERS: PLAINTEXT://0.0.0.0:9092
KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
```

### SQL Comments

DB init scripts must use `--` comments (not `#`) for psql compatibility:

```sql
-- Correct
CREATE INDEX idx_orders_customer_id ON orders(customer_id);

-- Wrong (psql ignores # comments but they can cause issues)
# CREATE INDEX idx_orders_customer_id ON orders(customer_id);
```

---

## Traefik Configuration

Uses **file provider** (not Docker provider) for Podman compatibility.

- Config baked into custom Dockerfile
- `traefik.yml` (static config) + `dynamic.yml` (routes/middleware)
- `--providers.file.watch=false` avoids file watching issues

### Routes

| Path | Service | Strip Prefix |
|------|---------|-------------|
| `/api/orders` | order-service | `/api/orders` |
| `/api/inventory` | inventory-service | `/api/inventory` |
| `/ws` | websocket-service | `/ws` |

---

## Building Services

```bash
# Build all services
go build -o bin/order-service ./cmd/order-service
go build -o bin/inventory-service ./cmd/inventory-service
go build -o bin/websocket-service ./cmd/websocket-service
go build -o bin/auth-service ./cmd/auth-service

# Or via Docker/Podman
podman compose build
```

---

## Useful Commands

```bash
# View logs
podman compose logs -f order-service
podman compose logs -f inventory-service

# Restart service
podman compose restart order-service

# Stop everything
podman compose down

# Stop and remove volumes
podman compose down -v

# Rebuild specific service
podman compose build order-service

# Check Traefik config
curl http://localhost:8090/api/http/routers | jq .

# Prometheus targets
curl http://localhost:9090/api/v1/targets | jq .

# Grafana datasources
curl -u admin:admin http://localhost:3000/api/datasources | jq .
```

---

## Troubleshooting

### Grafana won't start

Rootless Podman uid mismatch. Add `user: root` and `:z` labels to volume mounts:

```yaml
grafana:
  user: root
  volumes:
    - grafana-data:/var/lib/grafana:z
```

### Kafka won't start

Ensure advertised listener is `kafka:9092` (not localhost):

```yaml
KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
```

### PostgreSQL 18 won't start

Must mount to `/var/lib/postgresql` (not `/var/lib/postgresql/data`):

```yaml
volumes:
  - order-data:/var/lib/postgresql
```

### Traefik 404

- Check dynamic.yml is baked into image
- Verify `--providers.file.watch=false`
- Check container logs: `podman compose logs traefik`
