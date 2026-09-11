package cache

import (
	"context"
	"fmt"

	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/domain"
	pkgcache "github.com/racenak/Realtime-Order-Inventory-System/pkg/cache"
)

type OrderCache struct {
	inner domain.OrderRepository
	cache *pkgcache.Cache
}

func NewOrderCache(inner domain.OrderRepository, cache *pkgcache.Cache) *OrderCache {
	return &OrderCache{inner: inner, cache: cache}
}

func (c *OrderCache) Create(ctx context.Context, order *domain.Order) error {
	return c.inner.Create(ctx, order)
}

func (c *OrderCache) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	var order domain.Order
	if c.cache.Get(ctx, orderKey(id), &order) {
		return &order, nil
	}

	orderPtr, err := c.inner.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	c.cache.Set(ctx, orderKey(id), orderPtr)
	return orderPtr, nil
}

func (c *OrderCache) List(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, int, error) {
	return c.inner.List(ctx, customerID, limit, offset)
}

func (c *OrderCache) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus) error {
	err := c.inner.UpdateStatus(ctx, id, status)
	if err == nil {
		c.cache.Delete(ctx, orderKey(id))
	}
	return err
}

func orderKey(id string) string {
	return id
}

type OrderItemCache struct {
	inner domain.OrderItemRepository
	cache *pkgcache.Cache
}

func NewOrderItemCache(inner domain.OrderItemRepository, cache *pkgcache.Cache) *OrderItemCache {
	return &OrderItemCache{inner: inner, cache: cache}
}

func (c *OrderItemCache) Create(ctx context.Context, items []domain.OrderItem) error {
	err := c.inner.Create(ctx, items)
	if err == nil && len(items) > 0 {
		keys := make([]string, 0, len(items))
		seen := make(map[string]bool)
		for _, item := range items {
			if !seen[item.OrderID] {
				keys = append(keys, orderItemsKey(item.OrderID))
				seen[item.OrderID] = true
			}
		}
		c.cache.Delete(ctx, keys...)
	}
	return err
}

func (c *OrderItemCache) GetByOrderID(ctx context.Context, orderID string) ([]domain.OrderItem, error) {
	var items []domain.OrderItem
	if c.cache.Get(ctx, orderItemsKey(orderID), &items) {
		return items, nil
	}

	items, err := c.inner.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, err
	}

	c.cache.Set(ctx, orderItemsKey(orderID), items)
	return items, nil
}

func orderItemsKey(orderID string) string {
	return fmt.Sprintf("items:%s", orderID)
}

type OutboxCache struct {
	inner domain.OutboxRepository
}

func NewOutboxCache(inner domain.OutboxRepository) *OutboxCache {
	return &OutboxCache{inner: inner}
}

func (c *OutboxCache) Create(ctx context.Context, event domain.OutboxEvent) error {
	return c.inner.Create(ctx, event)
}

func (c *OutboxCache) GetPending(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	return c.inner.GetPending(ctx, limit)
}

func (c *OutboxCache) MarkPublished(ctx context.Context, id string) error {
	return c.inner.MarkPublished(ctx, id)
}

var _ domain.OrderRepository = (*OrderCache)(nil)
var _ domain.OrderItemRepository = (*OrderItemCache)(nil)
var _ domain.OutboxRepository = (*OutboxCache)(nil)
