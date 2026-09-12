# Security

## Authentication

### JWT via Traefik ForwardAuth

**Flow:**
```
Client → Traefik → Auth Service (/validate) → Traefik → Service
```

**Implementation:**
- Traefik intercepts all `/api/*` requests
- ForwardAuth sends request to `http://auth-service:8083/validate`
- Auth Service decodes JWT, validates:
  - Signature (HMAC-SHA256)
  - Issuer (`order-inventory-system`)
  - Audience (`order-inventory-api`)
  - Expiry
- On valid JWT: returns `200 OK` with `X-User-Id` and `X-User-Role` headers
- On invalid JWT: returns `401 Unauthorized`, Traefik blocks request

**Auth Service:**
- Stateless (no session storage)
- Validates JWT using `golang-jwt/jwt/v5`
- Secrets managed via environment variables (`JWT_SECRET`)

---

## Authorization

### Role-Based Access Control

| Endpoint | Method | Required Role |
|----------|--------|---------------|
| `POST /api/orders` | POST | user, admin |
| `GET /api/orders` | GET | user, admin |
| `GET /api/orders/:id` | GET | user, admin |
| `POST /api/orders/:id/cancel` | POST | user, admin |
| `POST /api/inventory/reserve` | POST | user, admin |
| `POST /api/inventory/release` | POST | user, admin |
| `PUT /api/inventory/stock` | PUT | admin |

### RBAC Implementation

- Permissions defined per role
- Middleware validates `X-User-Role` header (set by Traefik ForwardAuth)
- Route-level authorization in HTTP handlers

### Resource-Level Authorization

- Order ownership: User can only access their own orders
- Inventory: Admin-only for stock updates

---

## Transport Security

### TLS

- Traefik supports TLS termination (Let's Encrypt ACME config ready)
- Currently HTTP-only for local development
- DB connections: localhost only (no TLS for dev)

### Internal Communication

- Service-to-service via internal Docker network
- No external exposure except through Traefik

---

## Data Security

### SQL Injection Prevention

All queries use parameterized statements:

```go
// Parameterized query - safe
query := "SELECT * FROM orders WHERE id = $1"
row := db.QueryRowContext(ctx, query, orderID)

// String concatenation - vulnerable (NEVER USED)
// query := "SELECT * FROM orders WHERE id = '" + orderID + "'"
```

### Secrets Management

| Secret | Storage |
|--------|---------|
| DB passwords | Environment variables |
| JWT secret | Environment variable (`JWT_SECRET`) |
| Redis password | Environment variable |

### Logging

- No sensitive data in logs (no passwords, tokens, PII)
- Structured JSON logging via Zap
- Log shipping to Loki (encrypted in transit)

---

## Network Security

### Traefik Security Headers

```yaml
# dynamic.yml
middlewares:
  security-headers:
    headers:
      stsSeconds: 31536000
      stsIncludeSubdomains: true
      stsPreload: true
      forceSTSHeader: true
      contentTypeNosniff: true
      browserXssFilter: true
      referrerPolicy: "strict-origin-when-cross-origin"
      frameDeny: true
      contentTypeSecurityPolicy: "default-src 'self'"
      permissionsPolicy: "camera=(), microphone=(), geolocation=()"
```

### Rate Limiting

- **100 requests/second** per IP
- **50 burst** capacity
- Configured via Traefik middleware

### CORS

Configured per-service for browser clients.

---

## Container Security

### Podman (Rootless)

- Containers run without root privileges
- No daemon (unlike Docker)
- User namespace isolation

### Image Security

- Multi-stage builds (minimal attack surface)
- Alpine-based images (small footprint)
- No unnecessary packages

### Volume Permissions

```yaml
volumes:
  - grafana-data:/var/lib/grafana:z  # SELinux label
```

---

## Vulnerability Management

### Dependency Scanning

- Go modules with checksum verification
- `go.sum` committed for reproducibility

### Container Scanning

- Scan images before deployment
- Update base images regularly

---

## Audit Trail

### Order Status History

Every order status change is recorded:

```sql
CREATE TABLE order_status_history (
    id VARCHAR(26) PRIMARY KEY,
    order_id VARCHAR(26) NOT NULL,
    old_status VARCHAR(20),
    new_status VARCHAR(20) NOT NULL,
    reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### Inventory Movements

Every stock change is recorded:

```sql
CREATE TABLE inventory_movements (
    id VARCHAR(26) PRIMARY KEY,
    product_id VARCHAR(26) NOT NULL,
    warehouse_id VARCHAR(26) NOT NULL,
    movement_type VARCHAR(20) NOT NULL,
    quantity INT NOT NULL,
    reference_type VARCHAR(50),
    reference_id VARCHAR(26),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### Structured Logging

All operations logged with:
- Timestamp
- Service name
- Operation type
- Resource IDs
- User ID (from JWT)
- Duration
- Error (if any)

---

## What's Implemented

- [x] JWT authentication via Traefik ForwardAuth
- [x] Role-based access control (RBAC)
- [x] Rate limiting (100 req/s, 50 burst)
- [x] Security headers (HSTS, CSP, X-Frame-Options, etc.)
- [x] Parameterized SQL queries (no injection)
- [x] Structured logging without sensitive data
- [x] Environment-based secret management
- [x] Order audit trail (status history)
- [x] Inventory audit trail (movements)
- [x] Rootless Podman containers
- [x] Multi-stage Docker builds

## Not Implemented

- [ ] Database TLS encryption
- [ ] Refresh token rotation
- [ ] API key authentication
- [ ] OAuth2 integration
- [ ] WAF (Web Application Firewall)
