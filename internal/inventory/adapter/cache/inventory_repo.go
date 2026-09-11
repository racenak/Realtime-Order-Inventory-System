package cache

import (
	"context"
	"fmt"

	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/domain"
	pkgcache "github.com/racenak/Realtime-Order-Inventory-System/pkg/cache"
)

type InventoryCache struct {
	inner          domain.InventoryRepository
	stockCache     *pkgcache.Cache
	warehouseCache *pkgcache.Cache
}

func NewInventoryCache(inner domain.InventoryRepository, stockCache, warehouseCache *pkgcache.Cache) *InventoryCache {
	return &InventoryCache{inner: inner, stockCache: stockCache, warehouseCache: warehouseCache}
}

func (c *InventoryCache) Create(ctx context.Context, inventory *domain.Inventory) error {
	err := c.inner.Create(ctx, inventory)
	if err == nil {
		c.stockCache.Delete(ctx,
			inventoryByProductWarehouseKey(inventory.ProductID, inventory.WarehouseID),
			inventoryByProductKey(inventory.ProductID),
		)
	}
	return err
}

func (c *InventoryCache) GetByProductAndWarehouse(ctx context.Context, productID, warehouseID string) (*domain.Inventory, error) {
	key := inventoryByProductWarehouseKey(productID, warehouseID)
	var inv domain.Inventory
	if c.stockCache.Get(ctx, key, &inv) {
		return &inv, nil
	}

	invPtr, err := c.inner.GetByProductAndWarehouse(ctx, productID, warehouseID)
	if err != nil {
		return nil, err
	}

	c.stockCache.Set(ctx, key, invPtr)
	return invPtr, nil
}

func (c *InventoryCache) GetByProductID(ctx context.Context, productID string) ([]*domain.Inventory, error) {
	key := inventoryByProductKey(productID)
	var inventories []*domain.Inventory
	if c.stockCache.Get(ctx, key, &inventories) {
		return inventories, nil
	}

	inventories, err := c.inner.GetByProductID(ctx, productID)
	if err != nil {
		return nil, err
	}

	c.stockCache.Set(ctx, key, inventories)
	return inventories, nil
}

func (c *InventoryCache) UpdateStock(ctx context.Context, productID, warehouseID string, quantityChange int, expectedVersion int64) error {
	err := c.inner.UpdateStock(ctx, productID, warehouseID, quantityChange, expectedVersion)
	if err == nil {
		c.stockCache.Delete(ctx,
			inventoryByProductWarehouseKey(productID, warehouseID),
			inventoryByProductKey(productID),
		)
	}
	return err
}

func (c *InventoryCache) ReserveQuantity(ctx context.Context, productID, warehouseID string, quantity int, expectedVersion int64) error {
	err := c.inner.ReserveQuantity(ctx, productID, warehouseID, quantity, expectedVersion)
	if err == nil {
		c.stockCache.Delete(ctx,
			inventoryByProductWarehouseKey(productID, warehouseID),
			inventoryByProductKey(productID),
		)
	}
	return err
}

func (c *InventoryCache) ReleaseQuantity(ctx context.Context, productID, warehouseID string, quantity int) error {
	err := c.inner.ReleaseQuantity(ctx, productID, warehouseID, quantity)
	if err == nil {
		c.stockCache.Delete(ctx,
			inventoryByProductWarehouseKey(productID, warehouseID),
			inventoryByProductKey(productID),
		)
	}
	return err
}

func (c *InventoryCache) GetWarehouseByCode(ctx context.Context, code string) (*domain.Warehouse, error) {
	key := warehouseCodeKey(code)
	var wh domain.Warehouse
	if c.warehouseCache.Get(ctx, key, &wh) {
		return &wh, nil
	}

	whPtr, err := c.inner.GetWarehouseByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	c.warehouseCache.Set(ctx, key, whPtr)
	return whPtr, nil
}

func inventoryByProductWarehouseKey(productID, warehouseID string) string {
	return fmt.Sprintf("stock:warehouse:%s:%s", productID, warehouseID)
}

func inventoryByProductKey(productID string) string {
	return fmt.Sprintf("stock:%s", productID)
}

func warehouseCodeKey(code string) string {
	return fmt.Sprintf("warehouse:code:%s", code)
}

var _ domain.InventoryRepository = (*InventoryCache)(nil)
