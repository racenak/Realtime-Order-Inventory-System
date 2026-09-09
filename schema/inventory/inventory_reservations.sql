CREATE TABLE inventory_reservations (
    id UUID PRIMARY KEY,

    order_id UUID NOT NULL,
    order_item_id UUID NOT NULL,

    product_id UUID NOT NULL,
    sku VARCHAR(100) NOT NULL,

    warehouse_id UUID NOT NULL,

    quantity INT NOT NULL,

    status VARCHAR(30) NOT NULL,

    expires_at TIMESTAMP,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);