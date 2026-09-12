package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

	appLogger, err := logger.New("websocket-service")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() { _ = appLogger.Sync() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownTracer, err := tracing.InitTracer(ctx, "websocket-service", "otel-collector:4317")
	if err != nil {
		appLogger.Warn("Failed to initialize tracer", zap.Error(err))
	} else {
		defer func() { _ = shutdownTracer(context.Background()) }()
	}

	_ = otel.Tracer("websocket-service")

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       0,
	})

	hub := ws.NewHub(appLogger)
	go hub.Run()

	subscriber := wsinternal.NewSubscriber(rdb, hub, appLogger)
	go func() {
		if err := subscriber.Start(ctx); err != nil && err != context.Canceled {
			appLogger.Error("subscriber error", zap.Error(err))
		}
	}()

	handler := wsinternal.NewHandler(hub, appLogger)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)
	router.Use(metrics.Middleware("websocket-service"))

	router.Mount("/ws", handler.Routes())

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	router.Handle("/metrics", promhttp.Handler())

	addr := fmt.Sprintf(":%s", cfg.Server.HTTPPort)
	appLogger.Info("Starting websocket service", zap.String("addr", addr))

	go func() {
		if err := http.ListenAndServe(addr, router); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("Shutting down websocket service...")
	cancel()

	if err := rdb.Close(); err != nil {
		appLogger.Error("failed to close redis client", zap.Error(err))
	}

	appLogger.Info("Websocket service stopped")
}
