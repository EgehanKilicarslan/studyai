package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/pb"
)

func main() {
	fmt.Println("🚀 [Go] Starting Orchestrator...")

	// 1. Connect to Python gRPC Service
	aiServiceAddr := os.Getenv("AI_SERVICE_ADDR")
	if aiServiceAddr == "" {
		aiServiceAddr = "localhost:50051" // Default address for local testing
	}

	conn, err := grpc.NewClient(aiServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("❌ Failed to connect to Python service: %v", err)
	}
	defer conn.Close()

	// Create gRPC Client
	client := pb.NewRagServiceClient(conn)
	fmt.Println("✅ [Go] Connected to Python service!")

	// 2. Set up HTTP Server (Gin)
	r := gin.Default()

	// Untrusted Proxy Handling
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Fatalf("❌ Failed to set trusted proxies: %v", err)
	}

	// CORS Configuration (simple setup for React Frontend)
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// POST /api/chat endpoint
	r.POST("/api/chat", func(c *gin.Context) {
		// Parse incoming JSON
		var reqBody struct {
			Query     string `json:"query"`
			SessionID string `json:"session_id"`
		}

		if err := c.BindJSON(&reqBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
			return
		}

		// Prepare gRPC request to send to Python
		grpcReq := &pb.ChatRequest{
			Query:     reqBody.Query,
			SessionId: reqBody.SessionID,
			Config: &pb.QueryConfig{
				CollectionName: "public_data",
				MaxResults:     3,
			},
		}

		// Make gRPC call (Timeout: 10 seconds)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resp, err := client.Chat(ctx, grpcReq)
		if err != nil {
			// Handle error if Python service is down or returns an error
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI Service Error: %v", err)})
			return
		}

		// Return response to user
		c.JSON(http.StatusOK, gin.H{
			"answer":  resp.Answer,
			"sources": resp.SourceDocuments,
			"time":    resp.ProcessingTimeMs,
		})
	})

	// Start server on port 8080
	fmt.Println("🌍 [Go] HTTP Server Listening on port :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}
