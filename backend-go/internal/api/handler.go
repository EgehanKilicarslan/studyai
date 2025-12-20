package api

import (
	"context"
	"net/http"
	"time"

	"github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/internal/rag"
	pb "github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/pb"
	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests and forwards them to the RAG service
type Handler struct {
	ragClient *rag.Client
}

// NewHandler injects dependencies (Dependency Injection Go Style)
func NewHandler(ragClient *rag.Client) *Handler {
	return &Handler{ragClient: ragClient}
}

// ChatHandler: POST /api/chat
func (h *Handler) ChatHandler(c *gin.Context) {
	var reqBody struct {
		Query     string `json:"query"`
		SessionID string `json:"session_id"`
	}

	if err := c.BindJSON(&reqBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// gRPC Request
	grpcReq := &pb.ChatRequest{
		Query:     reqBody.Query,
		SessionId: reqBody.SessionID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Simple check to prevent panic if Client is nil (e.g., in mock tests)
	if h.ragClient == nil {
		c.JSON(500, gin.H{"error": "RAG Service not connected"})
		return
	}

	resp, err := h.ragClient.Service.Chat(ctx, grpcReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"answer":  resp.Answer,
		"sources": resp.SourceDocuments,
	})
}
