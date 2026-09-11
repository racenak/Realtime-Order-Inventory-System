# API Specification

## Overview

| Property | Value |
|----------|-------|
| Base URL | `https://{domain}/api` |
| Version | v1 |
| Format | JSON |
| Auth | Bearer Token (JWT) |

## Authentication

All API requests (except `/health`) require a valid JWT in the `Authorization` header. The JWT is validated by the Auth Service via Traefik ForwardAuth middleware.

### Auth Service

| Property | Value |
|----------|-------|
| Port | 8083 (internal) |
| Endpoint | `POST /verify` |
| Validation | HMAC-SHA256, issuer, audience, expiration |

### Token Structure

```json
{
  "user_id": "uuid",
  "email": "user@example.com",
  "role": "customer|warehouse_manager|admin",
  "iss": "order-inventory-system",
  "aud": "order-inventory-api",
  "exp": 1234567890
}
```

### Headers

```
Authorization: Bearer <token>
Content-Type: application/json
Idempotency-Key: <unique-key>   (for POST requests)
```

### Traefik Middleware Chain

| Chain | Middlewares | Used By |
|-------|------------|---------|
| `public-chain` | rate-limit, security-headers | `/health` |
| `protected-chain` | jwt-auth, rate-limit, security-headers | `/api/*`, `/ws` |

### Response Headers (from Auth Service)

When JWT is valid, Traefik forwards these headers to downstream services:

| Header | Description |
|--------|-------------|
| `X-User-Id` | Authenticated user's UUID |
| `X-User-Role` | User's role (customer, warehouse_manager, admin) |

---

## Order Service

**Base Path:** `/api/orders`

### 1. Create Order

```http
POST /api/orders
```

**Request:**

```json
{
  "items": [
    {
      "product_id": "prod_001",
      "quantity": 2
    },
    {
      "product_id": "prod_002",
      "quantity": 1
    }
  ],
  "shipping_address": {
    "street": "123 Main St",
    "city": "New York",
    "state": "NY",
    "zip": "10001",
    "country": "US"
  }
}
```

**Response (201):**

```json
{
  "success": true,
  "data": {
    "order_id": "ord_abc123",
    "status": "pending_payment",
    "items": [
      {
        "product_id": "prod_001",
        "product_name": "Widget A",
        "quantity": 2,
        "unit_price": 29.99,
        "total_price": 59.98
      },
      {
        "product_id": "prod_002",
        "product_name": "Gadget B",
        "quantity": 1,
        "unit_price": 49.99,
        "total_price": 49.99
      }
    ],
    "subtotal": 109.97,
    "tax": 8.80,
    "total": 118.77,
    "shipping_address": {
      "street": "123 Main St",
      "city": "New York",
      "state": "NY",
      "zip": "10001",
      "country": "US"
    },
    "created_at": "2024-01-15T10:30:00Z"
  }
}
```

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 400 | `INVALID_REQUEST` | Missing required fields |
| 400 | `EMPTY_CART` | No items in order |
| 400 | `INVALID_QUANTITY` | Quantity must be > 0 |
| 404 | `PRODUCT_NOT_FOUND` | Product does not exist |
| 409 | `INSUFFICIENT_STOCK` | Not enough inventory |
| 429 | `RATE_LIMITED` | Too many requests |

---

### 2. Get Order

```http
GET /api/orders/{order_id}
```

**Response (200):**

```json
{
  "success": true,
  "data": {
    "order_id": "ord_abc123",
    "user_id": "usr_xyz789",
    "status": "processing",
    "items": [...],
    "subtotal": 109.97,
    "tax": 8.80,
    "total": 118.77,
    "shipping_address": {...},
    "status_history": [
      {
        "status": "pending_payment",
        "changed_at": "2024-01-15T10:30:00Z"
      },
      {
        "status": "paid",
        "changed_at": "2024-01-15T10:31:15Z"
      },
      {
        "status": "processing",
        "changed_at": "2024-01-15T10:35:00Z"
      }
    ],
    "created_at": "2024-01-15T10:30:00Z",
    "updated_at": "2024-01-15T10:35:00Z"
  }
}
```

---

### 3. List Orders

```http
GET /api/orders
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | 1 | Page number |
| `limit` | int | 20 | Items per page (max 100) |
| `status` | string | - | Filter by status |
| `sort` | string | created_at | Sort field |
| `order` | string | desc | Sort order (asc/desc) |
| `from` | string | - | Start date (RFC3339) |
| `to` | string | - | End date (RFC3339) |

**Response (200):**

```json
{
  "success": true,
  "data": {
    "orders": [...],
    "pagination": {
      "current_page": 1,
      "total_pages": 10,
      "total_items": 195,
      "items_per_page": 20
    }
  }
}
```

---

### 4. Cancel Order

```http
POST /api/orders/{order_id}/cancel
```

**Request:**

```json
{
  "reason": "Changed my mind"
}
```

**Response (200):**

```json
{
  "success": true,
  "data": {
    "order_id": "ord_abc123",
    "status": "cancelled",
    "cancelled_at": "2024-01-15T11:00:00Z",
    "refund_status": "pending"
  }
}
```

**Error Responses:**

| Code | Error | Description |
|------|-------|-------------|
| 400 | `ORDER_NOT_CANCELLABLE` | Order already shipped/delivered |
| 404 | `ORDER_NOT_FOUND` | Order does not exist |

---

### 5. Get Order Status

```http
GET /api/orders/{order_id}/status
```

**Response (200):**

```json
{
  "success": true,
  "data": {
    "order_id": "ord_abc123",
    "status": "shipped",
    "tracking_number": "TRK123456",
    "carrier": "UPS",
    "estimated_delivery": "2024-01-18"
  }
}
```

---

## Inventory Service

**Base Path:** `/api/inventory`

### 6. Get Product Stock

```http
GET /api/inventory/stock/{product_id}
```

**Response (200):**

```json
{
  "success": true,
  "data": {
    "product_id": "prod_001",
    "product_name": "Widget A",
    "total_quantity": 500,
    "total_reserved": 25,
    "available": 475,
    "warehouses": [
      {
        "warehouse_id": "wh_nyc",
        "warehouse_name": "New York Warehouse",
        "quantity": 200,
        "reserved": 10,
        "available": 190
      },
      {
        "warehouse_id": "wh_la",
        "warehouse_name": "Los Angeles Warehouse",
        "quantity": 300,
        "reserved": 15,
        "available": 285
      }
    ]
  }
}
```

---

### 7. List Inventory

```http
GET /api/inventory
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | 1 | Page number |
| `limit` | int | 20 | Items per page |
| `warehouse_id` | string | - | Filter by warehouse |
| `low_stock` | bool | - | Only low stock items |
| `search` | string | - | Search by product name/SKU |

**Response (200):**

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "product_id": "prod_001",
        "product_name": "Widget A",
        "sku": "WGT-001",
        "warehouse_id": "wh_nyc",
        "quantity": 200,
        "reserved": 10,
        "available": 190,
        "low_stock_threshold": 50,
        "is_low_stock": false
      }
    ],
    "pagination": {...}
  }
}
```

---

### 8. Update Stock

```http
PUT /api/inventory/stock
```

**Request:**

```json
{
  "product_id": "prod_001",
  "warehouse_id": "wh_nyc",
  "quantity": 250,
  "reason": "Restock shipment received"
}
```

**Response (200):**

```json
{
  "success": true,
  "data": {
    "product_id": "prod_001",
    "warehouse_id": "wh_nyc",
    "previous_quantity": 200,
    "new_quantity": 250,
    "movement_id": "mov_xyz123",
    "updated_at": "2024-01-15T12:00:00Z"
  }
}
```

---

### 9. Transfer Stock

```http
POST /api/inventory/transfer
```

**Request:**

```json
{
  "product_id": "prod_001",
  "from_warehouse_id": "wh_la",
  "to_warehouse_id": "wh_nyc",
  "quantity": 50,
  "reason": "Rebalance inventory"
}
```

**Response (201):**

```json
{
  "success": true,
  "data": {
    "transfer_id": "tfr_abc123",
    "product_id": "prod_001",
    "from_warehouse": {
      "warehouse_id": "wh_la",
      "previous_quantity": 300,
      "new_quantity": 250
    },
    "to_warehouse": {
      "warehouse_id": "wh_nyc",
      "previous_quantity": 200,
      "new_quantity": 250
    },
    "status": "in_transit",
    "created_at": "2024-01-15T12:30:00Z"
  }
}
```

---

### 10. Get Movement History

```http
GET /api/inventory/movements
```

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `product_id` | string | - | Filter by product |
| `warehouse_id` | string | - | Filter by warehouse |
| `type` | string | - | in/out/transfer |
| `from` | string | - | Start date |
| `to` | string | - | End date |
| `page` | int | 1 | Page number |
| `limit` | int | 20 | Items per page |

**Response (200):**

```json
{
  "success": true,
  "data": {
    "movements": [
      {
        "movement_id": "mov_xyz123",
        "product_id": "prod_001",
        "warehouse_id": "wh_nyc",
        "type": "in",
        "quantity": 50,
        "reference_id": "tfr_abc123",
        "reference_type": "transfer",
        "notes": "Transfer from LA warehouse",
        "created_by": "usr_mgr001",
        "created_at": "2024-01-15T12:30:00Z"
      }
    ],
    "pagination": {...}
  }
}
```

---

### 11. Set Low Stock Threshold

```http
PUT /api/inventory/threshold
```

**Request:**

```json
{
  "product_id": "prod_001",
  "warehouse_id": "wh_nyc",
  "threshold": 50
}
```

**Response (200):**

```json
{
  "success": true,
  "data": {
    "product_id": "prod_001",
    "warehouse_id": "wh_nyc",
    "low_stock_threshold": 50,
    "current_stock": 190,
    "is_low_stock": false
  }
}
```

---

## WebSocket Service

**Connection:** `wss://{domain}/ws`

### Authentication

```
wss://domain.com/ws?token=<jwt_token>
```

### Subscribe to Channels

**Request:**

```json
{
  "action": "subscribe",
  "channels": [
    "order:ord_abc123",
    "inventory:prod_001",
    "inventory:warehouse:wh_nyc"
  ]
}
```

### Unsubscribe

```json
{
  "action": "unsubscribe",
  "channels": ["order:ord_abc123"]
}
```

### Events Received

#### Order Status Update

```json
{
  "event": "order.status_changed",
  "data": {
    "order_id": "ord_abc123",
    "status": "shipped",
    "previous_status": "processing",
    "tracking_number": "TRK123456",
    "updated_at": "2024-01-15T14:00:00Z"
  }
}
```

#### Inventory Update

```json
{
  "event": "inventory.updated",
  "data": {
    "product_id": "prod_001",
    "warehouse_id": "wh_nyc",
    "previous_quantity": 200,
    "new_quantity": 190,
    "available": 180,
    "updated_at": "2024-01-15T14:05:00Z"
  }
}
```

#### Low Stock Alert

```json
{
  "event": "inventory.low_stock",
  "data": {
    "product_id": "prod_001",
    "product_name": "Widget A",
    "warehouse_id": "wh_nyc",
    "current_stock": 45,
    "threshold": 50,
    "alert_at": "2024-01-15T14:10:00Z"
  }
}
```

### Heartbeat

**Server Ping (every 30s):**

```json
{
  "event": "ping"
}
```

**Client Pong:**

```json
{
  "event": "pong"
}
```

---

## Error Response Format

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "details": {
      "field": "Additional context"
    }
  },
  "request_id": "req_uuid"
}
```

### Standard Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Bad request format |
| `UNAUTHORIZED` | 401 | Invalid or missing token |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `CONFLICT` | 409 | Resource conflict |
| `RATE_LIMITED` | 429 | Rate limit exceeded |
| `INTERNAL_ERROR` | 500 | Server error |
| `SERVICE_UNAVAILABLE` | 503 | Downstream service unavailable |

---

## Rate Limits

| Endpoint | Limit | Window |
|----------|-------|--------|
| `POST /api/orders` | 10 req | 1 min |
| `GET /api/orders` | 100 req | 1 min |
| `GET /api/inventory` | 100 req | 1 min |
| `PUT /api/inventory/*` | 30 req | 1 min |
| WebSocket connections | 5 per user | - |

**Headers:**

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1705320000
```

---

## Pagination

**Request:**

```
GET /api/orders?page=2&limit=20
```

**Response:**

```json
{
  "pagination": {
    "current_page": 2,
    "total_pages": 10,
    "total_items": 195,
    "items_per_page": 20,
    "has_next": true,
    "has_previous": true
  }
}
```

---

## Versioning

API version is included in the URL path:

```
/api/v1/orders
/api/v2/orders
```

Deprecated versions return header:

```
Sunset: Sat, 01 Jun 2025 00:00:00 GMT
Deprecation: true
Link: </api/v2/orders>; rel="successor-version"
```
