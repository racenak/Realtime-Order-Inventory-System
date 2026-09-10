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

func (r *inventoryRepository) Create(ctx context.Context, inventory *domain.Inventory) error {
	query := `
		INSERT INTO inventory (id, product_id, sku, warehouse_id, quantity_on_hand, quantity_reserved, version, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := r.db.ExecContext(ctx, query,
		inventory.ID,
		inventory.ProductID,
		inventory.SKU,
		inventory.WarehouseID,
		inventory.QuantityOnHand,
		inventory.QuantityReserved,
		inventory.Version,
		inventory.UpdatedAt,
	)

	return err
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

func (r *inventoryRepository) ReserveQuantity(ctx context.Context, productID, warehouseID string, quantity int, expectedVersion int64) error {
	query := `
		UPDATE inventory
		SET quantity_reserved = quantity_reserved + $1,
		    version = version + 1,
		    updated_at = $2
		WHERE product_id = $3 AND warehouse_id = $4 AND version = $5 AND (quantity_on_hand - quantity_reserved) >= $1`

	result, err := r.db.ExecContext(ctx, query, quantity, time.Now(), productID, warehouseID, expectedVersion)
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

func (r *inventoryRepository) ReleaseQuantity(ctx context.Context, productID, warehouseID string, quantity int) error {
	query := `
		UPDATE inventory
		SET quantity_reserved = quantity_reserved - $1,
		    updated_at = $2
		WHERE product_id = $3 AND warehouse_id = $4 AND quantity_reserved >= $1`

	_, err := r.db.ExecContext(ctx, query, quantity, time.Now(), productID, warehouseID)
	return err
}

func (r *inventoryRepository) GetWarehouseByCode(ctx context.Context, code string) (*domain.Warehouse, error) {
	var wh domain.Warehouse
	query := `SELECT * FROM warehouses WHERE code = $1`

	err := r.db.GetContext(ctx, &wh, query, code)
	if err == sql.ErrNoRows {
		return nil, domain.ErrInventoryNotFound
	}
	if err != nil {
		return nil, err
	}

	return &wh, nil
}

type reservationRow struct {
	ID          string         `db:"id"`
	OrderID     string         `db:"order_id"`
	OrderItemID sql.NullString `db:"order_item_id"`
	ProductID   string         `db:"product_id"`
	SKU         string         `db:"sku"`
	WarehouseID string         `db:"warehouse_id"`
	Quantity    int            `db:"quantity"`
	Status      string         `db:"status"`
	ExpiresAt   sql.NullTime   `db:"expires_at"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
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

	var orderItemID sql.NullString
	if reservation.OrderItemID != "" {
		orderItemID = sql.NullString{String: reservation.OrderItemID, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query,
		reservation.ID,
		reservation.OrderID,
		orderItemID,
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
	var row reservationRow
	query := `SELECT * FROM inventory_reservations WHERE id = $1`

	err := r.db.GetContext(ctx, &row, query, id)
	if err == sql.ErrNoRows {
		return nil, domain.ErrReservationNotFound
	}
	if err != nil {
		return nil, err
	}

	return r.rowToReservation(&row), nil
}

func (r *reservationRepository) GetByOrderID(ctx context.Context, orderID string) ([]*domain.Reservation, error) {
	var rows []reservationRow
	query := `SELECT * FROM inventory_reservations WHERE order_id = $1`

	err := r.db.SelectContext(ctx, &rows, query, orderID)
	if err != nil {
		return nil, err
	}

	reservations := make([]*domain.Reservation, len(rows))
	for i, row := range rows {
		reservations[i] = r.rowToReservation(&row)
	}

	return reservations, nil
}

func (r *reservationRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	query := `UPDATE inventory_reservations SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, time.Now(), id)
	return err
}

func (r *reservationRepository) rowToReservation(row *reservationRow) *domain.Reservation {
	var expiresAt *time.Time
	if row.ExpiresAt.Valid {
		expiresAt = &row.ExpiresAt.Time
	}

	return &domain.Reservation{
		ID:          row.ID,
		OrderID:     row.OrderID,
		OrderItemID: row.OrderItemID.String,
		ProductID:   row.ProductID,
		SKU:         row.SKU,
		WarehouseID: row.WarehouseID,
		Quantity:    row.Quantity,
		Status:      row.Status,
		ExpiresAt:   expiresAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

type movementRow struct {
	ID            string         `db:"id"`
	ProductID     string         `db:"product_id"`
	SKU           string         `db:"sku"`
	WarehouseID   string         `db:"warehouse_id"`
	MovementType  string         `db:"movement_type"`
	Quantity      int            `db:"quantity"`
	ReferenceType sql.NullString `db:"reference_type"`
	ReferenceID   sql.NullString `db:"reference_id"`
	CreatedAt     time.Time      `db:"created_at"`
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

	var referenceType sql.NullString
	var referenceID sql.NullString
	if movement.ReferenceType != "" {
		referenceType = sql.NullString{String: movement.ReferenceType, Valid: true}
	}
	if movement.ReferenceID != "" {
		referenceID = sql.NullString{String: movement.ReferenceID, Valid: true}
	}

	_, err := r.db.ExecContext(ctx, query,
		movement.ID,
		movement.ProductID,
		movement.SKU,
		movement.WarehouseID,
		string(movement.MovementType),
		movement.Quantity,
		referenceType,
		referenceID,
		movement.CreatedAt,
	)

	return err
}

func (r *movementRepository) GetByProductID(ctx context.Context, productID string) ([]*domain.InventoryMovement, error) {
	var rows []movementRow
	query := `SELECT * FROM inventory_movements WHERE product_id = $1 ORDER BY created_at DESC`

	err := r.db.SelectContext(ctx, &rows, query, productID)
	if err != nil {
		return nil, err
	}

	movements := make([]*domain.InventoryMovement, len(rows))
	for i, row := range rows {
		movements[i] = r.rowToMovement(&row)
	}

	return movements, nil
}

func (r *movementRepository) rowToMovement(row *movementRow) *domain.InventoryMovement {
	return &domain.InventoryMovement{
		ID:            row.ID,
		ProductID:     row.ProductID,
		SKU:           row.SKU,
		WarehouseID:   row.WarehouseID,
		MovementType:  domain.MovementType(row.MovementType),
		Quantity:      row.Quantity,
		ReferenceType: row.ReferenceType.String,
		ReferenceID:   row.ReferenceID.String,
		CreatedAt:     row.CreatedAt,
	}
}
