CREATE TABLE inventory_movements (
    id UUID PRIMARY KEY,

    product_id UUID NOT NULL,
    sku VARCHAR(100) NOT NULL,

    warehouse_id UUID NOT NULL,

    movement_type VARCHAR(30) NOT NULL,

    quantity INT NOT NULL,

    reference_type VARCHAR(50),
    reference_id UUID,

    created_at TIMESTAMP NOT NULL
);