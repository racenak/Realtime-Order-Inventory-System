# Deployment

## Overview

| Component | Technology |
|-----------|------------|
| Container Runtime | Docker |
| Orchestration | Kubernetes |
| CI/CD | GitHub Actions |
| Infrastructure | Terraform |
| Monitoring | Prometheus + Grafana |

---

## Deployment Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           DEPLOYMENT ARCHITECTURE                                │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                          CI/CD PIPELINE                                  │   │
│  │                                                                         │   │
│  │  ┌──────┐   ┌──────┐   ┌──────┐   ┌──────┐   ┌──────┐               │   │
│  │  │ Build│──▶│ Test │──▶│ Scan │──▶│Push  │──▶│Deploy│               │   │
│  │  └──────┘   └──────┘   └──────┘   └──────┘   └──────┘               │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                     │                                           │
│                                     ▼                                           │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                         KUBERNETES CLUSTER                               │   │
│  │                                                                         │   │
│  │  ┌─────────────────────────────────────────────────────────────────┐   │   │
│  │  │                      INGRESS (Traefik)                          │   │   │
│  │  └─────────────────────────────┬───────────────────────────────────┘   │   │
│  │                                │                                        │   │
│  │  ┌─────────────────────────────┼───────────────────────────────────┐   │   │
│  │  │                     APPLICATION LAYER                            │   │   │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────┐                     │   │   │
│  │  │  │  Order   │  │Inventory │  │ WebSocket│                     │   │   │
│  │  │  │ Service  │  │ Service  │  │ Service  │                     │   │   │
│  │  │  │ (3 pods) │  │ (3 pods) │  │ (3 pods) │                     │   │   │
│  │  │  └──────────┘  └──────────┘  └──────────┘                     │   │   │
│  │  └─────────────────────────────────────────────────────────────────┘   │   │
│  │                                │                                        │   │
│  │  ┌─────────────────────────────┼───────────────────────────────────┐   │   │
│  │  │                     DATA LAYER                                   │   │   │
│  │  │  ┌──────────┐  ┌──────────┐  ┌──────────┐                     │   │   │
│  │  │  │PostgreSQL│  │  Redis   │  │  Kafka   │                     │   │   │
│  │  │  │ (HA)     │  │ (Cluster)│  │ (Cluster)│                     │   │   │
│  │  │  └──────────┘  └──────────┘  └──────────┘                     │   │   │
│  │  └─────────────────────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Docker

### Multi-Stage Dockerfile

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files first (cache layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/bin/order-service \
    ./cmd/order-service

# Final stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/order-service .
COPY --from=builder /app/configs ./configs

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -S appuser -u 1001 -G appgroup

USER appuser

EXPOSE 9001

ENTRYPOINT ["./order-service"]
```

### Docker Compose (Development)

```yaml
version: '3.8'

services:
  order-service:
    build:
      context: .
      dockerfile: cmd/order-service/Dockerfile
    ports:
      - "9001:9001"
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=postgres
      - DB_PASSWORD=postgres
      - DB_NAME=order_db
      - REDIS_HOST=redis
      - KAFKA_BROKERS=kafka:9092
    depends_on:
      - postgres
      - redis
      - kafka

  inventory-service:
    build:
      context: .
      dockerfile: cmd/inventory-service/Dockerfile
    ports:
      - "9002:9002"
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=postgres
      - DB_PASSWORD=postgres
      - DB_NAME=inventory_db
      - REDIS_HOST=redis
      - KAFKA_BROKERS=kafka:9092
    depends_on:
      - postgres
      - redis
      - kafka

  websocket-service:
    build:
      context: .
      dockerfile: cmd/websocket-service/Dockerfile
    ports:
      - "8082:8082"
    environment:
      - REDIS_HOST=redis
      - KAFKA_BROKERS=kafka:9092
    depends_on:
      - redis
      - kafka

  auth-service:
    build:
      context: .
      dockerfile: cmd/auth-service/Dockerfile
    ports:
      - "8083:8083"
    environment:
      - JWT_SECRET=${JWT_SECRET:-changeme}
      - JWT_ISSUER=${JWT_ISSUER:-order-inventory-system}
      - JWT_AUDIENCE=${JWT_AUDIENCE:-order-inventory-api}
      - HTTP_PORT=8083

  postgres:
    image: postgres:16-alpine
    ports:
      - "5432:5432"
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  kafka:
    image: confluentinc/cp-kafka:7.6.0
    ports:
      - "9092:9092"
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: PLAINTEXT:PLAINTEXT
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092

volumes:
  postgres_data:
```

---

## Kubernetes

### Namespace

```yaml
# namespace.yml
apiVersion: v1
kind: Namespace
metadata:
  name: order-inventory
  labels:
    app.kubernetes.io/part-of: order-inventory-system
```

### Order Service Deployment

```yaml
# deployments/order-service.yml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order-service
  namespace: order-inventory
  labels:
    app: order-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: order-service
  template:
    metadata:
      labels:
        app: order-service
    spec:
      containers:
        - name: order-service
          image: registry.example.com/order-service:latest
          ports:
            - containerPort: 9001
          env:
            - name: DB_HOST
              valueFrom:
                secretKeyRef:
                  name: order-service-secrets
                  key: db-host
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: order-service-secrets
                  key: db-password
          resources:
            requests:
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health/live
              port: 9001
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 9001
            initialDelaySeconds: 5
            periodSeconds: 5
          startupProbe:
            httpGet:
              path: /health/start
              port: 9001
            failureThreshold: 30
            periodSeconds: 10
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app: order-service
                topologyKey: kubernetes.io/hostname
```

### Service

```yaml
# services/order-service.yml
apiVersion: v1
kind: Service
metadata:
  name: order-service
  namespace: order-inventory
spec:
  selector:
    app: order-service
  ports:
    - port: 9001
      targetPort: 9001
  type: ClusterIP
```

### Horizontal Pod Autoscaler

```yaml
# hpa/order-service.yml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: order-service
  namespace: order-inventory
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: order-service
  minReplicas: 3
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

### IngressRoute (Traefik)

```yaml
# ingress/order-service.yml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: order-service
  namespace: order-inventory
spec:
  entryPoints:
    - websecure
  routes:
    - match: Host(`api.example.com`) && PathPrefix(`/api/orders`)
      kind: Rule
      services:
        - name: order-service
          port: 9001
      middlewares:
        - name: rate-limit
        - name: jwt-auth
        - name: cors
  tls:
    certResolver: letsencrypt
```

### Network Policies

```yaml
# network-policies/order-service.yml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: order-service
  namespace: order-inventory
spec:
  podSelector:
    matchLabels:
      app: order-service
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: traefik
      ports:
        - port: 9001
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: postgres
      ports:
        - port: 5432
    - to:
        - podSelector:
            matchLabels:
              app: redis
      ports:
        - port: 6379
    - to:
        - podSelector:
            matchLabels:
              app: kafka
      ports:
        - port: 9092
```

---

## CI/CD Pipeline

### GitHub Actions Workflow

```yaml
# .github/workflows/deploy.yml
name: Deploy

on:
  push:
    branches: [main]

env:
  REGISTRY: ghcr.io
  IMAGE_PREFIX: ${{ github.repository }}

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run Tests
        run: make test

      - name: Run Lint
        run: make lint

  build-and-push:
    needs: build-and-test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and Push Order Service
        uses: docker/build-push-action@v5
        with:
          context: .
          file: cmd/order-service/Dockerfile
          push: true
          tags: |
            ${{ env.REGISTRY }}/${{ env.IMAGE_PREFIX }}/order-service:${{ github.sha }}
            ${{ env.REGISTRY }}/${{ env.IMAGE_PREFIX }}/order-service:latest

      - name: Build and Push Inventory Service
        uses: docker/build-push-action@v5
        with:
          context: .
          file: cmd/inventory-service/Dockerfile
          push: true
          tags: |
            ${{ env.REGISTRY }}/${{ env.IMAGE_PREFIX }}/inventory-service:${{ github.sha }}
            ${{ env.REGISTRY }}/${{ env.IMAGE_PREFIX }}/inventory-service:latest

  deploy:
    needs: build-and-push
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup kubectl
        uses: azure/setup-kubectl@v3
        with:
          version: 'v1.28.0'

      - name: Set Kubernetes context
        uses: azure/k8s-set-context@v3
        with:
          kubeconfig: ${{ secrets.KUBE_CONFIG }}

      - name: Deploy to Kubernetes
        run: |
          kubectl set image deployment/order-service \
            order-service=${{ env.REGISTRY }}/${{ env.IMAGE_PREFIX }}/order-service:${{ github.sha }} \
            -n order-inventory

          kubectl set image deployment/inventory-service \
            inventory-service=${{ env.REGISTRY }}/${{ env.IMAGE_PREFIX }}/inventory-service:${{ github.sha }} \
            -n order-inventory

          kubectl rollout status deployment/order-service -n order-inventory
          kubectl rollout status deployment/inventory-service -n order-inventory
```

---

## Rollback Strategy

### Automatic Rollback

```yaml
# Deployment with rolling update strategy
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order-service
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    spec:
      containers:
        - name: order-service
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 9001
            failureThreshold: 3
            periodSeconds: 10
```

### Manual Rollback Commands

```bash
# Check rollout history
kubectl rollout history deployment/order-service -n order-inventory

# Rollback to previous version
kubectl rollout undo deployment/order-service -n order-inventory

# Rollback to specific revision
kubectl rollout undo deployment/order-service --to-revision=2 -n order-inventory

# Check rollout status
kubectl rollout status deployment/order-service -n order-inventory
```

---

## Environment Promotion

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Develop   │────▶│    Stage    │────▶│ Pre-Prod    │────▶│ Production  │
│             │     │             │     │             │     │             │
│ Auto-deploy│     │ Auto-deploy │     │ Manual      │     │ Manual      │
│ on push     │     │ on tag      │     │ approval    │     │ approval    │
└─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
```

### Environment Configuration

```yaml
# environments/production.yml
apiVersion: v1
kind: ConfigMap
metadata:
  name: order-service-config
  namespace: order-inventory
data:
  LOG_LEVEL: "info"
  DB_HOST: "postgres-primary.database.svc.cluster.local"
  REDIS_HOST: "redis-cluster.cache.svc.cluster.local"
  KAFKA_BROKERS: "kafka-0.kafka.messaging.svc.cluster.local:9092,kafka-1.kafka.messaging.svc.cluster.local:9092"
```

---

## Secrets Management

### Kubernetes Secrets

```yaml
# secrets/order-service.yml
apiVersion: v1
kind: Secret
metadata:
  name: order-service-secrets
  namespace: order-inventory
type: Opaque
stringData:
  db-host: "postgres-primary.database.svc.cluster.local"
  db-password: "encrypted-password"
  jwt-secret: "encrypted-jwt-secret"
```

### External Secrets Operator

```yaml
# external-secrets/order-service.yml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: order-service-secrets
  namespace: order-inventory
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: order-service-secrets
  data:
    - secretKey: db-password
      remoteRef:
        key: order-inventory/db-password
    - secretKey: jwt-secret
      remoteRef:
        key: order-inventory/jwt-secret
```

---

## Blue-Green Deployment

```yaml
# Blue-Green with Traefik
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: order-service
spec:
  routes:
    - match: Host(`api.example.com`) && PathPrefix(`/api/orders`)
      kind: Rule
      services:
        - name: order-service-blue
          port: 9001
          weight: 100
      middlewares:
        - name: traffic-switch
---
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: traffic-switch
spec:
  weighted:
    services:
      - name: order-service-blue
        port: 9001
        weight: 90
      - name: order-service-green
        port: 9001
        weight: 10
```

---

## Disaster Recovery

### Backup Strategy

| Component | Backup Method | Frequency | Retention |
|-----------|--------------|-----------|-----------|
| PostgreSQL | pg_dump + WAL | Daily + continuous | 30 days |
| Redis | RDB + AOF | Hourly | 7 days |
| Kafka | Topic snapshots | Daily | 14 days |
| Kubernetes | Velero | Daily | 30 days |

### Restore Procedure

```bash
# Restore PostgreSQL
pg_restore -h postgres-primary -U postgres -d order_db backup.dump

# Restore Redis
redis-cli --rdb /backup/dump.rdb

# Restore Kubernetes resources
velero restore create --from-backup daily-backup-2024-01-15
```

---

## Monitoring Deployment

### Deployment Metrics

```bash
# Check pod status
kubectl get pods -n order-inventory

# Check deployment status
kubectl get deployments -n order-inventory

# View pod logs
kubectl logs -f deployment/order-service -n order-inventory

# Check resource usage
kubectl top pods -n order-inventory
```

### Deployment Notifications

```yaml
# Slack notification on deploy
- name: Notify Slack
  uses: slackapi/slack-github-action@v1
  with:
    payload: |
      {
        "text": "Deployed ${{ github.repository }} to production",
        "blocks": [
          {
            "type": "section",
            "text": {
              "type": "mrkdwn",
              "text": "*Deployment Complete* :rocket:\n*Service:* order-service\n*Version:* ${{ github.sha }}\n*Environment:* production"
            }
          }
        ]
      }
  env:
    SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK }}
```
