package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Cache struct {
	rdb    *redis.Client
	prefix string
	ttl    time.Duration
	logger *zap.Logger
	tracer trace.Tracer
}

func New(rdb *redis.Client, prefix string, ttl time.Duration, logger *zap.Logger) *Cache {
	return &Cache{
		rdb:    rdb,
		prefix: prefix,
		ttl:    ttl,
		logger: logger,
		tracer: otel.Tracer("cache"),
	}
}

func (c *Cache) key(k string) string {
	return c.prefix + ":" + k
}

func (c *Cache) Get(ctx context.Context, key string, dest interface{}) bool {
	fullKey := c.key(key)
	ctx, span := c.tracer.Start(ctx, "redis.get",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.statement", "GET "+fullKey),
		),
	)
	defer span.End()

	val, err := c.rdb.Get(ctx, fullKey).Result()
	if err != nil {
		span.SetAttributes(attribute.Bool("cache.hit", false))
		return false
	}
	span.SetAttributes(attribute.Bool("cache.hit", true))

	if err := json.Unmarshal([]byte(val), dest); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.logger.Warn("cache unmarshal failed", zap.String("key", key), zap.Error(err))
		return false
	}
	return true
}

func (c *Cache) Set(ctx context.Context, key string, value interface{}) {
	fullKey := c.key(key)
	ctx, span := c.tracer.Start(ctx, "redis.set",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.String("db.statement", "SET "+fullKey),
			attribute.String("cache.ttl", c.ttl.String()),
		),
	)
	defer span.End()

	data, err := json.Marshal(value)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.logger.Warn("cache marshal failed", zap.String("key", key), zap.Error(err))
		return
	}
	if err := c.rdb.Set(ctx, fullKey, data, c.ttl).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.logger.Warn("cache set failed", zap.String("key", key), zap.Error(err))
	}
}

func (c *Cache) Delete(ctx context.Context, keys ...string) {
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = c.key(k)
	}

	ctx, span := c.tracer.Start(ctx, "redis.del",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "redis"),
			attribute.StringSlice("db.statement", fullKeys),
		),
	)
	defer span.End()

	if err := c.rdb.Del(ctx, fullKeys...).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.logger.Warn("cache delete failed", zap.Strings("keys", keys), zap.Error(err))
	}
}
