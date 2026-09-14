package main

import (
	"context"
	"fmt"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Khởi tạo Logger mới (Chuẩn OTLP/gRPC + Zap Stdout)
	otelCollectorAddr := getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	logClient, cleanupLogger, err := logger.InitLogger(ctx, "inventory-service", otelCollectorAddr)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer cleanupLogger()

	// 2. Khởi tạo OpenTelemetry Tracer
	shutdownTracer, err := tracing.InitTracer(ctx, "inventory-service", otelCollectorAddr)
	if err != nil {
		logClient.Warn("Failed to initialize tracer", zap.Error(err))
	} else {
		defer func() { _ = shutdownTracer(context.Background()) }()
	}

	_ = otel.Tracer("inventory-service")

	// 3. Kết nối Database PostgreSQL
	db, err := database.NewPostgresConnection(cfg.Database)
	if err != nil {
		logClient.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer func() { _ = db.Close() }()

	// 4. Kết nối Redis Cache
	rdb, err := pkgcache.NewRedisClient(cfg.Redis)
	if err != nil {
		logClient.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer func() { _ = rdb.Close() }()

	// 5. Khởi tạo các Repositories & Cache Layer
	inventoryRepo := postgres.NewInventoryRepository(db)
	reservationRepo := postgres.NewReservationRepository(db)
	movementRepo := postgres.NewMovementRepository(db)

	invCache := pkgcache.New(rdb, "inventory", 30*time.Second, logClient.Zap())
	whCache := pkgcache.New(rdb, "inventory", 10*time.Minute, logClient.Zap())

	cachedInventoryRepo := cache.NewInventoryCache(inventoryRepo, invCache, whCache)

	// 6. Khởi tạo UseCase & Handlers
	inventoryUC := usecase.NewInventoryUseCase(cachedInventoryRepo, reservationRepo, movementRepo)
	inventoryHandler := httpd.NewInventoryHandler(inventoryUC)
	adapter := &inventoryUseCaseAdapter{uc: inventoryUC}

	// 7. Khởi tạo Kafka Writer & Consumer
	failedEventWriter := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Kafka.Brokers...),
		Topic:        "inventory.reservation_failed",
		Balancer:     &kafka.LeastBytes{},
		BatchTimeout: 10 * time.Millisecond,
	}

	orderConsumer := kafkakit.NewConsumer(
		kafkakit.ConsumerConfig{
			Brokers:  cfg.Kafka.Brokers,
			Topic:    "order.created",
			GroupID:  "inventory-service-orders",
			MinBytes: 1,
			MaxBytes: 10e6,
			Service:  "inventory-service",
		},
		inventorykafka.NewOrderEventHandler(adapter, failedEventWriter, logClient.Zap()).Handle,
		logClient.Zap(),
	)
	go func() {
		if err := orderConsumer.Start(ctx); err != nil && err != context.Canceled {
			logClient.Error("order consumer error", zap.Error(err))
		}
	}()

	// 8. Định nghĩa Chi Router
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)
	router.Use(metrics.Middleware("inventory-service"))
	router.Use(tracing.Middleware("inventory-service"))

	router.Mount("/api/inventory", inventoryHandler.Routes())

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	router.Handle("/metrics", promhttp.Handler())

	// 9. Lắng nghe HTTP Server & Graceful Shutdown
	addr := fmt.Sprintf(":%s", cfg.Server.HTTPPort)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	logClient.Info("Starting inventory service", zap.String("addr", addr))

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logClient.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logClient.Info("Shutting down inventory service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logClient.Error("Server forced to shutdown", zap.Error(err))
	}

	if err := orderConsumer.Close(); err != nil {
		logClient.Error("Failed to close order consumer", zap.Error(err))
	}

	logClient.Info("Inventory service stopped cleanly")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
