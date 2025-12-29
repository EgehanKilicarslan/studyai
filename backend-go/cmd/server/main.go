package main

import (
	"fmt"
	"os"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/api"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/handler"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/logger"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/middleware"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/rag"
)

func main() {
	// 1. Config
	cfg := config.LoadConfig()

	// 2. Logger
	appLogger := logger.New(cfg)

	appLogger.Info("🚀 [Go] Starting Orchestrator...",
		"target_service", cfg.AIServiceAddr,
		"environment", cfg.AppEnv,
	)

	// 3. Connect to Database
	if err := database.ConnectDatabase(cfg, appLogger); err != nil {
		appLogger.Error("❌ Failed to connect to database", "error", err)
		os.Exit(1)
	}

	db := database.GetDatabase()

	// 4. Initialize Repositories
	userRepo := repository.NewUserRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)

	// 5. Initialize Services
	authService := service.NewAuthService(userRepo, refreshTokenRepo, cfg, appLogger)

	// 6. Initialize Handlers & Middleware
	authHandler := handler.NewAuthHandler(authService, appLogger)
	authMiddleware := middleware.NewAuthMiddleware(authService, appLogger)

	// 7. Start RAG Client
	ragClient, err := rag.NewClient(cfg.AIServiceAddr, false)
	if err != nil {
		appLogger.Error("❌ Failed to connect to Python service", "error", err)
	}
	defer ragClient.Close()

	// 8. Setup API Handler and Router
	apiHandler := handler.NewApiHandler(ragClient, cfg, appLogger)
	r := api.SetupRouter(apiHandler, authHandler, authMiddleware)

	// 9. Start Server
	addr := fmt.Sprintf(":%s", cfg.ApiServicePort)
	appLogger.Info("🌍 [Go] HTTP Server running on port...", "port", addr)
	if err := r.Run(addr); err != nil {
		appLogger.Error("❌ HTTP Server failed to start", "error", err)
		os.Exit(1)
	}
}
