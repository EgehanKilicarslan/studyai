package handler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/grpc"
	pb "github.com/EgehanKilicarslan/studyai/backend-go/pb"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/metadata"
)

// ChatHandler handles HTTP requests and forwards them to the RAG service
type ChatHandler struct {
	services *grpc.Client
	cfg      *config.Config
	logger   *slog.Logger
}

// NewChatHandler injects dependencies (Dependency Injection Go Style)
func NewChatHandler(services *grpc.Client, cfg *config.Config, logger *slog.Logger) *ChatHandler {
	return &ChatHandler{services: services, cfg: cfg, logger: logger}
}

// ChatHandler: POST /api/v1/chat
func (h *ChatHandler) ChatHandler(c *gin.Context) {
	// Extract authenticated user ID
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [Handler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Convert userID to string for gRPC metadata
	userIDStr := fmt.Sprintf("%v", userID)

	h.logger.Info("📨 [Handler] Chat request received",
		"user_id", userID,
	)

	var reqBody struct {
		Query     string `json:"query"`
		SessionID string `json:"session_id"`
	}

	if err := c.BindJSON(&reqBody); err != nil {
		h.logger.Error("❌ [Handler] Invalid JSON in chat request",
			"error", err,
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if reqBody.Query == "" {
		h.logger.Error("❌ [Handler] Missing query in chat request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query is required"})
		return
	}

	h.logger.Info("🔍 [Handler] Processing chat query",
		"query", reqBody.Query,
		"session_id", reqBody.SessionID,
		"user_id", userID,
	)

	// 1. gRPC Request
	grpcReq := &pb.ChatRequest{
		Query:     reqBody.Query,
		SessionId: reqBody.SessionID,
	}

	// Create context with timeout and user ID metadata
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.cfg.ChatTimeout)*time.Second)
	defer cancel()

	// Add user ID to gRPC metadata headers
	ctx = metadata.AppendToOutgoingContext(ctx, "x-user-id", userIDStr)

	stream, err := h.services.ChatService.Chat(ctx, grpcReq)
	if err != nil {
		h.logger.Error("❌ [Handler] Failed to call RAG service",
			"error", err,
			"query", reqBody.Query,
		)
		c.JSON(500, gin.H{"error": "Failed to call RAG service"})
		return
	}

	h.logger.Info("✅ [Handler] RAG service stream established")

	// 2. SSE Header Setup
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	// 3. Stream Responses
	messageCount := 0
	c.Stream(func(w io.Writer) bool {
		resp, err := stream.Recv()
		if err == io.EOF {
			h.logger.Info("✅ [Handler] Chat stream completed",
				"messages_sent", messageCount,
				"query", reqBody.Query,
			)
			return false // Stop streaming on EOF
		}
		if err != nil {
			h.logger.Error("❌ [Handler] Error receiving stream message",
				"error", err,
				"messages_sent", messageCount,
			)
			return false // Stop streaming on error
		}

		messageCount++
		c.SSEvent("message", gin.H{
			"answer":  resp.Answer,
			"sources": resp.SourceDocuments,
			"time":    resp.ProcessingTimeMs,
		})

		return true // Continue streaming
	})
}
