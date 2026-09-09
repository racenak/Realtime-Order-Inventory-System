CREATE TABLE order_status_history (
    id UUID PRIMARY KEY,

    order_id UUID NOT NULL,

    old_status VARCHAR(30),
    new_status VARCHAR(30) NOT NULL,

    reason VARCHAR(255),

    created_at TIMESTAMP NOT NULL,

    FOREIGN KEY (order_id) REFERENCES orders(id)
);