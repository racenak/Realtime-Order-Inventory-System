CREATE TABLE IF NOT EXISTS inventory_reservations (
    id UUID PRIMARY KEY DEFAULT uuidv7(),

    order_id UUID NOT NULL,
    order_item_id UUID NOT NULL,

    product_id UUID NOT NULL,
    sku VARCHAR(100) NOT NULL DEFAULT '',

    warehouse_id UUID NOT NULL,

    quantity INT NOT NULL,

    status VARCHAR(30) NOT NULL,

    expires_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);