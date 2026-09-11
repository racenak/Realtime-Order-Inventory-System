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

	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/adapter/cache"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/adapter/httpd"
	inventorykafka "github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/adapter/kafka"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/adapter/postgres"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/usecase"
	pkgcache "github.com/racenak/Realtime-Order-Inventory-System/pkg/cache"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/config"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/database"
	kafkakit "github.com/racenak/Realtime-Order-Inventory-System/pkg/kafka"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/logger"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/metrics"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/tracing"
)

type inventoryUseCaseAdapter struct {
	uc usecase.InventoryUseCase
}

func (a *inventoryUseCaseAdapter) ReserveStock(ctx context.Context, req inventorykafka.ReserveStockRequest) error {
	_, err := a.uc.ReserveStock(ctx, usecase.ReserveStockRequest{
		OrderID:     req.OrderID,
		ProductID:   req.ProductID,
		WarehouseID: req.WarehouseID,
		Quantity:    req.Quantity,
	})
	return err
}

func (a *inventoryUseCaseAdapter) ReleaseReservation(ctx context.Context, orderID string) error {
	return a.uc.ReleaseReservation(ctx, orderID)
}

func main() {
	cfg := config.Load()

	logger, err := logger.New("inventory-service")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownTracer, err := tracing.InitTracer(ctx, "inventory-service", "otel-collector:4317")
	if err != nil {
		logger.Warn("Failed to initialize tracer", zap.Error(err))
	} else {
		defer shutdownTracer(context.Background())
	}

	_ = otel.Tracer("inventory-service")

	db, err := database.NewPostgresConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	rdb, err := pkgcache.NewRedisClient(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer rdb.Close()

	inventoryRepo := postgres.NewInventoryRepository(db)
	reservationRepo := postgres.NewReservationRepository(db)
	movementRepo := postgres.NewMovementRepository(db)

	invCache := pkgcache.New(rdb, "inventory", 30*time.Second, logger)
	whCache := pkgcache.New(rdb, "inventory", 10*time.Minute, logger)

	cachedInventoryRepo := cache.NewInventoryCache(inventoryRepo, invCache, whCache)

	inventoryUC := usecase.NewInventoryUseCase(cachedInventoryRepo, reservationRepo, movementRepo)

	inventoryHandler := httpd.NewInventoryHandler(inventoryUC)

	adapter := &inventoryUseCaseAdapter{uc: inventoryUC}
	orderConsumer := kafkakit.NewConsumer(
		kafkakit.ConsumerConfig{
			Brokers:  cfg.Kafka.Brokers,
			Topic:    "order.created",
			GroupID:  "inventory-service-orders",
			MinBytes: 1,
			MaxBytes: 10e6,
		},
		inventorykafka.NewOrderEventHandler(adapter, &kafka.Writer{}, logger).Handle,
		logger,
	)
	go func() {
		if err := orderConsumer.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("order consumer error", zap.Error(err))
		}
	}()

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)
	router.Use(metrics.Middleware("inventory-service"))

	router.Mount("/api/inventory", inventoryHandler.Routes())

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	router.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf(":%s", cfg.Server.HTTPPort)
	logger.Info("Starting inventory service", zap.String("addr", addr))

	go func() {
		if err := http.ListenAndServe(addr, router); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down inventory service...")
	cancel()

	if err := orderConsumer.Close(); err != nil {
		logger.Error("failed to close order consumer", zap.Error(err))
	}

	logger.Info("Inventory service stopped")
}
