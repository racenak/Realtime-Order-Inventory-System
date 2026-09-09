CREATE TABLE inventory (
    id UUID PRIMARY KEY,

    product_id UUID NOT NULL,
    sku VARCHAR(100) NOT NULL,

    warehouse_id UUID NOT NULL,

    quantity_on_hand INT NOT NULL DEFAULT 0,
    quantity_reserved INT NOT NULL DEFAULT 0,

    version BIGINT NOT NULL DEFAULT 0,

    updated_at TIMESTAMP NOT NULL,

    UNIQUE(product_id, warehouse_id),

    FOREIGN KEY (warehouse_id)
        REFERENCES warehouses(id)
);