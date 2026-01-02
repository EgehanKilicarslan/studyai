package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/api"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
	internalgrpc "github.com/EgehanKilicarslan/studyai/backend-go/internal/grpc"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/handler"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/logger"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/middleware"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/worker"
	pb "github.com/EgehanKilicarslan/studyai/backend-go/pb"
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

	// 3. Initialize Worker Pool for background tasks
	workerPool := worker.NewPool(appLogger)

	// 4. Connect to Database
	if err := database.ConnectDatabase(cfg, appLogger); err != nil {
		appLogger.Error("❌ Failed to connect to database", "error", err)
		os.Exit(1)
	}

	db := database.GetDatabase()

	// 5. Initialize Repositories
	userRepo := repository.NewUserRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)
	groupRepo := repository.NewGroupRepository(db)
	documentRepo := repository.NewDocumentRepository(db)
	orgRepo := repository.NewOrganizationRepository(db)
	chatRepo := repository.NewChatRepository(db)

	// 6. Initialize Redis Client
	redisClient, err := database.NewRedisClient(cfg, appLogger)
	if err != nil {
		appLogger.Warn("⚠️ Failed to connect to Redis for chat history", "error", err)
		appLogger.Info("💡 Chat history will only use Postgres (no Redis caching)")
		// Continue without Redis - chat will still work with Postgres only
	}

	// 7. Initialize Services
	authService := service.NewAuthService(userRepo, refreshTokenRepo, cfg, appLogger)
	groupService := service.NewGroupService(groupRepo, orgRepo, userRepo, appLogger)
	documentService := service.NewDocumentService(documentRepo, groupRepo, orgRepo, cfg, appLogger)
	orgService := service.NewOrganizationService(orgRepo, groupRepo, userRepo, appLogger)

	// 8. Initialize Handlers & Middleware
	authHandler := handler.NewAuthHandler(authService, appLogger)
	groupHandler := handler.NewGroupHandler(groupService, appLogger)
	adminHandler := handler.NewAdminHandler(orgService, groupService, appLogger)
	authMiddleware := middleware.NewAuthMiddleware(authService, appLogger)

	// 9. Initialize Rate Limiter
	rateLimiter, err := middleware.NewRateLimiter(cfg, appLogger)
	if err != nil {
		appLogger.Warn("⚠️ Failed to connect to Redis, using no-op rate limiter", "error", err)
		rateLimiter = middleware.NewNoOpRateLimiter(appLogger)
	}

	// 10. Start RAG Client (Go -> Python)
	grpcClient, err := internalgrpc.NewClient(cfg.AIServiceAddr, false)
	if err != nil {
		appLogger.Error("❌ Failed to connect to Python service", "error", err)
	}

	// 11. Start gRPC Server (Python -> Go)
	ragServiceServer := internalgrpc.NewRagServiceServer(documentRepo, orgRepo, appLogger)
	grpcServer := grpc.NewServer()
	pb.RegisterRagServiceServer(grpcServer, ragServiceServer)

	grpcAddr := fmt.Sprintf(":%s", cfg.ApiGrpcPort)
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		appLogger.Error("❌ Failed to listen for gRPC", "error", err)
		os.Exit(1)
	}

	// Start gRPC server in a goroutine
	go func() {
		appLogger.Info("🔌 [Go] gRPC Server running...", "port", cfg.ApiGrpcPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			appLogger.Error("❌ gRPC Server failed", "error", err)
		}
	}()

	// 12. Setup Chat and KnowledgeBase Handlers and Router
	chatHandler := handler.NewChatHandler(grpcClient, cfg, appLogger, rateLimiter, orgRepo, groupRepo, redisClient, chatRepo, workerPool)
	knowledgeBaseHandler := handler.NewKnowledgeBaseHandler(grpcClient, documentService, groupRepo, cfg, appLogger)

	r := api.SetupRouter(chatHandler, knowledgeBaseHandler, authHandler, groupHandler, adminHandler, authMiddleware)

	// 13. Create HTTP Server with graceful shutdown support
	addr := fmt.Sprintf(":%s", cfg.ApiServicePort)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// 14. Start HTTP Server in a goroutine
	go func() {
		appLogger.Info("🌍 [Go] HTTP Server running on port...", "port", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLogger.Error("❌ HTTP Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// 15. Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	appLogger.Info("🛑 [Go] Shutdown signal received, initiating graceful shutdown...")

	// 16. Create shutdown context with timeout
	shutdownTimeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// 17. Stop accepting new HTTP requests and wait for existing ones
	appLogger.Info("🔄 [Go] Shutting down HTTP server...")
	if err := httpServer.Shutdown(ctx); err != nil {
		appLogger.Error("❌ HTTP Server forced to shutdown", "error", err)
	} else {
		appLogger.Info("✅ [Go] HTTP server stopped gracefully")
	}

	// 18. Stop gRPC server gracefully
	appLogger.Info("🔄 [Go] Shutting down gRPC server...")
	grpcServer.GracefulStop()
	appLogger.Info("✅ [Go] gRPC server stopped gracefully")

	// 19. Wait for background tasks to complete
	appLogger.Info("🔄 [Go] Waiting for background tasks to complete...")
	workerPool.Shutdown(15 * time.Second)

	// 20. Close connections
	appLogger.Info("🔄 [Go] Closing connections...")
	if redisClient != nil {
		redisClient.Close()
	}
	rateLimiter.Close()
	grpcClient.Close()

	appLogger.Info("✅ [Go] Server exited gracefully")
}
