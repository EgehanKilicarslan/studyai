package main

import (
	"fmt"
	"os"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/api"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/grpc"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/handler"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/logger"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/middleware"
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
	groupRepo := repository.NewGroupRepository(db)
	documentRepo := repository.NewDocumentRepository(db)
	orgRepo := repository.NewOrganizationRepository(db)
	chatRepo := repository.NewChatRepository(db)

	// 5. Initialize Redis Client
	redisClient, err := database.NewRedisClient(cfg, appLogger)
	if err != nil {
		appLogger.Warn("⚠️ Failed to connect to Redis for chat history", "error", err)
		appLogger.Info("💡 Chat history will only use Postgres (no Redis caching)")
		// Continue without Redis - chat will still work with Postgres only
	}
	defer func() {
		if redisClient != nil {
			redisClient.Close()
		}
	}()

	// 6. Initialize Services
	authService := service.NewAuthService(userRepo, refreshTokenRepo, cfg, appLogger)
	groupService := service.NewGroupService(groupRepo, orgRepo, userRepo, appLogger)
	documentService := service.NewDocumentService(documentRepo, groupRepo, orgRepo, cfg, appLogger)
	orgService := service.NewOrganizationService(orgRepo, groupRepo, userRepo, appLogger)

	// 7. Initialize Handlers & Middleware
	authHandler := handler.NewAuthHandler(authService, appLogger)
	groupHandler := handler.NewGroupHandler(groupService, appLogger)
	adminHandler := handler.NewAdminHandler(orgService, groupService, appLogger)
	authMiddleware := middleware.NewAuthMiddleware(authService, appLogger)

	// 7. Initialize Rate Limiter
	rateLimiter, err := middleware.NewRateLimiter(cfg, appLogger)
	if err != nil {
		appLogger.Warn("⚠️ Failed to connect to Redis, using no-op rate limiter", "error", err)
		rateLimiter = middleware.NewNoOpRateLimiter(appLogger)
	}
	defer rateLimiter.Close()

	// 8. Start RAG Client
	grpcClient, err := grpc.NewClient(cfg.AIServiceAddr, false)
	if err != nil {
		appLogger.Error("❌ Failed to connect to Python service", "error", err)
	}
	defer grpcClient.Close()

	// 9. Setup Chat and KnowledgeBase Handlers and Router
	chatHandler := handler.NewChatHandler(grpcClient, cfg, appLogger, rateLimiter, orgRepo, groupRepo, redisClient, chatRepo)
	knowledgeBaseHandler := handler.NewKnowledgeBaseHandler(grpcClient, documentService, groupRepo, cfg, appLogger)

	r := api.SetupRouter(chatHandler, knowledgeBaseHandler, authHandler, groupHandler, adminHandler, authMiddleware)

	// 10. Start Server
	addr := fmt.Sprintf(":%s", cfg.ApiServicePort)
	appLogger.Info("🌍 [Go] HTTP Server running on port...", "port", addr)
	if err := r.Run(addr); err != nil {
		appLogger.Error("❌ HTTP Server failed to start", "error", err)
		os.Exit(1)
	}
}
