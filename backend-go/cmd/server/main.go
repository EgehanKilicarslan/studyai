package main

import (
	"fmt"
	"os"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/api"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/logger"
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

	// 3. Start RAG Client
	ragClient, err := rag.NewClient(cfg.AIServiceAddr, false)
	if err != nil {
		appLogger.Error("❌ Failed to connect to Python service",
			"error", err,
		)
	}
	defer ragClient.Close()

	// 4. Setup API Handler and Router (Dependency Injection)
	handler := api.NewHandler(ragClient, cfg, appLogger)
	r := api.SetupRouter(handler)

	// 5. Start Server
	addr := fmt.Sprintf(":%s", cfg.ApiServicePort)
	appLogger.Info("🌍 [Go] HTTP Server running on port...",
		"port", addr,
	)
	if err := r.Run(addr); err != nil {
		appLogger.Error("❌ HTTP Server failed to start",
			"error", err,
		)
		os.Exit(1)
	}
}
