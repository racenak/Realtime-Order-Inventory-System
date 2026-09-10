package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/adapter/httpd"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/adapter/postgres"
	"github.com/racenak/Realtime-Order-Inventory-System/internal/inventory/usecase"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/config"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/database"
	"github.com/racenak/Realtime-Order-Inventory-System/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()

	logger, err := logger.New("inventory-service")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	db, err := database.NewPostgresConnection(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	inventoryRepo := postgres.NewInventoryRepository(db)
	reservationRepo := postgres.NewReservationRepository(db)
	movementRepo := postgres.NewMovementRepository(db)

	inventoryUC := usecase.NewInventoryUseCase(inventoryRepo, reservationRepo, movementRepo)

	inventoryHandler := httpd.NewInventoryHandler(inventoryUC)

	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.RequestID)

	router.Mount("/api/inventory", inventoryHandler.Routes())

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := fmt.Sprintf(":%s", cfg.Server.HTTPPort)
	logger.Info("Starting inventory service", zap.String("addr", addr))

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
