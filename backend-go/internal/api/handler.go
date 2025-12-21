package api

import (
	"context"
	"fmt"
	"io"
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

func (h *Handler) UploadHandler(c *gin.Context) {
	// 1. Get the file from the request
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "Could not get uploaded file"})
		return
	}
	defer file.Close()

	// 2. Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(500, gin.H{"error": "Could not read uploaded file"})
		return
	}

	// 3. gRPC Request
	grpcReq := &pb.UploadRequest{
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		FileContent: content,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 4. Send to RAG service
	resp, err := h.ragClient.Service.UploadDocument(ctx, grpcReq)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("Processing error: %v", err)})
		return
	}

	c.JSON(200, gin.H{
		"status":  resp.Status,
		"message": resp.Message,
		"chunks":  resp.ChunksCount,
	})
}
