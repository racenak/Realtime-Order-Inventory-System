package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/httpd"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/adapter/postgres"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/order/usecase"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/config"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/database"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/logger"
)

func main() {
	cfg := config.Load()

	logger, err := logger.New("order-service")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	db, err := database.NewPostgresConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	orderRepo := postgres.NewOrderRepository(db)
	orderItemRepo := postgres.NewOrderItemRepository(db)
	outboxRepo := postgres.NewOutboxRepository(db)

	orderUC := usecase.NewCreateOrderUseCase(orderRepo, orderItemRepo, outboxRepo)

	orderHandler := httpd.NewOrderHandler(orderUC)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)

	router.Route("/api/orders", orderHandler.Routes)

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := fmt.Sprintf(":%s", cfg.Server.HTTPPort)
	logger.Info("Starting order service", "addr", addr)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
