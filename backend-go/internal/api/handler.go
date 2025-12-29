package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/rag"
	pb "github.com/EgehanKilicarslan/studyai/backend-go/pb"
	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests and forwards them to the RAG service
type Handler struct {
	ragClient *rag.Client
	cfg       *config.Config
	logger    *slog.Logger
}

// NewHandler injects dependencies (Dependency Injection Go Style)
func NewHandler(ragClient *rag.Client, cfg *config.Config, logger *slog.Logger) *Handler {
	return &Handler{ragClient: ragClient, cfg: cfg, logger: logger}
}

// ChatHandler: POST /api/chat
func (h *Handler) ChatHandler(c *gin.Context) {
	h.logger.Info("📨 [Handler] Chat request received")

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

	h.logger.Info("🔍 [Handler] Processing chat query",
		"query", reqBody.Query,
		"session_id", reqBody.SessionID,
	)

	// 1. gRPC Request
	grpcReq := &pb.ChatRequest{
		Query:     reqBody.Query,
		SessionId: reqBody.SessionID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.cfg.ChatTimeout)*time.Second)
	defer cancel()

	stream, err := h.ragClient.Service.Chat(ctx, grpcReq)
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

func (h *Handler) UploadHandler(c *gin.Context) {
	h.logger.Info("📤 [Handler] Upload request received")

	// 1. Parse File
	header, err := c.FormFile("file")
	if err != nil {
		h.logger.Error("❌ [Handler] File not provided in upload request",
			"error", err,
		)
		c.JSON(400, gin.H{"error": "File not provided"})
		return
	}

	h.logger.Info("📁 [Handler] Processing file upload",
		"filename", header.Filename,
		"size_bytes", header.Size,
		"content_type", header.Header.Get("Content-Type"),
	)

	// 2. Validate File Size
	if header.Size > h.cfg.MaxFileSize {
		h.logger.Warn("⚠️ [Handler] File size exceeds limit",
			"filename", header.Filename,
			"size_bytes", header.Size,
			"max_size_bytes", h.cfg.MaxFileSize,
		)
		c.JSON(400, gin.H{"error": fmt.Sprintf("File size exceeds the limit of %dMB", h.cfg.MaxFileSize/(1024*1024))})
		return
	}

	file, err := header.Open()
	if err != nil {
		h.logger.Error("❌ [Handler] Could not open uploaded file",
			"filename", header.Filename,
			"error", err,
		)
		c.JSON(500, gin.H{"error": "Could not open the uploaded file"})
		return
	}
	defer file.Close()

	// 3. gRPC Stream to RAG Service
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.cfg.UploadTimeout)*time.Second)
	defer cancel()

	stream, err := h.ragClient.Service.UploadDocument(ctx)
	if err != nil {
		h.logger.Error("❌ [Handler] Could not connect to RAG Service for upload",
			"filename", header.Filename,
			"error", err,
		)
		c.JSON(500, gin.H{"error": "Could not connect to RAG Service"})
		return
	}

	h.logger.Info("✅ [Handler] Upload stream established with RAG Service",
		"filename", header.Filename,
	)

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
		h.logger.Error("❌ [Handler] Could not send metadata to RAG Service",
			"filename", header.Filename,
			"error", err,
		)
		c.JSON(500, gin.H{"error": "Could not send metadata to RAG Service"})
		return
	}

	h.logger.Info("📋 [Handler] Metadata sent, streaming file chunks...",
		"filename", header.Filename,
	)

	// 5. Stream File Chunks
	buffer := make([]byte, 32*1024) // 32KB buffer
	var totalBytesSent int64 = 0
	chunkCount := 0

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			h.logger.Error("❌ [Handler] Error reading file",
				"filename", header.Filename,
				"error", err,
			)
			c.JSON(500, gin.H{"error": "Error reading the file"})
			return
		}

		// ** Live Size Check **
		totalBytesSent += int64(n)
		if totalBytesSent > h.cfg.MaxFileSize {
			h.logger.Warn("⚠️ [Handler] File size exceeds limit during streaming",
				"filename", header.Filename,
				"bytes_sent", totalBytesSent,
				"max_size_bytes", h.cfg.MaxFileSize,
			)
			c.JSON(400, gin.H{"error": fmt.Sprintf("File size exceeds the limit of %dMB", h.cfg.MaxFileSize/(1024*1024))})
			return
		}

		reqChunk := &pb.UploadRequest{
			Data: &pb.UploadRequest_Chunk{
				Chunk: buffer[:n], // Send only the read bytes
			},
		}
		if err := stream.Send(reqChunk); err != nil {
			h.logger.Error("❌ [Handler] Could not send file chunk to RAG Service",
				"filename", header.Filename,
				"chunk_number", chunkCount,
				"error", err,
			)
			c.JSON(500, gin.H{"error": "Could not send file chunk to RAG Service"})
			return
		}
		chunkCount++
	}

	h.logger.Info("✅ [Handler] All chunks sent, waiting for RAG Service response",
		"filename", header.Filename,
		"total_bytes_sent", totalBytesSent,
		"chunk_count", chunkCount,
	)

	// 6. Close and Receive Response
	resp, err := stream.CloseAndRecv()
	if err != nil {
		h.logger.Error("❌ [Handler] Error receiving response from RAG Service",
			"filename", header.Filename,
			"error", err,
		)
		c.JSON(500, gin.H{"error": "Error receiving response from RAG Service"})
		return
	}

	h.logger.Info("✅ [Handler] Upload completed successfully",
		"filename", header.Filename,
		"status", resp.Status,
		"chunks_processed", resp.ChunksCount,
		"message", resp.Message,
	)

	c.JSON(200, gin.H{
		"status":  resp.Status,
		"message": resp.Message,
		"chunks":  resp.ChunksCount,
	})
}
