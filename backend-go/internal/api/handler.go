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

	// 1. gRPC Request
	grpcReq := &pb.ChatRequest{
		Query:     reqBody.Query,
		SessionId: reqBody.SessionID,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := h.ragClient.Service.Chat(ctx, grpcReq)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to call RAG service"})
		return
	}

	// 2. SSE Header Setup
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	// 3. Stream Responses
	c.Stream(func(w io.Writer) bool {
		resp, err := stream.Recv()
		if err == io.EOF {
			return false // End of stream
		}
		if err != nil {
			return false // Error occurred
		}

		c.SSEvent("message", gin.H{
			"answer":  resp.Answer,
			"sources": resp.SourceDocuments,
			"time":    resp.ProcessingTimeMs,
		})

		return true // Continue streaming
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
