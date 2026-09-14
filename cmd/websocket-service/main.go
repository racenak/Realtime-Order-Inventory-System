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
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	wsinternal "github.com/racenak/Realtime-Order-Inventory-System/internal/websocket"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/config"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/logger"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/metrics"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/tracing"
	ws "github.com/racenak/Realtime-Order-Inventory-System/pkg/websocket"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Khởi tạo Logger mới (Xuất Stdout + Gửi OTLP về otel-collector)
	otelCollectorAddr := getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	logClient, cleanupLogger, err := logger.InitLogger(ctx, "websocket-service", otelCollectorAddr)
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer cleanupLogger()

	// 2. Khởi tạo OpenTelemetry Tracer
	shutdownTracer, err := tracing.InitTracer(ctx, "websocket-service", otelCollectorAddr)
	if err != nil {
		logClient.Warn("Failed to initialize tracer", zap.Error(err))
	} else {
		defer func() { _ = shutdownTracer(context.Background()) }()
	}

	_ = otel.Tracer("websocket-service")

	// 3. Kết nối Redis Client
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       0,
	})

	// Test kết nối Redis
	if err := rdb.Ping(ctx).Err(); err != nil {
		logClient.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			logClient.Error("Failed to close redis client", zap.Error(err))
		}
	}()

	// 4. Khởi tạo WebSocket Hub & Subscriber
	hub := ws.NewHub(logClient.Zap())
	go hub.Run()

	subscriber := wsinternal.NewSubscriber(rdb, hub, logClient.Zap())
	go func() {
		if err := subscriber.Start(ctx); err != nil && err != context.Canceled {
			logClient.Error("Subscriber error", zap.Error(err))
		}
	}()

	handler := wsinternal.NewHandler(hub, logClient.Zap())

	// 5. Định nghĩa Chi Router
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)
	router.Use(metrics.Middleware("websocket-service"))
	router.Use(tracing.Middleware("websocket-service"))

	router.Mount("/ws", handler.Routes())

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	router.Handle("/metrics", promhttp.Handler())

	// 6. Lắng nghe HTTP Server & Graceful Shutdown
	addr := fmt.Sprintf(":%s", cfg.Server.HTTPPort)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	logClient.Info("Starting websocket service", zap.String("addr", addr))

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logClient.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logClient.Info("Shutting down websocket service...")
	cancel() // Cancel context để dừng subscriber goroutine

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logClient.Error("Server forced to shutdown", zap.Error(err))
	}

	logClient.Info("Websocket service stopped cleanly")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
