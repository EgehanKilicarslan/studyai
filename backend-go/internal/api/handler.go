package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/internal/config"
	"github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/internal/rag"
	pb "github.com/EgehanKilicarslan/constructor-rag-assistant/backend-go/pb"
	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests and forwards them to the RAG service
type Handler struct {
	ragClient *rag.Client
	Config    *config.Config
}

// NewHandler injects dependencies (Dependency Injection Go Style)
func NewHandler(ragClient *rag.Client, cfg *config.Config) *Handler {
	return &Handler{ragClient: ragClient, Config: cfg}
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.Config.ChatTimeout)*time.Second)
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
			return false // Stop streaming on EOF
		}
		if err != nil {
			return false // Stop streaming on error
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
	// 1. Parse File
	header, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "File not provided"})
		return
	}

	// 2. Validate File Size
	if header.Size > h.Config.MaxFileSize {
		c.JSON(400, gin.H{"error": fmt.Sprintf("File size exceeds the limit of %dMB", h.Config.MaxFileSize/(1024*1024))})
		return
	}

	file, err := header.Open()
	if err != nil {
		c.JSON(500, gin.H{"error": "Could not open the uploaded file"})
		return
	}
	defer file.Close()

	// 3. gRPC Stream to RAG Service
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.Config.UploadTimeout)*time.Second)
	defer cancel()

	stream, err := h.ragClient.Service.UploadDocument(ctx)
	if err != nil {
		c.JSON(500, gin.H{"error": "Could not connect to RAG Service"})
		return
	}

	// 4. Send Metadata
	reqMeta := &pb.UploadRequest{
		Data: &pb.UploadRequest_Metadata{
			Metadata: &pb.UploadMetadata{
				Filename:    header.Filename,
				ContentType: header.Header.Get("Content-Type"),
			},
		},
	}
	if err := stream.Send(reqMeta); err != nil {
		c.JSON(500, gin.H{"error": "Could not send metadata to RAG Service"})
		return
	}

	// 5. Stream File Chunks
	buffer := make([]byte, 32*1024) // 32KB buffer
	var totalBytesSent int64 = 0

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			c.JSON(500, gin.H{"error": "Error reading the file"})
			return
		}

		// ** Live Size Check **
		totalBytesSent += int64(n)
		if totalBytesSent > h.Config.MaxFileSize {
			c.JSON(400, gin.H{"error": fmt.Sprintf("File size exceeds the limit of %dMB", h.Config.MaxFileSize/(1024*1024))})
			return
		}

		reqChunk := &pb.UploadRequest{
			Data: &pb.UploadRequest_Chunk{
				Chunk: buffer[:n], // Send only the read bytes
			},
		}
		if err := stream.Send(reqChunk); err != nil {
			c.JSON(500, gin.H{"error": "Could not send file chunk to RAG Service"})
			return
		}
	}

	// 6. Close and Receive Response
	resp, err := stream.CloseAndRecv()
	if err != nil {
		c.JSON(500, gin.H{"error": "Error receiving response from RAG Service"})
		return
	}

	c.JSON(200, gin.H{
		"status":  resp.Status,
		"message": resp.Message,
		"chunks":  resp.ChunksCount,
	})
}
