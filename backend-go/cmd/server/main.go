package main

import (
	"fmt"
	"log"
	"os"

	"github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/internal/api"
	"github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/internal/rag"
)

func main() {
	// 1. Config
	pythonAddr := os.Getenv("AI_SERVICE_ADDR")
	if pythonAddr == "" {
		pythonAddr = "localhost:50051"
	}

	fmt.Printf("🚀 [Go] Starting Orchestrator... (Target: %s)\n", pythonAddr)

	// 2. Start RAG Client
	ragClient, err := rag.NewClient(pythonAddr)
	if err != nil {
		log.Fatalf("❌ Failed to connect to Python service: %v", err)
	}
	defer ragClient.Close()

	// 3. Setup API Handler and Router (Dependency Injection)
	handler := api.NewHandler(ragClient)
	r := api.SetupRouter(handler)

	// 4. Start Server
	fmt.Println("🌍 [Go] HTTP Server running on port :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
