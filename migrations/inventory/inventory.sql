CREATE TABLE IF NOT EXISTS inventory (
    id UUID PRIMARY KEY DEFAULT uuidv7(),

    product_id UUID NOT NULL,
    sku VARCHAR(100) NOT NULL,

    warehouse_id UUID NOT NULL,

    quantity_on_hand INT NOT NULL DEFAULT 0,
    quantity_reserved INT NOT NULL DEFAULT 0,

    version BIGINT NOT NULL DEFAULT 0,

    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(product_id, warehouse_id),

    FOREIGN KEY (warehouse_id) REFERENCES warehouses(id)
);

CREATE INDEX IF NOT EXISTS idx_inventory_product_id ON inventory(product_id);
CREATE INDEX IF NOT EXISTS idx_inventory_warehouse_id ON inventory(warehouse_id);
CREATE INDEX IF NOT EXISTS idx_inventory_low_stock ON inventory(quantity_on_hand - quantity_reserved)
    WHERE (quantity_on_hand - quantity_reserved) < 50;