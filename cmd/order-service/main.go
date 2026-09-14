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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Khởi tạo Logger mới (Xuất Stdout + Gửi OTLP về otel-collector)
	otelCollectorAddr := getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	logClient, cleanupLogger, err := logger.InitLogger(ctx, "order-service", otelCollectorAddr)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer cleanupLogger()

	// 2. Khởi tạo OpenTelemetry Tracer
	shutdownTracer, err := tracing.InitTracer(ctx, "order-service", otelCollectorAddr)
	if err != nil {
		logClient.Warn("Failed to initialize tracer", zap.Error(err))
	} else {
		defer func() { _ = shutdownTracer(context.Background()) }()
	}

	_ = otel.Tracer("order-service")

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
	orderRepo := postgres.NewOrderRepository(db)
	orderItemRepo := postgres.NewOrderItemRepository(db)
	outboxRepo := postgres.NewOutboxRepository(db)

	orderCache := pkgcache.New(rdb, "order", 5*time.Minute, logClient.Zap())
	orderItemCache := pkgcache.New(rdb, "order", 5*time.Minute, logClient.Zap())

	cachedOrderRepo := cache.NewOrderCache(orderRepo, orderCache)
	cachedOrderItemRepo := cache.NewOrderItemCache(orderItemRepo, orderItemCache)
	cachedOutboxRepo := cache.NewOutboxCache(outboxRepo)

	// 6. Khởi tạo UseCase & Handlers
	orderUC := usecase.NewCreateOrderUseCase(cachedOrderRepo, cachedOrderItemRepo, cachedOutboxRepo)
	orderHandler := httpd.NewOrderHandler(orderUC)

	// 7. Khởi tạo Outbox Publisher & Kafka Consumer
	outboxPublisher := orderkafka.NewOutboxPublisher(outboxRepo, cfg.Kafka.Brokers, logClient.Zap())
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
			Service:  "order-service",
		},
		orderkafka.NewInventoryEventHandler(orderUC, inventoryEventWriter, logClient.Zap()).Handle,
		logClient.Zap(),
	)
	go func() {
		if err := inventoryConsumer.Start(ctx); err != nil && err != context.Canceled {
			logClient.Error("inventory consumer error", zap.Error(err))
		}
	}()

	// 8. Định nghĩa Chi Router
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)
	router.Use(metrics.Middleware("order-service"))
	router.Use(tracing.Middleware("order-service"))

	router.Mount("/api/orders", orderHandler.Routes())

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

	logClient.Info("Starting order service", zap.String("addr", addr))

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logClient.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logClient.Info("Shutting down order service...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logClient.Error("Server forced to shutdown", zap.Error(err))
	}

	if err := outboxPublisher.Close(); err != nil {
		logClient.Error("Failed to close outbox publisher", zap.Error(err))
	}
	if err := inventoryConsumer.Close(); err != nil {
		logClient.Error("Failed to close inventory consumer", zap.Error(err))
	}

	logClient.Info("Order service stopped cleanly")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
