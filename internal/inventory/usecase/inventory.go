package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/domain"
)

type inventoryUseCase struct {
	inventoryRepo    domain.InventoryRepository
	reservationRepo  domain.ReservationRepository
	movementRepo     domain.MovementRepository
}

func NewInventoryUseCase(
	inventoryRepo domain.InventoryRepository,
	reservationRepo domain.ReservationRepository,
	movementRepo domain.MovementRepository,
) InventoryUseCase {
	return &inventoryUseCase{
		inventoryRepo:   inventoryRepo,
		reservationRepo: reservationRepo,
		movementRepo:    movementRepo,
	}
}

func (uc *inventoryUseCase) GetStock(ctx context.Context, productID string) (*StockResponse, error) {
	inventories, err := uc.inventoryRepo.GetByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	warehouses := make([]WarehouseStock, len(inventories))
	for i, inv := range inventories {
		warehouses[i] = WarehouseStock{
			WarehouseID: inv.WarehouseID,
			Quantity:    inv.QuantityOnHand,
			Reserved:    inv.QuantityReserved,
			Available:   inv.Available(),
		}
	}

	return &StockResponse{
		ProductID:  productID,
		Warehouses: warehouses,
	}, nil
}

func (uc *inventoryUseCase) ReserveStock(ctx context.Context, req ReserveStockRequest) (*domain.Reservation, error) {
	if req.Quantity <= 0 {
		return nil, domain.ErrInvalidQuantity
	}

	inv, err := uc.inventoryRepo.GetByProductAndWarehouse(ctx, req.ProductID, req.WarehouseID)
	if err != nil {
		return nil, err
	}

	if inv.Available() < req.Quantity {
		return nil, domain.ErrInsufficientStock
	}

	err = uc.inventoryRepo.UpdateStock(ctx, req.ProductID, req.WarehouseID, 0, inv.Version)
	if err == domain.ErrConcurrentModification {
		return nil, domain.ErrConcurrentModification
	}
	if err != nil {
		return nil, err
	}

	reservation := &domain.Reservation{
		ID:          uuid.New().String(),
		OrderID:     req.OrderID,
		ProductID:   req.ProductID,
		WarehouseID: req.WarehouseID,
		Quantity:    req.Quantity,
		Status:      "reserved",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := uc.reservationRepo.Create(ctx, reservation); err != nil {
		return nil, err
	}

	movement := &domain.InventoryMovement{
		ID:            uuid.New().String(),
		ProductID:     req.ProductID,
		WarehouseID:   req.WarehouseID,
		MovementType:  domain.MovementReserve,
		Quantity:      -req.Quantity,
		ReferenceType: "order",
		ReferenceID:   req.OrderID,
		CreatedAt:     time.Now(),
	}

	_ = uc.movementRepo.Create(ctx, movement)

	return reservation, nil
}

func (uc *inventoryUseCase) ReleaseReservation(ctx context.Context, reservationID string) error {
	reservation, err := uc.reservationRepo.GetByID(ctx, reservationID)
	if err != nil {
		return err
	}

	if reservation.Status != "reserved" {
		return nil
	}

	err = uc.reservationRepo.UpdateStatus(ctx, reservationID, "released")
	if err != nil {
		return fmt.Errorf("failed to update reservation status: %w", err)
	}

	movement := &domain.InventoryMovement{
		ID:            uuid.New().String(),
		ProductID:     reservation.ProductID,
		WarehouseID:   reservation.WarehouseID,
		MovementType:  domain.MovementRelease,
		Quantity:      reservation.Quantity,
		ReferenceType: "order",
		ReferenceID:   reservation.OrderID,
		CreatedAt:     time.Now(),
	}

	_ = uc.movementRepo.Create(ctx, movement)

	return nil
}

func (uc *inventoryUseCase) UpdateStock(ctx context.Context, req UpdateStockRequest) error {
	inv, err := uc.inventoryRepo.GetByProductAndWarehouse(ctx, req.ProductID, req.WarehouseID)
	if err != nil {
		return err
	}

	err = uc.inventoryRepo.UpdateStock(ctx, req.ProductID, req.WarehouseID, req.Quantity, inv.Version)
	if err != nil {
		return err
	}

	movementType := domain.MovementIn
	if req.Quantity < 0 {
		movementType = domain.MovementOut
	}

	movement := &domain.InventoryMovement{
		ID:            uuid.New().String(),
		ProductID:     req.ProductID,
		WarehouseID:   req.WarehouseID,
		MovementType:  movementType,
		Quantity:      req.Quantity,
		ReferenceType: "manual",
		CreatedAt:     time.Now(),
	}

	_ = uc.movementRepo.Create(ctx, movement)

	return nil
}
