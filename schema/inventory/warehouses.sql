CREATE TABLE warehouses (
    id UUID PRIMARY KEY,

    code VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,

    status VARCHAR(20) NOT NULL,

    created_at TIMESTAMP NOT NULL
);