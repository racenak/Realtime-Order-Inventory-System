package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Cache struct {
	rdb    *redis.Client
	prefix string
	ttl    time.Duration
	logger *zap.Logger
}

func New(rdb *redis.Client, prefix string, ttl time.Duration, logger *zap.Logger) *Cache {
	return &Cache{
		rdb:    rdb,
		prefix: prefix,
		ttl:    ttl,
		logger: logger,
	}
}

func (c *Cache) key(k string) string {
	return c.prefix + ":" + k
}

func (c *Cache) Get(ctx context.Context, key string, dest interface{}) bool {
	val, err := c.rdb.Get(ctx, c.key(key)).Result()
	if err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(val), dest); err != nil {
		c.logger.Warn("cache unmarshal failed", zap.String("key", key), zap.Error(err))
		return false
	}
	return true
}

func (c *Cache) Set(ctx context.Context, key string, value interface{}) {
	data, err := json.Marshal(value)
	if err != nil {
		c.logger.Warn("cache marshal failed", zap.String("key", key), zap.Error(err))
		return
	}
	if err := c.rdb.Set(ctx, c.key(key), data, c.ttl).Err(); err != nil {
		c.logger.Warn("cache set failed", zap.String("key", key), zap.Error(err))
	}
}

func (c *Cache) Delete(ctx context.Context, keys ...string) {
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = c.key(k)
	}
	if err := c.rdb.Del(ctx, fullKeys...).Err(); err != nil {
		c.logger.Warn("cache delete failed", zap.Strings("keys", keys), zap.Error(err))
	}
}
