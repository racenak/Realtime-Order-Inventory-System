package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/cache"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/httpd"
	orderkafka "github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/kafka"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/postgres"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/usecase"
	pkgcache "github.com/racenak/Realtime-Order-Inventory-System/pkg/cache"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/config"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/database"
	kafkakit "github.com/racenak/Realtime-Order-Inventory-System/pkg/kafka"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/logger"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/metrics"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/tracing"
)

func main() {
	cfg := config.Load()

	logger, err := logger.New("order-service")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownTracer, err := tracing.InitTracer(ctx, "order-service", "otel-collector:4317")
	if err != nil {
		logger.Warn("Failed to initialize tracer", zap.Error(err))
	} else {
		defer func() { _ = shutdownTracer(context.Background()) }()
	}

	_ = otel.Tracer("order-service")

	db, err := database.NewPostgresConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	rdb, err := pkgcache.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer func() { _ = rdb.Close() }()

	orderRepo := postgres.NewOrderRepository(db)
	orderItemRepo := postgres.NewOrderItemRepository(db)
	outboxRepo := postgres.NewOutboxRepository(db)

	orderCache := pkgcache.New(rdb, "order", 5*time.Minute, logger)
	orderItemCache := pkgcache.New(rdb, "order", 5*time.Minute, logger)

	cachedOrderRepo := cache.NewOrderCache(orderRepo, orderCache)
	cachedOrderItemRepo := cache.NewOrderItemCache(orderItemRepo, orderItemCache)
	cachedOutboxRepo := cache.NewOutboxCache(outboxRepo)

	orderUC := usecase.NewCreateOrderUseCase(cachedOrderRepo, cachedOrderItemRepo, cachedOutboxRepo)

	orderHandler := httpd.NewOrderHandler(orderUC)

	outboxPublisher := orderkafka.NewOutboxPublisher(outboxRepo, cfg.Kafka.Brokers, logger)
	go outboxPublisher.Start(ctx, 5*time.Second, 10)

	inventoryEventWriter := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Kafka.Brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
	}

	inventoryConsumer := kafkakit.NewConsumer(
		kafkakit.ConsumerConfig{
			Brokers:  cfg.Kafka.Brokers,
			Topic:    "inventory.reserved",
			GroupID:  "order-service-inventory",
			MinBytes: 1,
			MaxBytes: 10e6,
		},
		orderkafka.NewInventoryEventHandler(orderUC, inventoryEventWriter, logger).Handle,
		logger,
	)
	go func() {
		if err := inventoryConsumer.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("inventory consumer error", zap.Error(err))
		}
	}()

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)
	router.Use(metrics.Middleware("order-service"))

	router.Mount("/api/orders", orderHandler.Routes())

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	router.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf(":%s", cfg.Server.HTTPPort)
	logger.Info("Starting order service", zap.String("addr", addr))

	go func() {
		if err := http.ListenAndServe(addr, router); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down order service...")
	cancel()

	if err := outboxPublisher.Close(); err != nil {
		logger.Error("failed to close outbox publisher", zap.Error(err))
	}
	if err := inventoryConsumer.Close(); err != nil {
		logger.Error("failed to close inventory consumer", zap.Error(err))
	}

	logger.Info("Order service stopped")
}
