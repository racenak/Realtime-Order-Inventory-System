CREATE TABLE order_items (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL,

    product_id UUID NOT NULL,
    sku VARCHAR(100) NOT NULL,

    product_name VARCHAR(255) NOT NULL,

    quantity INT NOT NULL,
    unit_price DECIMAL(18,2) NOT NULL,

    discount_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    total_amount DECIMAL(18,2) NOT NULL,

    created_at TIMESTAMP NOT NULL,

    FOREIGN KEY (order_id) REFERENCES orders(id)
);