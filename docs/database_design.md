# Database Design

## Overview

| Property     | Value                   |
| ------------ | ----------------------- |
| Database     | PostgreSQL 18          |
| Architecture | Database per Service    |
| ID Strategy  | UUID v7 (time-sortable) |
| Consistency  | Strong (ACID)           |

## Databases

| Database         | Service           | Purpose                        |
| ---------------- | ----------------- | ------------------------------ |
| `order_db`     | Order Service     | Orders, payments, order events |
| `inventory_db` | Inventory Service | Stock, warehouses, movements   |

---

## Entity Relationship Diagram

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              ORDER DATABASE                                     │
│                                                                                 │
│  ┌──────────────┐       ┌──────────────────┐       ┌───────────────────────┐  │
│  │    orders     │──────▶│   order_items     │       │   order_status_history │  │
│  └──────────────┘       └──────────────────┘       └───────────────────────┘  │
│           │                                                                │  │
│           │              ┌──────────────────┐                               │  │
│           └─────────────▶│  outbox_events    │                               │  │
│                          └──────────────────┘                               │  │
└─────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────┐
│                           INVENTORY DATABASE                                    │
│                                                                                 │
│  ┌──────────────┐       ┌──────────────────┐       ┌───────────────────────┐  │
│  │  warehouses   │──────▶│    inventory      │──────▶│ inventory_reservations │  │
│  └──────────────┘       └──────────────────┘       └───────────────────────┘  │
│                                  │                                            │
│                                  ▼                                            │
│                          ┌──────────────────┐                                  │
│                          │ inventory_movements│                                 │
│                          └──────────────────┘                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Order Database

### 1. orders

Main table storing order records.

| Column               | Type          | Nullable | Description               |
| -------------------- | ------------- | -------- | ------------------------- |
| `id`               | UUID          | NO       | Primary key (UUID v7)     |
| `customer_id`      | UUID          | NO       | Reference to user service |
| `status`           | VARCHAR(30)   | NO       | Order status              |
| `currency`         | CHAR(3)       | NO       | ISO 4217 currency code    |
| `subtotal`         | DECIMAL(18,2) | NO       | Sum of item totals        |
| `discount_amount`  | DECIMAL(18,2) | NO       | Total discount applied    |
| `shipping_fee`     | DECIMAL(18,2) | NO       | Shipping cost             |
| `tax_amount`       | DECIMAL(18,2) | NO       | Tax amount                |
| `total_amount`     | DECIMAL(18,2) | NO       | Final amount              |
| `shipping_address` | JSONB         | NO       | Delivery address          |
| `idempotency_key`  | VARCHAR(100)  | YES      | Prevent duplicate orders  |
| `created_at`       | TIMESTAMPTZ   | NO       | Creation timestamp        |
| `updated_at`       | TIMESTAMPTZ   | NO       | Last update timestamp     |

**Status Values:**

```
pending_payment → paid → processing → shipped → delivered
      ↓                      ↓
   cancelled              cancelled
```

**shipping_address JSON Structure:**

```json
{
  "street": "123 Main St",
  "city": "New York",
  "state": "NY",
  "zip": "10001",
  "country": "US"
}
```

---

### 2. order_items

Line items belonging to an order.

| Column              | Type          | Nullable | Description                       |
| ------------------- | ------------- | -------- | --------------------------------- |
| `id`              | UUID          | NO       | Primary key                       |
| `order_id`        | UUID          | NO       | FK → orders.id                   |
| `product_id`      | UUID          | NO       | Reference to product              |
| `sku`             | VARCHAR(100)  | NO       | Product SKU                       |
| `product_name`    | VARCHAR(255)  | NO       | Snapshot of product name          |
| `quantity`        | INT           | NO       | Ordered quantity                  |
| `unit_price`      | DECIMAL(18,2) | NO       | Price at time of order            |
| `discount_amount` | DECIMAL(18,2) | NO       | Item discount                     |
| `total_amount`    | DECIMAL(18,2) | NO       | quantity × unit_price - discount |
| `created_at`      | TIMESTAMPTZ   | NO       | Creation timestamp                |

---

### 3. order_status_history

Audit trail of order status changes.

| Column         | Type         | Nullable | Description                      |
| -------------- | ------------ | -------- | -------------------------------- |
| `id`         | UUID         | NO       | Primary key                      |
| `order_id`   | UUID         | NO       | FK → orders.id                  |
| `old_status` | VARCHAR(30)  | YES      | Previous status (NULL for first) |
| `new_status` | VARCHAR(30)  | NO       | New status                       |
| `reason`     | VARCHAR(255) | YES      | Reason for change                |
| `created_at` | TIMESTAMPTZ  | NO       | When change occurred             |

---

### 4. outbox_events

Events pending publication to Kafka (Outbox Pattern).

| Column             | Type         | Nullable | Description                  |
| ------------------ | ------------ | -------- | ---------------------------- |
| `id`             | UUID         | NO       | Primary key                  |
| `aggregate_type` | VARCHAR(50)  | NO       | e.g., "order", "inventory"   |
| `aggregate_id`   | UUID         | NO       | ID of the aggregate          |
| `event_type`     | VARCHAR(100) | NO       | e.g., "order.created"        |
| `payload`        | JSONB        | NO       | Full event payload           |
| `status`         | VARCHAR(20)  | NO       | PENDING / PUBLISHED / FAILED |
| `created_at`     | TIMESTAMPTZ  | NO       | When event was created       |
| `published_at`   | TIMESTAMPTZ  | YES      | When event was published     |

**Event Types:**

```
order.created
order.paid
order.confirmed
order.cancelled
order.shipped
order.delivered
```

---

## Inventory Database

### 5. warehouses

Warehouse locations.

| Column         | Type         | Nullable | Description           |
| -------------- | ------------ | -------- | --------------------- |
| `id`         | UUID         | NO       | Primary key           |
| `code`       | VARCHAR(50)  | NO       | Unique warehouse code |
| `name`       | VARCHAR(255) | NO       | Warehouse name        |
| `status`     | VARCHAR(20)  | NO       | active / inactive     |
| `created_at` | TIMESTAMPTZ  | NO       | Creation timestamp    |

---

### 6. inventory

Current stock levels per product per warehouse.

| Column                | Type         | Nullable | Description                 |
| --------------------- | ------------ | -------- | --------------------------- |
| `id`                | UUID         | NO       | Primary key                 |
| `product_id`        | UUID         | NO       | Reference to product        |
| `sku`               | VARCHAR(100) | NO       | Product SKU                 |
| `warehouse_id`      | UUID         | NO       | FK → warehouses.id         |
| `quantity_on_hand`  | INT          | NO       | Physical stock count        |
| `quantity_reserved` | INT          | NO       | Reserved for pending orders |
| `version`           | BIGINT       | NO       | Optimistic locking version  |
| `updated_at`        | TIMESTAMPTZ  | NO       | Last update timestamp       |

**Constraints:**

- UNIQUE(product_id, warehouse_id)
- quantity_on_hand >= 0
- quantity_reserved >= 0
- quantity_reserved <= quantity_on_hand

**Available Stock Formula:**

```sql
available = quantity_on_hand - quantity_reserved
```

---

### 7. inventory_reservations

Tracks reserved stock for pending orders.

| Column            | Type         | Nullable | Description             |
| ----------------- | ------------ | -------- | ----------------------- |
| `id`            | UUID         | NO       | Primary key             |
| `order_id`      | UUID         | NO       | Reference to order      |
| `order_item_id` | UUID         | NO       | Reference to order item |
| `product_id`    | UUID         | NO       | Reference to product    |
| `sku`           | VARCHAR(100) | NO       | Product SKU             |
| `warehouse_id`  | UUID         | NO       | FK → warehouses.id     |
| `quantity`      | INT          | NO       | Reserved quantity       |
| `status`        | VARCHAR(30)  | NO       | Reservation status      |
| `expires_at`    | TIMESTAMPTZ  | YES      | Reservation expiration  |
| `created_at`    | TIMESTAMPTZ  | NO       | Creation timestamp      |
| `updated_at`    | TIMESTAMPTZ  | NO       | Last update timestamp   |

**Status Values:**

```
reserved → confirmed → fulfilled
    ↓
  expired / released
```

---

### 8. inventory_movements

Audit trail of all inventory changes.

| Column             | Type         | Nullable | Description               |
| ------------------ | ------------ | -------- | ------------------------- |
| `id`             | UUID         | NO       | Primary key               |
| `product_id`     | UUID         | NO       | Reference to product      |
| `sku`            | VARCHAR(100) | NO       | Product SKU               |
| `warehouse_id`   | UUID         | NO       | FK → warehouses.id       |
| `movement_type`  | VARCHAR(30)  | NO       | Type of movement          |
| `quantity`       | INT          | NO       | Quantity changed (+/-)    |
| `reference_type` | VARCHAR(50)  | YES      | e.g., "order", "transfer" |
| `reference_id`   | UUID         | YES      | ID of related entity      |
| `created_at`     | TIMESTAMPTZ  | NO       | When movement occurred    |

**Movement Types:**

| Type             | Quantity | Description                     |
| ---------------- | -------- | ------------------------------- |
| `in`           | +        | Stock received                  |
| `out`          | -        | Stock shipped                   |
| `reserve`      | -        | Reserved for order              |
| `release`      | +        | Reservation released            |
| `adjustment`   | +/-      | Manual adjustment               |
| `transfer_in`  | +        | Received from another warehouse |
| `transfer_out` | -        | Sent to another warehouse       |

---

## Indexes

### Order Database

```sql
-- orders
CREATE INDEX idx_orders_customer_id ON orders(customer_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);
CREATE INDEX idx_orders_idempotency_key ON orders(idempotency_key) WHERE idempotency_key IS NOT NULL;

-- order_items
CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_product_id ON order_items(product_id);

-- order_status_history
CREATE INDEX idx_order_status_history_order_id ON order_status_history(order_id);
CREATE INDEX idx_order_status_history_created_at ON order_status_history(created_at DESC);

-- outbox_events
CREATE INDEX idx_outbox_events_status ON outbox_events(status) WHERE status = 'PENDING';
CREATE INDEX idx_outbox_events_created_at ON outbox_events(created_at);
```

### Inventory Database

```sql
-- warehouses
CREATE INDEX idx_warehouses_status ON warehouses(status);

-- inventory
CREATE INDEX idx_inventory_product_id ON inventory(product_id);
CREATE INDEX idx_inventory_warehouse_id ON inventory(warehouse_id);
CREATE INDEX idx_inventory_low_stock ON inventory(quantity_on_hand - quantity_reserved)
    WHERE (quantity_on_hand - quantity_reserved) < 50;

-- inventory_reservations
CREATE INDEX idx_inventory_reservations_order_id ON inventory_reservations(order_id);
CREATE INDEX idx_inventory_reservations_status ON inventory_reservations(status);
CREATE INDEX idx_inventory_reservations_expires_at ON inventory_reservations(expires_at)
    WHERE status = 'reserved';

-- inventory_movements
CREATE INDEX idx_inventory_movements_product_id ON inventory_movements(product_id);
CREATE INDEX idx_inventory_movements_warehouse_id ON inventory_movements(warehouse_id);
CREATE INDEX idx_inventory_movements_created_at ON inventory_movements(created_at DESC);
CREATE INDEX idx_inventory_movements_reference ON inventory_movements(reference_type, reference_id);
```

---

## Constraints

### Order Database

```sql
-- orders
ALTER TABLE orders ADD CONSTRAINT chk_orders_status
    CHECK (status IN ('pending_payment', 'paid', 'processing', 'shipped', 'delivered', 'cancelled'));

ALTER TABLE orders ADD CONSTRAINT chk_orders_amounts
    CHECK (subtotal >= 0 AND discount_amount >= 0 AND tax_amount >= 0 AND total_amount >= 0);

-- order_items
ALTER TABLE order_items ADD CONSTRAINT chk_order_items_quantity
    CHECK (quantity > 0);

ALTER TABLE order_items ADD CONSTRAINT chk_order_items_price
    CHECK (unit_price >= 0 AND total_amount >= 0);

-- outbox_events
ALTER TABLE outbox_events ADD CONSTRAINT chk_outbox_status
    CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED'));
```

### Inventory Database

```sql
-- inventory
ALTER TABLE inventory ADD CONSTRAINT chk_inventory_quantity
    CHECK (quantity_on_hand >= 0);

ALTER TABLE inventory ADD CONSTRAINT chk_inventory_reserved
    CHECK (quantity_reserved >= 0);

ALTER TABLE inventory ADD CONSTRAINT chk_inventory_reserved_lte_on_hand
    CHECK (quantity_reserved <= quantity_on_hand);

-- inventory_reservations
ALTER TABLE inventory_reservations ADD CONSTRAINT chk_reservation_status
    CHECK (status IN ('reserved', 'confirmed', 'fulfilled', 'expired', 'released'));

ALTER TABLE inventory_reservations ADD CONSTRAINT chk_reservation_quantity
    CHECK (quantity > 0);
```

---

## Stored Procedures

### Reserve Inventory

```sql
CREATE OR REPLACE PROCEDURE reserve_inventory(
    p_order_id UUID,
    p_order_item_id UUID,
    p_product_id UUID,
    p_warehouse_id UUID,
    p_quantity INT
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_available INT;
BEGIN
    -- Check available stock with lock
    SELECT (quantity_on_hand - quantity_reserved) INTO v_available
    FROM inventory
    WHERE product_id = p_product_id AND warehouse_id = p_warehouse_id
    FOR UPDATE;

    IF v_available < p_quantity THEN
        RAISE EXCEPTION 'Insufficient stock';
    END IF;

    -- Update inventory
    UPDATE inventory
    SET quantity_reserved = quantity_reserved + p_quantity,
        version = version + 1,
        updated_at = NOW()
    WHERE product_id = p_product_id AND warehouse_id = p_warehouse_id;

    -- Create reservation
    INSERT INTO inventory_reservations (
        id, order_id, order_item_id, product_id, sku, warehouse_id,
        quantity, status, expires_at, created_at, updated_at
    ) VALUES (
        uuidv7(), p_order_id, p_order_item_id, p_product_id, '',
        p_warehouse_id, p_quantity, 'reserved', NOW() + INTERVAL '15 minutes',
        NOW(), NOW()
    );

    -- Log movement
    INSERT INTO inventory_movements (
        id, product_id, sku, warehouse_id, movement_type,
        quantity, reference_type, reference_id, created_at
    ) VALUES (
        uuidv7(), p_product_id, '', p_warehouse_id, 'reserve',
        -p_quantity, 'order', p_order_id, NOW()
    );
END;
$$;
```

### Release Reservation

```sql
CREATE OR REPLACE PROCEDURE release_reservation(
    p_reservation_id UUID
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_reservation RECORD;
BEGIN
    -- Get reservation
    SELECT * INTO v_reservation
    FROM inventory_reservations
    WHERE id = p_reservation_id AND status = 'reserved'
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Reservation not found';
    END IF;

    -- Update inventory
    UPDATE inventory
    SET quantity_reserved = quantity_reserved - v_reservation.quantity,
        version = version + 1,
        updated_at = NOW()
    WHERE product_id = v_reservation.product_id
      AND warehouse_id = v_reservation.warehouse_id;

    -- Update reservation status
    UPDATE inventory_reservations
    SET status = 'released', updated_at = NOW()
    WHERE id = p_reservation_id;

    -- Log movement
    INSERT INTO inventory_movements (
        id, product_id, sku, warehouse_id, movement_type,
        quantity, reference_type, reference_id, created_at
    ) VALUES (
        uuidv7(), v_reservation.product_id, v_reservation.sku,
        v_reservation.warehouse_id, 'release',
        v_reservation.quantity, 'order', v_reservation.order_id, NOW()
    );
END;
$$;
```

### Confirm Order (Deduct Stock)

```sql
CREATE OR REPLACE PROCEDURE confirm_order_stock(
    p_order_id UUID
)
LANGUAGE plpgsql
AS $$
DECLARE
    v_reservation RECORD;
BEGIN
    FOR v_reservation IN
        SELECT * FROM inventory_reservations
        WHERE order_id = p_order_id AND status = 'reserved'
    LOOP
        -- Deduct from on_hand
        UPDATE inventory
        SET quantity_on_hand = quantity_on_hand - v_reservation.quantity,
            version = version + 1,
            updated_at = NOW()
        WHERE product_id = v_reservation.product_id
          AND warehouse_id = v_reservation.warehouse_id;

        -- Update reservation status
        UPDATE inventory_reservations
        SET status = 'fulfilled', updated_at = NOW()
        WHERE id = v_reservation.id;

        -- Log movement
        INSERT INTO inventory_movements (
            id, product_id, sku, warehouse_id, movement_type,
            quantity, reference_type, reference_id, created_at
        ) VALUES (
            uuidv7(), v_reservation.product_id, v_reservation.sku,
            v_reservation.warehouse_id, 'out',
            -v_reservation.quantity, 'order', p_order_id, NOW()
        );
    END LOOP;
END;
$$;
```

---

## Migration Strategy

| Tool           | Purpose           |
| -------------- | ----------------- |
| Raw SQL files  | Schema init via Docker entrypoint |

**Directory Structure:**

```
migrations/
├── init.sql                          # Combined schema (both DBs)
├── order/
│   ├── 01-init.sql                   # Full order schema + uuidv7()
│   ├── orders.sql                    # orders table only
│   ├── order_items.sql               # order_items table only
│   ├── order_status_history.sql      # order_status_history table only
│   └── outbox_events.sql             # outbox_events table only
└── inventory/
    ├── 01-init.sql                   # Full inventory schema + uuidv7()
    ├── warehouses.sql                # warehouses table only
    ├── inventory.sql                 # inventory table only
    ├── inventory_reservations.sql    # inventory_reservations table only
    └── inventory_movements.sql       # inventory_movements table only
```

**How it works:**

- `cmd/order-db/Dockerfile` copies `migrations/order/01-init.sql` → `/docker-entrypoint-initdb.d/`
- `cmd/inventory-db/Dockerfile` copies `migrations/inventory/01-init.sql` → `/docker-entrypoint-initdb.d/`
- PostgreSQL runs `01-init.sql` automatically on first container start
- Individual table files (e.g., `orders.sql`) are for reference/maintenance only

---

## Data Types Reference

| Type          | Size     | Range    | Use Case            |
| ------------- | -------- | -------- | ------------------- |
| UUID          | 16 bytes | -        | Primary keys        |
| VARCHAR(100)  | -        | -        | SKU, codes          |
| VARCHAR(255)  | -        | -        | Names               |
| VARCHAR(30)   | -        | -        | Status fields       |
| INT           | 4 bytes  | ±2.1B   | Quantities          |
| BIGINT        | 8 bytes  | ±9.2E18 | Version counters    |
| DECIMAL(18,2) | -        | -        | Money amounts       |
| TIMESTAMP     | 8 bytes  | -        | Dates/times (with tz)   |
| JSONB         | -        | -        | Flexible structures |

---

## Backup Strategy

| Database     | Frequency | Retention | Method                  |
| ------------ | --------- | --------- | ----------------------- |
| order_db     | Daily     | 30 days   | pg_dump + WAL archiving |
| inventory_db | Daily     | 30 days   | pg_dump + WAL archiving |

**Point-in-time Recovery:** Enabled via WAL streaming.
