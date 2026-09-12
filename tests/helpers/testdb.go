package helpers

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func SetupTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	host := getEnv("TEST_DB_HOST", "localhost")
	port := getEnv("TEST_DB_PORT", "5432")
	user := getEnv("TEST_DB_USER", "postgres")
	pass := getEnv("TEST_DB_PASSWORD", "postgres")
	dbname := getEnv("TEST_DB_NAME", "order_inventory")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, pass, dbname,
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	db.SetMaxOpenConns(5)

	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping test database: %v", err)
	}

	// Create tables if they don't exist
	createTables(t, db)

	CleanDatabase(t, db)

	t.Cleanup(func() {
		CleanDatabase(t, db)
		_ = db.Close()
	})

	return db
}

func createTables(t *testing.T, db *sqlx.DB) {
	t.Helper()

	schema := `
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

	CREATE OR REPLACE FUNCTION uuidv7() RETURNS uuid
		LANGUAGE plpgsql
		PARALLEL SAFE
		AS $$
	DECLARE
		unix_ts_ms bytea;
		uuid_bytes bytea;
	BEGIN
		unix_ts_ms = substring(
			int8send(floor(extract(epoch from clock_timestamp()) * 1000)::bigint) from 3
		);
		uuid_bytes = unix_ts_ms || gen_random_bytes(10);
		uuid_bytes = set_byte(uuid_bytes, 6, (b'0111' || get_byte(uuid_bytes, 6)::bit(4))::bit(8)::byte);
		uuid_bytes = set_byte(uuid_bytes, 8, (b'10' || get_byte(uuid_bytes, 8)::bit(6))::bit(8)::byte);
		return encode(uuid_bytes, 'hex')::uuid;
	END;
	$$;

	CREATE TABLE IF NOT EXISTS orders (
		id UUID PRIMARY KEY DEFAULT uuidv7(),
		customer_id UUID NOT NULL,
		status VARCHAR(30) NOT NULL,
		currency CHAR(3) NOT NULL DEFAULT 'USD',
		subtotal DECIMAL(18,2) NOT NULL DEFAULT 0,
		discount_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
		shipping_fee DECIMAL(18,2) NOT NULL DEFAULT 0,
		tax_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
		total_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
		shipping_address JSONB NOT NULL DEFAULT '{}',
		idempotency_key VARCHAR(100) UNIQUE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

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

	CREATE TABLE IF NOT EXISTS order_status_history (
		id UUID PRIMARY KEY DEFAULT uuidv7(),
		order_id UUID NOT NULL,
		old_status VARCHAR(30),
		new_status VARCHAR(30) NOT NULL,
		reason VARCHAR(255),
		created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS outbox_events (
		id UUID PRIMARY KEY DEFAULT uuidv7(),
		aggregate_type VARCHAR(50) NOT NULL,
		aggregate_id UUID NOT NULL,
		event_type VARCHAR(100) NOT NULL,
		payload JSONB NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
		created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		published_at TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS warehouses (
		id UUID PRIMARY KEY DEFAULT uuidv7(),
		code VARCHAR(50) UNIQUE NOT NULL,
		name VARCHAR(255) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'active',
		created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

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

	CREATE TABLE IF NOT EXISTS inventory_reservations (
		id UUID PRIMARY KEY DEFAULT uuidv7(),
		order_id UUID NOT NULL,
		order_item_id UUID,
		product_id UUID NOT NULL,
		sku VARCHAR(100) NOT NULL DEFAULT '',
		warehouse_id UUID NOT NULL,
		quantity INT NOT NULL,
		status VARCHAR(30) NOT NULL,
		expires_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS inventory_movements (
		id UUID PRIMARY KEY DEFAULT uuidv7(),
		product_id UUID NOT NULL,
		sku VARCHAR(100) NOT NULL DEFAULT '',
		warehouse_id UUID NOT NULL,
		movement_type VARCHAR(30) NOT NULL,
		quantity INT NOT NULL,
		reference_type VARCHAR(50),
		reference_id UUID,
		created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id);
	CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
	CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
	CREATE INDEX IF NOT EXISTS idx_order_items_product_id ON order_items(product_id);
	CREATE INDEX IF NOT EXISTS idx_order_status_history_order_id ON order_status_history(order_id);
	CREATE INDEX IF NOT EXISTS idx_outbox_events_status ON outbox_events(status) WHERE status = 'PENDING';
	CREATE INDEX IF NOT EXISTS idx_outbox_events_created_at ON outbox_events(created_at);
	CREATE INDEX IF NOT EXISTS idx_inventory_product_id ON inventory(product_id);
	CREATE INDEX IF NOT EXISTS idx_inventory_warehouse_id ON inventory(warehouse_id);
	CREATE INDEX IF NOT EXISTS idx_inventory_reservations_order_id ON inventory_reservations(order_id);
	CREATE INDEX IF NOT EXISTS idx_inventory_reservations_status ON inventory_reservations(status);
	CREATE INDEX IF NOT EXISTS idx_inventory_movements_product_id ON inventory_movements(product_id);
	CREATE INDEX IF NOT EXISTS idx_inventory_movements_warehouse_id ON inventory_movements(warehouse_id);
	CREATE INDEX IF NOT EXISTS idx_inventory_movements_created_at ON inventory_movements(created_at DESC);
	`

	_, err := db.Exec(schema)
	if err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}
}

func CleanDatabase(t *testing.T, db *sqlx.DB) {
	t.Helper()

	tables := []string{
		"inventory_movements",
		"inventory_reservations",
		"inventory",
		"order_status_history",
		"order_items",
		"outbox_events",
		"orders",
		"warehouses",
	}

	for _, table := range tables {
		_, _ = db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
	}
}

func SeedWarehouse(t *testing.T, db *sqlx.DB, id, code, name string) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO warehouses (id, code, name, status) 
		VALUES ($1, $2, $3, 'active')
		ON CONFLICT (code) DO NOTHING`,
		id, code, name,
	)
	if err != nil {
		t.Fatalf("Failed to seed warehouse: %v", err)
	}
}

func SeedInventory(t *testing.T, db *sqlx.DB, id, productID, sku, warehouseID string, quantity int) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO inventory (id, product_id, sku, warehouse_id, quantity_on_hand, quantity_reserved, version)
		VALUES ($1, $2, $3, $4, $5, 0, 0)
		ON CONFLICT (product_id, warehouse_id) DO UPDATE SET quantity_on_hand = $5`,
		id, productID, sku, warehouseID, quantity,
	)
	if err != nil {
		t.Fatalf("Failed to seed inventory: %v", err)
	}
}

func GetOrderCount(t *testing.T, db *sqlx.DB) int {
	t.Helper()

	var count int
	err := db.Get(&count, "SELECT COUNT(*) FROM orders")
	if err != nil {
		t.Fatalf("Failed to get order count: %v", err)
	}
	return count
}

func GetInventoryQuantity(t *testing.T, db *sqlx.DB, productID, warehouseID string) int {
	t.Helper()

	var quantity int
	err := db.Get(&quantity,
		"SELECT quantity_on_hand FROM inventory WHERE product_id = $1 AND warehouse_id = $2",
		productID, warehouseID,
	)
	if err != nil {
		t.Fatalf("Failed to get inventory quantity: %v", err)
	}
	return quantity
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func CleanupRows(t *testing.T, db *sqlx.DB, query string, args ...interface{}) {
	t.Helper()
	_, _ = db.Exec(query, args...)
}

func TestUUID() string {
	return "550e8400-e29b-41d4-a716-446655440000"
}

func SetupTestDBConnection(t *testing.T) *sql.DB {
	t.Helper()

	host := getEnv("TEST_DB_HOST", "localhost")
	port := getEnv("TEST_DB_PORT", "5432")
	user := getEnv("TEST_DB_USER", "postgres")
	pass := getEnv("TEST_DB_PASSWORD", "postgres")
	dbname := getEnv("TEST_DB_NAME", "order_inventory")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, pass, dbname,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping test database: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}
