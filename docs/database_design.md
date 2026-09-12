# Database Design

## Overview

Two separate PostgreSQL 18 databases:

| Database | Port | Services |
|----------|------|----------|
| `order_db` | 5432 | Order Service |
| `inventory_db` | 5433 | Inventory Service |

Schema SQL is baked into dedicated DB Dockerfiles (`database/order/Dockerfile`, `database/inventory/Dockerfile`).

---

## Order Database (`order_db`)

### Tables

#### `orders`

```sql
CREATE TABLE orders (
    id VARCHAR(26) PRIMARY KEY,
    customer_id VARCHAR(26) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    subtotal DECIMAL(12,2) NOT NULL DEFAULT 0,
    discount_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    shipping_fee DECIMAL(12,2) NOT NULL DEFAULT 0,
    tax_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    total_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    shipping_address JSONB,
    idempotency_key VARCHAR(26),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_orders_idempotency_key ON orders(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_orders_customer_id ON orders(customer_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created_at ON orders(created_at);
```

#### `order_items`

```sql
CREATE TABLE order_items (
    id VARCHAR(26) PRIMARY KEY,
    order_id VARCHAR(26) NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id VARCHAR(26) NOT NULL,
    sku VARCHAR(50) NOT NULL,
    product_name VARCHAR(255) NOT NULL,
    quantity INT NOT NULL CHECK (quantity > 0),
    unit_price DECIMAL(12,2) NOT NULL,
    discount_amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    total_amount DECIMAL(12,2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_product_id ON order_items(product_id);
```

#### `order_status_history`

```sql
CREATE TABLE order_status_history (
    id VARCHAR(26) PRIMARY KEY,
    order_id VARCHAR(26) NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    old_status VARCHAR(20),
    new_status VARCHAR(20) NOT NULL,
    reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_order_status_history_order_id ON order_status_history(order_id);
```

#### `outbox_events`

```sql
CREATE TABLE outbox_events (
    id VARCHAR(26) PRIMARY KEY,
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id VARCHAR(26) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    published_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_outbox_events_status ON outbox_events(status) WHERE status = 'PENDING';
CREATE INDEX idx_outbox_events_created_at ON outbox_events(created_at);
```

**Status flow**: `PENDING → CLAIMED → PUBLISHED/FAILED`

---

## Inventory Database (`inventory_db`)

### Tables

#### `warehouses`

```sql
CREATE TABLE warehouses (
    id VARCHAR(26) PRIMARY KEY,
    code VARCHAR(20) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_warehouses_code ON warehouses(code);
```

#### `inventory`

```sql
CREATE TABLE inventory (
    id VARCHAR(26) PRIMARY KEY,
    product_id VARCHAR(26) NOT NULL,
    sku VARCHAR(50) NOT NULL,
    warehouse_id VARCHAR(26) NOT NULL REFERENCES warehouses(id),
    quantity_on_hand INT NOT NULL DEFAULT 0 CHECK (quantity_on_hand >= 0),
    quantity_reserved INT NOT NULL DEFAULT 0 CHECK (quantity_reserved >= 0),
    version INT NOT NULL DEFAULT 1,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(product_id, warehouse_id)
);

CREATE INDEX idx_inventory_product_id ON inventory(product_id);
CREATE INDEX idx_inventory_warehouse_id ON inventory(warehouse_id);
CREATE INDEX idx_inventory_sku ON inventory(sku);
```

#### `inventory_reservations`

```sql
CREATE TABLE inventory_reservations (
    id VARCHAR(26) PRIMARY KEY,
    order_id VARCHAR(26) NOT NULL,
    order_item_id VARCHAR(26) NOT NULL,
    product_id VARCHAR(26) NOT NULL,
    sku VARCHAR(50) NOT NULL,
    warehouse_id VARCHAR(26) NOT NULL REFERENCES warehouses(id),
    quantity INT NOT NULL CHECK (quantity > 0),
    status VARCHAR(20) NOT NULL DEFAULT 'confirmed',
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_inventory_reservations_order_id ON inventory_reservations(order_id);
CREATE INDEX idx_inventory_reservations_product_id ON inventory_reservations(product_id);
CREATE INDEX idx_inventory_reservations_status ON inventory_reservations(status);
```

#### `inventory_movements`

```sql
CREATE TABLE inventory_movements (
    id VARCHAR(26) PRIMARY KEY,
    product_id VARCHAR(26) NOT NULL,
    sku VARCHAR(50) NOT NULL,
    warehouse_id VARCHAR(26) NOT NULL REFERENCES warehouses(id),
    movement_type VARCHAR(20) NOT NULL,
    quantity INT NOT NULL,
    reference_type VARCHAR(50),
    reference_id VARCHAR(26),
    notes TEXT,
    created_by VARCHAR(26),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_inventory_movements_product_id ON inventory_movements(product_id);
CREATE INDEX idx_inventory_movements_warehouse_id ON inventory_movements(warehouse_id);
CREATE INDEX idx_inventory_movements_reference ON inventory_movements(reference_type, reference_id);
```

---

## ID Generation

UUID v7 (time-ordered) generated via custom `uuidv7()` SQL function:

```sql
CREATE OR REPLACE FUNCTION uuidv7() RETURNS uuid
    LANGUAGE plpgsql
    AS $$
DECLARE
    unix_ts_ms bytea;
    uuid bytes;
BEGIN
    unix_ts_ms = substring(
        int8send(floor(extract(epoch from clock_timestamp()) * 1000)::bigint)
        from 3
    );
    uuid = unix_ts_ms || gen_random_bytes(10);
    uuid = set_bit(set_bit(uuid, 148, 0), 149, 0);
    uuid = set_bit(set_bit(uuid, 150, 0), 151, 1);
    uuid = set_bit(set_bit(uuid, 152, 0), 153, 0);
    uuid = set_bit(set_bit(uuid, 154, 0), 155, 0);
    uuid = set_bit(set_bit(uuid, 156, 0), 157, 1);
    uuid = set_bit(set_bit(uuid, 158, 0), 159, 0);
    RETURN encode(uuid, 'hex')::uuid;
END;
$$;
```

---

## What's Implemented

- [x] 2 separate PostgreSQL 18 databases
- [x] 8 tables total (4 per database)
- [x] uuidv7() for all primary keys
- [x] Unique index on `orders.idempotency_key`
- [x] Proper foreign key constraints
- [x] JSONB for `orders.shipping_address`
- [x] Optimistic locking via `inventory.version`
- [x] DB Dockerfiles with schema baked in
- [x] Ded DB Dockerfiles with schema baked in
