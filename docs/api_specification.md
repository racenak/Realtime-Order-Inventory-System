# API Specification

**Base URL**: `http://localhost:8088` (Traefik reverse proxy)

## Authentication

All API requests require a valid JWT token in the `Authorization` header:

```
Authorization: Bearer <token>
```

Traefik forwards authentication to the Auth Service via ForwardAuth middleware. On valid JWT, downstream services receive `X-User-Id` and `X-User-Role` headers.

### JWT Validation Rules

| Rule | Value |
|------|-------|
| Algorithm | HMAC-SHA256 |
| Issuer | `order-inventory-system` |
| Audience | `order-inventory-api` |
| Expiry | Required |

### Rate Limiting

- **100 requests/second** per IP
- **50 burst** capacity

---

## Response Format

All responses use JSON. Successful operations return the resource directly. Errors follow a standard envelope:

```json
{
  "error": {
    "code": "ORDER_NOT_FOUND",
    "message": "order not found"
  }
}
```

### HTTP Status Codes

| Code | Meaning | When |
|------|---------|------|
| 200 | OK | GET requests, successful operations |
| 201 | Created | POST requests that create a resource |
| 400 | Bad Request | Invalid request body, missing fields |
| 404 | Not Found | Resource doesn't exist |
| 409 | Conflict | Duplicate idempotency key |
| 500 | Internal Server Error | Unexpected errors |

---

## Order Endpoints

### Create Order

```http
POST /api/orders
Content-Type: application/json
Idempotency-Key: unique-key-from-client

{
  "customer_id": "cust_01HXYZ123456",
  "items": [
    {
      "product_id": "prod_01HXYZ789012",
      "sku": "LAPTOP-001",
      "product_name": "Gaming Laptop",
      "quantity": 1,
      "unit_price": 1299.99
    }
  ],
  "shipping_address": {
    "street": "123 Main St",
    "city": "Portland",
    "state": "OR",
    "zip": "97201",
    "country": "US"
  }
}
```

**Idempotency:**
- Client sends `Idempotency-Key` header with a unique value
- If header is omitted, server auto-generates a UUID v4 key
- Order is looked up by idempotency key before insert (`GetByIDempotencyKey`)
- If key exists: returns `409 Conflict` with existing order in error body
- If key is new: inserts order, creates outbox event in same DB transaction

**Response (201):**
```json
{
  "id": "01HXYZ1234567890ABCDEF01",
  "customer_id": "cust_01HXYZ123456",
  "status": "pending",
  "currency": "USD",
  "subtotal": 1299.99,
  "total_amount": 1299.99,
  "created_at": "2026-09-09T10:30:00Z"
}
```

**Response (409):**
```json
{
  "error": {
    "code": "ORDER_ALREADY_EXISTS",
    "message": "duplicate idempotency key",
    "details": "existing order 01HXYZ9876543210FEDCBA98 returned"
  }
}
```

**Events Produced:**
- `order.created` (Topic: `order.created`)
- Stored in `outbox_events` table (status: PENDING) within DB transaction

### Get Order

```http
GET /api/orders/{id}
```

**Response (200):**
```json
{
  "id": "01HXYZ1234567890ABCDEF01",
  "customer_id": "cust_01HXYZ123456",
  "status": "pending",
  "currency": "USD",
  "subtotal": 1299.99,
  "total_amount": 1299.99,
  "items": [
    {
      "id": "item-uuid",
      "product_id": "prod_01HXYZ789012",
      "sku": "LAPTOP-001",
      "product_name": "Gaming Laptop",
      "quantity": 1,
      "unit_price": 1299.99,
      "total_amount": 1299.99
    }
  ],
  "created_at": "2026-09-09T10:30:00Z",
  "updated_at": "2026-09-09T10:30:00Z"
}
```

### List Orders

```http
GET /api/orders?customer_id=cust_01HXYZ123456&limit=20&offset=0
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| customer_id | string | (empty) | Filter by customer. If omitted, returns all orders. |
| limit | int | 20 | Max results per page (max 100) |
| offset | int | 0 | Pagination offset |

### Cancel Order

```http
POST /api/orders/{id}/cancel
```

Cancels a pending order. Cannot cancel orders with status `shipped` or `delivered`.

**Response (200):**
```json
{
  "id": "01HXYZ1234567890ABCDEF01",
  "status": "cancelled"
}
```

**Events Produced:**
- `order.cancelled` (Topic: `order.cancelled`)

---

## Inventory Endpoints

### Get Stock

```http
GET /api/inventory/stock/{product_id}
```

Returns stock for a product across all warehouses.

**Response (200):**
```json
{
  "product_id": "prod_01HXYZ789012",
  "warehouses": [
    {
      "warehouse_id": "wh_01HXYZ...",
      "warehouse_code": "MAIN",
      "quantity_on_hand": 50,
      "quantity_reserved": 5,
      "available": 45
    }
  ],
  "total_available": 45
}
```

### Reserve Stock

```http
POST /api/inventory/reserve
Content-Type: application/json

{
  "order_id": "01HXYZ1234567890ABCDEF01",
  "order_item_id": "item-uuid",
  "product_id": "prod_01HXYZ789012",
  "sku": "LAPTOP-001",
  "quantity": 2,
  "warehouse_code": "MAIN"
}
```

**Response (201):**
```json
{
  "reservation_id": "01HXYZ...",
  "status": "confirmed",
  "quantity": 2,
  "expires_at": "2026-09-09T11:00:00Z"
}
```

**Events Produced:**
- `inventory.reserved` (on success)
- `inventory.reservation_failed` (on insufficient stock)

### Release Reservation

```http
POST /api/inventory/release
Content-Type: application/json

{
  "reservation_id": "01HXYZ...",
  "order_id": "01HXYZ1234567890ABCDEF01"
}
```

**Events Produced:**
- `inventory.released`

### Update Stock

```http
PUT /api/inventory/stock
Content-Type: application/json

{
  "product_id": "prod_01HXYZ789012",
  "sku": "LAPTOP-001",
  "warehouse_code": "MAIN",
  "quantity": 50
}
```

Upserts inventory record: creates if not found for product/warehouse combo.

**Events Produced:**
- `inventory.updated`

---

## WebSocket Service

### Connect

```http
GET /ws?channels=order_id:01HXYZ...&channels=product_id:prod_01HXYZ789012
```

Query parameter `channels` specifies which event channels to subscribe to. Multiple channels can be specified.

### Subscribe to Order Updates

```http
GET /ws?channels=order_id:{order_id}
```

### Subscribe to Product Updates

```http
GET /ws?channels=product_id:{product_id}
```

### Subscribe to Inventory Updates

```http
GET /ws?channels=product_id:{product_id}
```

### Message Format

**Order Update:**
```json
{
  "type": "order_update",
  "data": {
    "order_id": "01HXYZ...",
    "status": "confirmed",
    "updated_at": "2026-09-09T10:35:00Z"
  }
}
```

**Inventory Update:**
```json
{
  "type": "inventory_update",
  "data": {
    "product_id": "prod_01HXYZ789012",
    "available": 45,
    "updated_at": "2026-09-09T10:35:00Z"
  }
}
```

### Heartbeat

WebSocket service sends ping frames every 54 seconds. Clients must respond with pong within 60 seconds or the connection is closed.

---

## Health Check

```http
GET /health
```

**Response (200):**
```json
{
  "status": "healthy",
  "service": "order-service"
}
```

---

## Metrics

```http
GET :9090/metrics  (Prometheus scraping endpoint)
```

Each Go service exposes `/metrics` via `promhttp.Handler()`.

**Custom Business Metrics (16 total):**

| Metric | Type | Description |
|--------|------|-------------|
| `orders_created_total` | Counter | Total orders created |
| `orders_completed_total` | Counter | Total orders completed |
| `orders_cancelled_total` | Counter | Total orders cancelled |
| `order_processing_duration_seconds` | Histogram | Order processing duration |
| `inventory_reservations_total` | Counter | Total reservations |
| `inventory_releases_total` | Counter | Total releases |
| `inventory_movements_total` | Counter | Total movements |
| `inventory_stock_level` | Gauge | Current stock level |
| `websocket_connections_active` | Gauge | Active WS connections |
| `websocket_messages_sent_total` | Counter | Total WS messages sent |
| `kafka_messages_published_total` | Counter | Total messages published |
| `kafka_messages_consumed_total` | Counter | Total messages consumed |
| `kafka_publish_errors_total` | Counter | Total publish errors |
| `kafka_dlq_messages_total` | Counter | Total DLQ messages |
| `http_requests_total` | Counter | Total HTTP requests |
| `http_request_duration_seconds` | Histogram | HTTP request duration |
