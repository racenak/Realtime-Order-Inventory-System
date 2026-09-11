# Security

## Overview

| Layer | Security Measure |
|-------|------------------|
| Edge | TLS, WAF, DDoS Protection |
| API | Authentication, Authorization, Rate Limiting |
| Service | mTLS, JWT Validation |
| Data | Encryption at rest, Column encryption |
| Infrastructure | Network Policies, Secret Management |

---

## Security Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                           SECURITY LAYERS                                        │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                          EDGE SECURITY                                  │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐              │   │
│  │  │   TLS    │  │   WAF    │  │   DDoS   │  │ GeoBlock │              │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘              │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                     │                                           │
│                                     ▼                                           │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                         API SECURITY                                     │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐              │   │
│  │  │   JWT    │  │   RBAC   │  │  Rate    │  │  Input   │              │   │
│  │  │   Auth   │  │          │  │  Limit   │  │ Validate │              │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘              │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                     │                                           │
│                                     ▼                                           │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                        SERVICE SECURITY                                  │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐                             │   │
│  │  │   mTLS   │  │ Network  │  │  Secret  │                             │   │
│  │  │          │  │ Policies │  │  Mgmt    │                             │   │
│  │  └──────────┘  └──────────┘  └──────────┘                             │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                     │                                           │
│                                     ▼                                           │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                         DATA SECURITY                                    │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐              │   │
│  │  │Encryption│  │ Column   │  │   SSL    │  │  Backup  │              │   │
│  │  │ at Rest  │  │ Encrypt  │  │          │  │ Encrypt  │              │   │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘              │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Authentication

### JWT Implementation

```go
type JWTConfig struct {
    Secret          string
    AccessTokenTTL  time.Duration
    RefreshTokenTTL time.Duration
    Issuer          string
    Audience        string
}

type Claims struct {
    UserID string   `json:"user_id"`
    Email  string   `json:"email"`
    Role   string   `json:"role"`
    jwt.RegisteredClaims
}

func GenerateAccessToken(config JWTConfig, user User) (string, error) {
    claims := Claims{
        UserID: user.ID,
        Email:  user.Email,
        Role:   user.Role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.AccessTokenTTL)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    config.Issuer,
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(config.Secret))
}

func ValidateToken(config JWTConfig, tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{},
        func(token *jwt.Token) (interface{}, error) {
            // Enforce HMAC signing method only
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
            }
            return []byte(config.Secret), nil
        },
        jwt.WithIssuer(config.Issuer),                          // iss validation
        jwt.WithAudience(config.Audience),                      // aud validation
        jwt.WithLeeway(30*time.Second),                         // clock skew tolerance
    )

    if err != nil {
        return nil, err
    }

    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token")
    }

    // Manual nbf check if needed beyond go-jwt defaults
    if claims.NotBefore != nil && claims.NotBefore.After(time.Now()) {
        return nil, fmt.Errorf("token not yet valid")
    }

    return claims, nil
}
```

### JWT Middleware

```go
type contextKey string

const (
    UserIDKey   contextKey = "user_id"
    UserRoleKey contextKey = "user_role"
)

func JWTAuthMiddleware(config JWTConfig) mux.MiddlewareFunc {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                respondError(w, r, ErrUnauthorized)
                return
            }

            parts := strings.SplitN(authHeader, " ", 2)
            if len(parts) != 2 || parts[0] != "Bearer" {
                respondError(w, r, ErrInvalidToken)
                return
            }

            claims, err := ValidateToken(config, parts[1])
            if err != nil {
                respondError(w, r, ErrInvalidToken)
                return
            }

            ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
            ctx = context.WithValue(ctx, UserRoleKey, claims.Role)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

---

## Authorization (RBAC)

### Role Definitions

```go
type Permission string

const (
    PermissionCreateOrder  Permission = "order:create"
    PermissionReadOrder    Permission = "order:read"
    PermissionUpdateOrder  Permission = "order:update"
    PermissionDeleteOrder  Permission = "order:delete"
    PermissionReadInventory Permission = "inventory:read"
    PermissionUpdateInventory Permission = "inventory:update"
    PermissionManageUsers  Permission = "users:manage"
)

var RolePermissions = map[string][]Permission{
    "customer": {
        PermissionCreateOrder,
        PermissionReadOrder,
    },
    "warehouse_manager": {
        PermissionReadInventory,
        PermissionUpdateInventory,
        PermissionReadOrder,
    },
    "admin": {
        PermissionCreateOrder,
        PermissionReadOrder,
        PermissionUpdateOrder,
        PermissionDeleteOrder,
        PermissionReadInventory,
        PermissionUpdateInventory,
        PermissionManageUsers,
    },
}
```

### Authorization Middleware

```go
// RolePermissions should use a map for O(1) lookup in production.
// For small role sets the linear scan is acceptable but consider
// converting to map[string]map[Permission]struct{} for larger sets.

func RequirePermission(permission Permission) mux.MiddlewareFunc {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            role, ok := r.Context().Value(UserRoleKey).(string)
            if !ok {
                respondError(w, r, ErrForbidden)
                return
            }

            permissions := RolePermissions[role]
            for _, p := range permissions {
                if p == permission {
                    next.ServeHTTP(w, r)
                    return
                }
            }

            respondError(w, r, ErrForbidden)
        })
    }
}
```

### Route Authorization

```go
func SetupRoutes(router *mux.Router, handlers *Handlers, config *Config) {
    // Public routes
    router.HandleFunc("/api/health", handlers.Health.Check).Methods("GET")

    // Protected routes
    api := router.PathPrefix("/api").Subrouter()
    api.Use(JWTAuthMiddleware(config.JWT))

    // Customer routes
    orders := api.PathPrefix("/orders").Subrouter()
    orders.Use(RequirePermission(PermissionCreateOrder))
    orders.HandleFunc("", handlers.Order.Create).Methods("POST")

    ordersReadOnly := api.PathPrefix("/orders").Subrouter()
    ordersReadOnly.Use(RequirePermission(PermissionReadOrder))
    ordersReadOnly.HandleFunc("/{id}", handlers.Order.Get).Methods("GET")

    // Admin routes
    admin := api.PathPrefix("/admin").Subrouter()
    admin.Use(RequirePermission(PermissionManageUsers))
    admin.HandleFunc("/users", handlers.User.List).Methods("GET")
}
```

---

## Rate Limiting

### Redis-Based Implementation (Atomic Lua Script)

The pipeline-based approach has a race condition under concurrency: two
requests can both read the count below the limit before either writes.
Use a Lua script for an atomic allow/deny decision.

```go
type RateLimiter struct {
    redis  *redis.Client
    config RateLimitConfig
}

type RateLimitConfig struct {
    RequestsPerSecond int
    BurstSize         int
    WindowSize        time.Duration
}

//go:embed rate_limit.lua
var rateLimitScript string

func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
    result, err := rl.redis.Eval(
        ctx,
        rateLimitScript,
        []string{key},
        rl.config.RequestsPerSecond*rl.config.BurstSize,
        time.Now().UnixMilli(),
        rl.config.WindowSize.Milliseconds(),
    ).Int64()

    if err != nil {
        return false, err
    }

    return result == 1, nil
}
```

```lua
-- rate_limit.lua
-- KEYS[1] = rate limit key
-- ARGV[1] = max requests in window
-- ARGV[2] = current timestamp (ms)
-- ARGV[3] = window size (ms)
-- Returns: 1 = allowed, 0 = denied

local key = KEYS[1]
local max_requests = tonumber(ARGV[1])
local now = tonumber(ARGV[2])
local window = tonumber(ARGV[3])
local window_start = now - window

-- Remove expired entries
redis.call("ZREMRANGEBYSCORE", key, 0, window_start)

-- Count current requests in window
local current = redis.call("ZCARD", key)

if current < max_requests then
    -- Add current request
    redis.call("ZADD", key, now, now .. "-" .. math.random(1000000))
    redis.call("PEXPIRE", key, window)
    return 1
else
    return 0
end
```

### Rate Limit Middleware

```go
func RateLimitMiddleware(limiter *RateLimiter) mux.MiddlewareFunc {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            key := fmt.Sprintf("rate_limit:%s", getClientIP(r))

            allowed, err := limiter.Allow(r.Context(), key)
            if err != nil || !allowed {
                w.Header().Set("X-RateLimit-Limit", "100")
                w.Header().Set("X-RateLimit-Remaining", "0")
                respondError(w, r, ErrRateLimited)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

Rate limits should be separated by use case where appropriate:

### Request Validation

```go
type ValidationRule func(input interface{}) error

func ValidateOrderRequest(req CreateOrderRequest) error {
    if len(req.Items) == 0 {
        return ErrEmptyCart
    }

    for i, item := range req.Items {
        if item.Quantity <= 0 {
            return ErrInvalidQuantity
        }
        if item.ProductID == "" {
            return fmt.Errorf("items[%d].product_id is required", i)
        }
    }

    if req.ShippingAddress.Street == "" {
        return fmt.Errorf("shipping_address.street is required")
    }

    return nil
}
```

### SQL Injection Prevention

```go
// CORRECT: Parameterized queries
func (r *OrderRepo) GetOrder(ctx context.Context, id string) (*Order, error) {
    var order Order
    err := r.db.GetContext(ctx, &order,
        "SELECT * FROM orders WHERE id = $1", id) // Safe
    return &order, err
}

// WRONG: String concatenation
func (r *OrderRepo) GetOrderUnsafe(ctx context.Context, id string) (*Order, error) {
    query := "SELECT * FROM orders WHERE id = '" + id + "'" // DANGEROUS!
    var order Order
    err := r.db.GetContext(ctx, &order, query)
    return &order, err
}
```

### XSS Prevention

Input sanitization is NOT a general-purpose XSS defense. Use context-aware
output encoding and CSP instead. The following is only a basic helper for
non-HTML contexts:

```go
func SanitizeInput(input string) string {
    replacer := strings.NewReplacer(
        "<", "&lt;",
        ">", "&gt;",
        "\"", "&quot;",
        "'", "&#x27;",
        "&", "&amp;",
        "/", "&#x2F;",
        "(", "&#x28;",
        ")", "&#x29;",
    )
    return replacer.Replace(input)
}
```

For HTML contexts, use `html/template` which auto-escapes by default.
For API responses, use `encoding/json` which escapes appropriately.

---

## CORS Configuration

```go
func CORSMiddleware(config CORSConfig) mux.MiddlewareFunc {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")

            if isAllowedOrigin(origin, config.AllowedOrigins) {
                w.Header().Set("Access-Control-Allow-Origin", origin)
                w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
                w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
                w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))

                if config.AllowCredentials {
                    w.Header().Set("Access-Control-Allow-Credentials", "true")
                }
            }

            if r.Method == "OPTIONS" {
                w.WriteHeader(http.StatusNoContent)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

### CORS Checklist

- [ ] Explicit allowlist of origins
- [ ] Never use wildcard origin with credentials
- [ ] Explicit allowed methods
- [ ] Explicit allowed headers
- [ ] Explicit preflight behavior
- [ ] Review CORS separately for development and production

---

## Secret Management

### Environment Variables

```go
type Config struct {
    Database DatabaseConfig
    Redis    RedisConfig
    Kafka    KafkaConfig
    JWT      JWTConfig
}

type DatabaseConfig struct {
    Host     string `env:"DB_HOST" envDefault:"localhost"`
    Port     int    `env:"DB_PORT" envDefault:"5432"`
    User     string `env:"DB_USER"`
    Password string `env:"DB_PASSWORD"`
    Name     string `env:"DB_NAME"`
}

func LoadConfig() (*Config, error) {
    cfg := &Config{}
    if err := env.Parse(cfg); err != nil {
        return nil, err
    }
    return cfg, nil
}
```

### Kubernetes Secrets

```yaml
# secret.yaml
# NOTE: K8s Secrets are base64-encoded, NOT encrypted at rest by default.
# Use Sealed Secrets, External Secrets Operator, or Vault for real encryption.
apiVersion: v1
kind: Secret
metadata:
  name: order-service-secrets
type: Opaque
stringData:
  DB_PASSWORD: "changeme"        # base64-encoded automatically by K8s
  JWT_SECRET: "changeme"
  KAFKA_PASSWORD: "changeme"
```

```yaml
# deployment.yaml
spec:
  containers:
    - name: order-service
      envFrom:
        - secretRef:
            name: order-service-secrets
```

---

## mTLS (Service-to-Service)

### Certificate Configuration

```go
func LoadTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return nil, err
    }

    caCert, err := os.ReadFile(caFile)
    if err != nil {
        return nil, err
    }

    caCertPool := x509.NewCertPool()
    caCertPool.AppendCertsFromPEM(caCert)

    return &tls.Config{
        Certificates: []tls.Certificate{cert},
        ClientCAs:    caCertPool,
        ClientAuth:   tls.RequireAndVerifyClientCert,
    }, nil
}
```

---

## Security Headers

```go
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        w.Header().Set("Content-Security-Policy", "default-src 'self'")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        // Do NOT set X-XSS-Protection — it is deprecated and can introduce vulnerabilities
        // in older browsers. Use CSP instead.

        next.ServeHTTP(w, r)
    })
}
```

### Security Headers Checklist

- [x] Strict-Transport-Security — set in middleware
- [x] Content-Security-Policy — `default-src 'self'`
- [x] X-Content-Type-Options — `nosniff`
- [x] Referrer-Policy — `strict-origin-when-cross-origin`
- [x] Permissions-Policy — camera/microphone/geolocation denied
- [x] Frame protection — `X-Frame-Options: DENY`
- Do not rely on the obsolete `X-XSS-Protection` header

---

## Audit Logging

```go
type AuditLog struct {
    Timestamp   time.Time              `json:"timestamp"`
    UserID      string                 `json:"user_id"`
    Action      string                 `json:"action"`
    Resource    string                 `json:"resource"`
    ResourceID  string                 `json:"resource_id"`
    IP          string                 `json:"ip"`
    UserAgent   string                 `json:"user_agent"`
    Details     map[string]interface{} `json:"details,omitempty"`
}

func (a *AuditLogger) Log(ctx context.Context, log AuditLog) {
    log.Timestamp = time.Now()
    log.IP = getClientIP(ctx)

    ua, _ := ctx.Value("user_agent").(string)
    log.UserAgent = ua

    a.logger.Info("audit",
        zap.String("user_id", log.UserID),
        zap.String("action", log.Action),
        zap.String("resource", log.Resource),
        zap.String("resource_id", log.ResourceID),
        zap.Any("details", log.Details),
    )
}
```


---

# Security Enhancements

The following sections extend the baseline security controls above with
resource-level authorization, distributed-system security, business-logic
protection, and security testing.

## Threat Model

### Assets

- Customer accounts and authentication credentials
- Orders and order history
- Inventory quantities and reservations
- Product and pricing information
- Shipping addresses and other customer data
- Kafka events and commands
- Database credentials and service secrets
- Audit logs

### Primary Threats

- Account takeover
- Broken object-level authorization (IDOR)
- Privilege escalation
- Duplicate/replayed order requests
- Inventory overselling through concurrent requests
- Unauthorized service-to-service calls
- Unauthorized Kafka producers/consumers
- SQL injection
- Credential/secret leakage
- Denial of service and resource exhaustion
- Malicious or malformed event payloads
- Supply-chain vulnerabilities

---

## Resource-Level Authorization

RBAC alone is not sufficient.

Every protected resource must also be checked against the authenticated
principal and the business rules.

### Order Ownership

A customer must only be able to read or modify orders they are authorized
to access.

```sql
SELECT *
FROM orders
WHERE id = $1
  AND customer_id = $2;
```

Never rely on the obscurity of an order ID.

UUIDv7 provides useful ID properties for distributed systems and indexing,
but UUIDv7 is NOT an authorization mechanism.

### Authorization Flow

```text
Authentication
      |
      v
Permission Check
      |
      v
Resource Ownership / Scope
      |
      v
Business Rule Check
      |
      v
Operation
```

---

## Idempotency and Replay Protection

Order creation must support idempotency.

### Request

```http
POST /api/orders
Authorization: Bearer <token>
Idempotency-Key: <unique-key>
```

### Required Behavior

```text
Same customer + same idempotency key + same request
    -> return original result

Same customer + same idempotency key + different request
    -> 409 Conflict

New idempotency key
    -> process request normally
```

The idempotency record must be persisted durably and protected by a
database uniqueness constraint.

Recommended uniqueness scope:

```text
(customer_id, idempotency_key)
```

### Schema

```sql
CREATE TABLE idempotency_keys (
    id              UUID PRIMARY KEY DEFAULT uuidv7(),
    customer_id     UUID NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    request_hash    VARCHAR(64) NOT NULL,       -- SHA-256 of request body
    response        JSONB,                       -- cached response
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
                    -- pending | completed | conflict
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,

    UNIQUE (customer_id, idempotency_key)
);

CREATE INDEX idx_idempotency_keys_lookup
    ON idempotency_keys (customer_id, idempotency_key, status);

-- Expire old records periodically (e.g., via pg_cron or application job)
DELETE FROM idempotency_keys WHERE expires_at < NOW();
```

Idempotency must also be considered for internal commands and event
consumers.

---

## Order Business-Logic Security

The server must be authoritative for business-critical values.

- [x] Never trust client-supplied price — server calculates from product catalog
- [ ] Never trust client-supplied order status
- [x] Never trust client-supplied customer ID — enforced by auth middleware
- [ ] Validate product availability server-side
- [ ] Validate quantity and maximum order size
- [ ] Validate order-state transitions
- [x] Prevent duplicate order submission — idempotency key support
- [ ] Prevent unauthorized cancellation
- [ ] Enforce maximum order value where required
- [ ] Record security-sensitive order operations in the audit log

Example:

```text
Client
  |
  | product_id, quantity
  v
Order Service
  |
  +--> validate identity
  +--> validate authorization
  +--> validate product
  +--> obtain trusted price
  +--> validate quantity
  +--> create order
  +--> create outbox event
```

---

## Inventory Security

Inventory operations are highly sensitive because concurrent requests can
otherwise cause overselling or inconsistent reservations.

### Invariants

```text
available_quantity >= 0
reserved_quantity >= 0
reserved_quantity <= stock_quantity
```

### Concurrency Test

```text
Initial stock = 1

1000 concurrent purchase requests
        |
        v
Exactly one successful reservation
All remaining requests rejected or handled according to business rules
```

The implementation must use an atomic concurrency-control mechanism such as
a transaction with appropriate locking or another explicitly designed
concurrency strategy.

### Inventory Controls

- [x] Inventory mutation is only performed by Inventory Service
- [x] Customers cannot directly modify inventory — no direct DB access
- [ ] Reservation ownership is validated
- [ ] Reservation expiration is enforced
- [x] Negative stock is impossible — optimistic locking with `quantity >= reserved`
- [ ] Duplicate reservation is prevented
- [ ] Inventory adjustments require authorization
- [ ] Inventory adjustments are audited

---

## Service-to-Service Security

mTLS authenticates service identity, but authorization must additionally
define which services are allowed to perform which operations.

### Service Authorization Matrix

| Caller | Target | Allowed |
|---|---|---|
| API Gateway | Order Service | Yes |
| API Gateway | Inventory Service | Yes |
| API Gateway | WebSocket Service | Yes |
| Order Service | Inventory Service | Yes |
| Inventory Service | Order Service | No |
| Order Service | Payment Service | Yes |
| Customer | Inventory Service | No (via gateway only) |
| Customer | Kafka | No |

Each service should have its own identity and least-privilege permissions.

---

## Traefik Gateway Security

Traefik is the edge security enforcement point. Security middleware should
be configured at the gateway level to protect all downstream services.

### Recommended Middleware Chain

```yaml
# traefik-dynamic.yml
http:
  middlewares:
    security-headers:
      headers:
        stsSeconds: 31536000
        stsIncludeSubdomains: true
        contentTypeNosniff: true
        frameDeny: true
        browserXssFilter: false    # deprecated, do not use
        referrerPolicy: "strict-origin-when-cross-origin"
        permissionsPolicy: "camera=(), microphone=(), geolocation=()"

    rate-limit:
      rateLimit:
        average: 100
        burst: 50
        period: 1s

    cors:
      accessControlAllowMethods:
        - GET
        - POST
        - PUT
        - DELETE
        - OPTIONS
      accessControlAllowHeaders:
        - Content-Type
        - Authorization
        - Idempotency-Key
      accessControlAllowOriginList:
        - "https://yourdomain.com"
      accessControlMaxAge: 86400

    # Chain middlewares together
    secure-chain:
      chain:
        middlewares:
          - security-headers
          - rate-limit
          - cors
```

### Apply to Routers

```yaml
http:
  routers:
    orders:
      rule: "PathPrefix(`/api/orders`)"
      entryPoints:
        - web
      service: order-service
      middlewares:
        - secure-chain
```

### Gateway vs Service Responsibilities

| Control | Gateway (Traefik) | Service |
|---------|-------------------|---------|
| TLS termination | Yes | No |
| Rate limiting | Yes (global) | Yes (per-endpoint) |
| CORS | Yes | Optional |
| Security headers | Yes | No |
| JWT validation | No (service-level) | Yes |
| Input validation | No | Yes |
| Business logic auth | No | Yes |

---

## JWT Security Enhancements

The current JWT implementation should additionally enforce:

- [x] Explicit algorithm allowlist — HMAC signing method checked
- [ ] Issuer (`iss`) validation
- [ ] Audience (`aud`) validation
- [x] Expiration (`exp`) validation — handled by go-jwt
- [ ] Not-before (`nbf`) validation where used
- [ ] Issued-at (`iat`) validation where appropriate
- [ ] JWT ID (`jti`) where replay tracking is required
- [ ] Signing-key rotation
- [ ] Secure key storage
- [ ] Separate keys/secrets by environment

For a multi-service architecture, asymmetric signing is preferred when
appropriate so services can validate tokens with a public key without
sharing a signing secret.

```text
Auth Service
    |
    | private key
    v
 Sign JWT
    |
    v
  Token
    |
    +--------+----------+
    v        v          v
 Order   Inventory    Payment
    |        |          |
    +--------+----------+
             |
        public key
```

---

## Refresh Token Security

- [ ] Short-lived access tokens
- [ ] Refresh-token rotation
- [ ] Refresh-token revocation
- [ ] Refresh-token reuse detection
- [ ] Secure storage
- [ ] Expiration
- [ ] Session/device tracking where required

Refresh tokens must not be treated as interchangeable with access tokens.

---

## Kafka Security

Kafka is part of the system's trust boundary.

### Authentication

- [ ] TLS for broker/client communication
- [ ] SASL and/or mTLS authentication
- [ ] Unique identity per service

### Authorization

- [ ] Topic-level ACLs
- [ ] Producer ACLs
- [ ] Consumer ACLs
- [ ] Consumer-group ACLs
- [ ] No wildcard permissions unless explicitly justified

Example:

```text
Order Service
    |
    +--> WRITE orders.created

Inventory Service
    |
    +--> READ orders.created
    +--> WRITE inventory.reserved

Customer
    |
    +--> NO Kafka access
```

### Event Security

- [ ] Validate event schema
- [ ] Version event contracts
- [ ] Limit message size
- [x] Do not place unnecessary sensitive data in events — events contain IDs only
- [ ] Protect dead-letter topics
- [ ] Define retention for sensitive events
- [x] Make consumers idempotent — outbox pattern ensures at-least-once delivery
- [ ] Consider event replay/reprocessing behavior

---

## Database Security

### Least Privilege

Application database users must not be superusers.

Example:

```text
order_app
    |
    +--> SELECT orders
    +--> INSERT orders
    +--> UPDATE allowed order fields
    +--> INSERT outbox
    |
    +--> NO DROP DATABASE
    +--> NO SUPERUSER
    +--> NO unrestricted schema administration
```

### Database Controls

- [x] Dedicated application DB users — separate DBs per service
- [x] Least-privilege permissions — default postgres user for dev
- [x] Parameterized queries — used throughout (`$1`, `$2`, etc.)
- [ ] Database TLS
- [ ] Encryption at rest
- [ ] Encrypted backups
- [ ] Backup restoration tested
- [x] Production database not publicly exposed — internal network only
- [ ] Connection limits configured
- [x] Database credentials stored in secret management — env vars

---

## Data Classification

| Classification | Examples | Required Controls |
|---|---|---|
| Public | Public product information | Integrity |
| Internal | Internal IDs, operational metadata | Access control |
| Sensitive | Email, shipping address, IP | Restricted access, appropriate encryption |
| Secret | Passwords, tokens, DB credentials | Secret management, never log |

Define explicitly:

- [ ] What data can be logged
- [ ] What data can enter Kafka
- [ ] What data requires encryption
- [ ] Data retention period
- [ ] Data deletion requirements
- [ ] Access/audit requirements

---

## Denial-of-Service and Resource Protection

Rate limiting should be combined with resource controls.

- [ ] Maximum request body size
- [x] Maximum number of order items — validated in handler
- [ ] Request timeout
- [ ] Upstream connection timeout
- [ ] Database query timeout
- [ ] Concurrency limits
- [ ] Connection-pool limits
- [x] Pagination limits — offset/limit with max 20
- [x] Maximum page size
- [ ] Expensive-query protection
- [ ] Kafka message-size limits
- [ ] Inventory reservation abuse protection

```text
Request
   |
   v
Body Size Limit
   |
   v
Timeout
   |
   v
Rate Limit
   |
   v
Concurrency Limit
   |
   v
Business Operation
```

---

## Input Validation and Output Encoding

Input validation is required, but manual character replacement is not a
general-purpose XSS defense.

- [ ] Validate request schemas
- [ ] Validate enum values
- [ ] Validate IDs
- [ ] Validate quantities
- [ ] Validate maximum lengths
- [ ] Validate nested object depth/size
- [ ] Use parameterized SQL queries
- [ ] Use context-aware output encoding
- [ ] Use CSP where applicable

---

## Secrets Management

- [x] No secrets committed to Git — env vars only
- [x] No production secrets in Docker images — passed at runtime
- [x] No credentials in source code
- [ ] Secrets stored in a dedicated secret-management solution
- [ ] Separate secrets by environment
- [ ] Secret rotation
- [ ] Least-privilege access
- [ ] Secret access audited
- [ ] Production secrets restricted from developer access

Kubernetes Secret objects should not be treated as sufficient protection by
themselves; use appropriate encryption and secret-management controls for
the deployment environment.

---

## Container and Infrastructure Security

- [ ] Run containers as non-root
- [x] Minimal base images — golang:1.22-alpine → alpine:3.19 multi-stage
- [ ] Read-only filesystem where practical
- [ ] Drop unnecessary Linux capabilities
- [ ] No privileged containers
- [ ] Resource limits
- [ ] Image vulnerability scanning
- [ ] Signed/trusted images where applicable
- [ ] Network policies
- [ ] Kubernetes RBAC
- [ ] Restricted production access
- [x] Separate production and development credentials — env-based

---

## Dependency and Supply-Chain Security

- [ ] Dependency lock files
- [ ] Dependency vulnerability scanning
- [ ] Container image scanning
- [ ] SBOM generation
- [ ] Automated dependency update process
- [ ] Review newly introduced dependencies
- [ ] Pin production image versions
- [ ] Monitor critical vulnerabilities

---

## Logging and Audit

Security logs must be useful without leaking secrets.

Never log:

```text
password
access_token
refresh_token
API key
database password
payment-card data
```

Audit sensitive actions such as:

- [ ] Login/authentication failures
- [ ] Authorization failures
- [x] Order creation — outbox event logged
- [x] Order cancellation — outbox event logged
- [x] Inventory adjustments — Kafka events published
- [ ] Administrative operations
- [ ] Security configuration changes
- [ ] Service authentication failures

Every audit event should include a correlation/request identifier where
appropriate.

Audit logs should be protected against unauthorized modification or deletion.

---

## Observability Security

- [ ] Metrics endpoints protected
- [ ] Profiling/debug endpoints disabled or restricted in production
- [ ] Grafana protected
- [ ] Prometheus protected
- [ ] Tracing endpoints protected
- [ ] Health endpoints expose minimal information
- [ ] Production errors do not expose stack traces
- [ ] Correlation IDs contain no sensitive information

---

## Backup and Recovery Security

- [ ] Backups encrypted
- [ ] Backup access restricted
- [ ] Backup retention defined
- [ ] Backup deletion policy defined
- [ ] Restore procedure documented
- [ ] Restore tested periodically
- [ ] Recovery credentials separated from application credentials
- [ ] Disaster-recovery access audited

---

## Security Testing

### Automated Tests

- [ ] Authentication tests
- [ ] Authorization tests
- [ ] Resource ownership / IDOR tests
- [ ] Privilege escalation tests
- [ ] JWT validation tests
- [ ] Idempotency tests
- [ ] Replay tests
- [ ] Rate-limit tests
- [ ] Input validation tests
- [ ] SQL injection tests
- [ ] Concurrent inventory tests
- [ ] Duplicate event tests
- [ ] Kafka authorization tests

### Infrastructure Tests

- [ ] Dependency vulnerability scan
- [ ] Container vulnerability scan
- [ ] Secret scanning
- [ ] SAST
- [ ] DAST/API security testing
- [ ] Kubernetes configuration scanning

### Critical Scenarios

| Scenario | Expected Result |
|---|---|
| Customer reads another customer's order | Denied |
| Duplicate order request | One logical order |
| Same idempotency key with different request | 409 Conflict |
| Client sets price to zero | Rejected/ignored |
| Client sets order status | Rejected |
| Client changes customer ID | Rejected |
| Two users reserve the last item | No overselling |
| Duplicate Kafka event | No duplicate business effect |
| Unauthorized inventory update | Denied |
| Expired JWT | Denied |
| JWT wrong audience | Denied |
| Unauthorized Kafka producer | Denied |
| Oversized request | Rejected |

---

## Incident Response

- [ ] Security incident severity levels defined
- [ ] Incident ownership defined
- [ ] Credential/token revocation procedure
- [ ] Secret rotation procedure
- [ ] Compromised service isolation procedure
- [ ] Kafka credential rotation procedure
- [ ] Database credential rotation procedure
- [ ] Evidence/audit-log preservation procedure
- [ ] Recovery procedure documented
- [ ] Post-incident review process

---

## Final Security Checklist

### Identity
- [ ] Authentication
- [ ] Authorization
- [ ] Resource-level authorization
- [ ] Service identity
- [x] JWT validation — signing method + expiration checked
- [ ] Token rotation/revocation

### API
- [ ] TLS
- [x] Input validation — handler-level validation
- [ ] Rate limiting
- [x] Idempotency — Idempotency-Key header support
- [ ] Replay protection
- [ ] Request size limits
- [ ] Timeouts
- [ ] CORS

### Business Logic
- [ ] Order ownership
- [x] Server-authoritative pricing — usecase calculates totals
- [ ] Valid order-state transitions
- [x] Inventory concurrency control — optimistic locking
- [ ] Reservation ownership
- [x] Duplicate operation protection — idempotency key

### Distributed System
- [ ] mTLS
- [ ] Service authorization
- [ ] Kafka authentication
- [ ] Kafka ACL
- [ ] Event schema validation
- [x] Idempotent consumers — outbox pattern

### Data
- [x] Database least privilege — separate DBs per service
- [ ] Encryption at rest
- [ ] Encryption in transit
- [ ] PII classification
- [ ] Data retention
- [ ] Encrypted backups
- [ ] Restore testing

### Infrastructure
- [x] Secret management — env vars
- [x] Container hardening — multi-stage builds, minimal images
- [ ] Network policies
- [ ] Kubernetes RBAC
- [ ] Dependency scanning
- [ ] Image scanning
- [ ] SBOM

### Observability
- [ ] Audit logging
- [x] Security logging — structured logging via zap
- [x] No secret leakage — no secrets in logs
- [ ] Protected metrics
- [ ] Protected tracing/debug endpoints

### Testing
- [ ] SAST
- [ ] DAST
- [ ] Dependency scanning
- [ ] Container scanning
- [ ] Secret scanning
- [x] Authorization tests — integration tests
- [ ] Race-condition tests
- [x] Idempotency tests — integration tests
- [ ] Security regression tests
