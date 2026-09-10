CREATE TABLE IF NOT EXISTS order_items (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    order_id UUID NOT NULL,

    product_id UUID NOT NULL,
    sku VARCHAR(100) NOT NULL,

    product_name VARCHAR(255) NOT NULL DEFAULT '',

    quantity INT NOT NULL,
    unit_price DECIMAL(18,2) NOT NULL,

    discount_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    total_amount DECIMAL(18,2) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
);