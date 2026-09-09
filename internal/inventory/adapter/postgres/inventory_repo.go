package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/domain"
)

type inventoryRepository struct {
	db *sqlx.DB
}

func NewInventoryRepository(db *sqlx.DB) domain.InventoryRepository {
	return &inventoryRepository{db: db}
}

func (r *inventoryRepository) GetByProductAndWarehouse(ctx context.Context, productID, warehouseID string) (*domain.Inventory, error) {
	var inv domain.Inventory
	query := `SELECT * FROM inventory WHERE product_id = $1 AND warehouse_id = $2`

	err := r.db.GetContext(ctx, &inv, query, productID, warehouseID)
	if err == sql.ErrNoRows {
		return nil, domain.ErrInventoryNotFound
	}
	if err != nil {
		return nil, err
	}

	return &inv, nil
}

func (r *inventoryRepository) GetByProductID(ctx context.Context, productID string) ([]*domain.Inventory, error) {
	var inventories []*domain.Inventory
	query := `SELECT * FROM inventory WHERE product_id = $1`

	err := r.db.SelectContext(ctx, &inventories, query, productID)
	if err != nil {
		return nil, err
	}

	return inventories, nil
}

func (r *inventoryRepository) UpdateStock(ctx context.Context, productID, warehouseID string, quantityChange int, expectedVersion int64) error {
	query := `
		UPDATE inventory
		SET quantity_on_hand = quantity_on_hand + $1,
		    version = version + 1,
		    updated_at = $2
		WHERE product_id = $3 AND warehouse_id = $4 AND version = $5`

	result, err := r.db.ExecContext(ctx, query, quantityChange, time.Now(), productID, warehouseID, expectedVersion)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return domain.ErrConcurrentModification
	}

	return nil
}

type reservationRepository struct {
	db *sqlx.DB
}

func NewReservationRepository(db *sqlx.DB) domain.ReservationRepository {
	return &reservationRepository{db: db}
}

func (r *reservationRepository) Create(ctx context.Context, reservation *domain.Reservation) error {
	query := `
		INSERT INTO inventory_reservations (id, order_id, order_item_id, product_id, sku, warehouse_id, quantity, status, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err := r.db.ExecContext(ctx, query,
		reservation.ID,
		reservation.OrderID,
		reservation.OrderItemID,
		reservation.ProductID,
		reservation.SKU,
		reservation.WarehouseID,
		reservation.Quantity,
		reservation.Status,
		reservation.ExpiresAt,
		reservation.CreatedAt,
		reservation.UpdatedAt,
	)

	return err
}

func (r *reservationRepository) GetByID(ctx context.Context, id string) (*domain.Reservation, error) {
	var reservation domain.Reservation
	query := `SELECT * FROM inventory_reservations WHERE id = $1`

	err := r.db.GetContext(ctx, &reservation, query, id)
	if err == sql.ErrNoRows {
		return nil, domain.ErrReservationNotFound
	}
	if err != nil {
		return nil, err
	}

	return &reservation, nil
}

func (r *reservationRepository) GetByOrderID(ctx context.Context, orderID string) ([]*domain.Reservation, error) {
	var reservations []*domain.Reservation
	query := `SELECT * FROM inventory_reservations WHERE order_id = $1`

	err := r.db.SelectContext(ctx, &reservations, query, orderID)
	if err != nil {
		return nil, err
	}

	return reservations, nil
}

func (r *reservationRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	query := `UPDATE inventory_reservations SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	return err
}

type movementRepository struct {
	db *sqlx.DB
}

func NewMovementRepository(db *sqlx.DB) domain.MovementRepository {
	return &movementRepository{db: db}
}

func (r *movementRepository) Create(ctx context.Context, movement *domain.InventoryMovement) error {
	query := `
		INSERT INTO inventory_movements (id, product_id, sku, warehouse_id, movement_type, quantity, reference_type, reference_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.ExecContext(ctx, query,
		movement.ID,
		movement.ProductID,
		movement.SKU,
		movement.WarehouseID,
		movement.MovementType,
		movement.Quantity,
		movement.ReferenceType,
		movement.ReferenceID,
		movement.CreatedAt,
	)

	return err
}

func (r *movementRepository) GetByProductID(ctx context.Context, productID string) ([]*domain.InventoryMovement, error) {
	var movements []*domain.InventoryMovement
	query := `SELECT * FROM inventory_movements WHERE product_id = $1 ORDER BY created_at DESC`

	err := r.db.SelectContext(ctx, &movements, query, productID)
	if err != nil {
		return nil, err
	}

	return movements, nil
}
