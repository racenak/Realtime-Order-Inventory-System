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
            if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
            }
            return []byte(config.Secret), nil
        },
    )

    if err != nil {
        return nil, err
    }

    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token")
    }

    return claims, nil
}
```

### JWT Middleware

```go
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

            // Add claims to context
            ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
            ctx = context.WithValue(ctx, "user_role", claims.Role)
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
func RequirePermission(permission Permission) mux.MiddlewareFunc {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            role, ok := r.Context().Value("user_role").(string)
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

### Redis-Based Implementation

```go
type RateLimiter struct {
    redis  *redis.Client
    config RateLimitConfig
}

type RateLimitConfig struct {
    RequestsPerSecond int
    BurstSize         int
    CleanupInterval   time.Duration
}

func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
    now := time.Now().UnixMilli()
    windowStart := now - int64(rl.config.CleanupInterval.Milliseconds())

    pipe := rl.redis.Pipeline()

    // Remove old entries
    pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))

    // Count current requests
    count := pipe.ZCard(ctx, key)

    // Add current request
    pipe.ZAdd(ctx, key, &redis.Z{
        Score:  float64(now),
        Member: now,
    })

    // Set expiry
    pipe.Expire(ctx, key, rl.config.CleanupInterval)

    _, err := pipe.Exec(ctx)
    if err != nil {
        return false, err
    }

    return count.Val() < int64(rl.config.RequestsPerSecond*rl.config.BurstSize), nil
}
```

### Middleware

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

---

## Input Validation

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

```go
func SanitizeInput(input string) string {
    replacer := strings.NewReplacer(
        "<", "&lt;",
        ">", "&gt;",
        "\"", "&quot;",
        "'", "&#x27;",
        "/", "&#x2F;",
    )
    return replacer.Replace(input)
}
```

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
apiVersion: v1
kind: Secret
metadata:
  name: order-service-secrets
type: Opaque
stringData:
  DB_PASSWORD: "encrypted-password"
  JWT_SECRET: "encrypted-jwt-secret"
  KAFKA_PASSWORD: "encrypted-kafka-password"
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
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        w.Header().Set("Content-Security-Policy", "default-src 'self'")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

        next.ServeHTTP(w, r)
    })
}
```

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
    log.UserAgent = ctx.Value("user_agent").(string)

    a.logger.Info("audit",
        zap.String("user_id", log.UserID),
        zap.String("action", log.Action),
        zap.String("resource", log.Resource),
        zap.String("resource_id", log.ResourceID),
        zap.Any("details", log.Details),
    )
}
```
