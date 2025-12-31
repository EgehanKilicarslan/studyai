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

// KnowledgeBaseHandler handles HTTP requests and forwards them to the RAG service
type KnowledgeBaseHandler struct {
	services *grpc.Client
	cfg      *config.Config
	logger   *slog.Logger
}

// NewKnowledgeBaseHandler injects dependencies (Dependency Injection Go Style)
func NewKnowledgeBaseHandler(services *grpc.Client, cfg *config.Config, logger *slog.Logger) *KnowledgeBaseHandler {
	return &KnowledgeBaseHandler{services: services, cfg: cfg, logger: logger}
}

func (h *KnowledgeBaseHandler) UploadHandler(c *gin.Context) {
	// Extract authenticated user ID
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [Handler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Convert userID to string for gRPC metadata
	userIDStr := fmt.Sprintf("%v", userID)

	h.logger.Info("📤 [Handler] Upload request received",
		"user_id", userID,
	)

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

	// 3. gRPC Stream to RAG Service with user ID in metadata
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.cfg.UploadTimeout)*time.Second)
	defer cancel()

	// Add user ID to gRPC metadata headers
	ctx = metadata.AppendToOutgoingContext(ctx, "x-user-id", userIDStr)

	stream, err := h.services.KnowledgeBaseService.UploadDocument(ctx)
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

	// Check if the RAG service returned an error status
	if resp.Status == "error" {
		h.logger.Error("❌ [Handler] Upload failed in RAG Service",
			"filename", header.Filename,
			"status", resp.Status,
			"message", resp.Message,
		)
		c.JSON(500, gin.H{
			"status":  resp.Status,
			"message": resp.Message,
		})
		return
	}

	h.logger.Info("✅ [Handler] Upload completed successfully",
		"filename", header.Filename,
		"status", resp.Status,
		"chunks_processed", resp.ChunksCount,
		"message", resp.Message,
	)

	c.JSON(200, gin.H{
		"status":      resp.Status,
		"message":     resp.Message,
		"chunks":      resp.ChunksCount,
		"document_id": resp.DocumentId,
	})
}

// DeleteHandler: DELETE /api/v1/knowledge-base/:document_id
func (h *KnowledgeBaseHandler) DeleteHandler(c *gin.Context) {
	// Extract authenticated user ID
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [Handler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Convert userID to string for gRPC metadata
	userIDStr := fmt.Sprintf("%v", userID)

	// Get document ID from URL parameter
	documentID := c.Param("document_id")
	if documentID == "" {
		h.logger.Error("❌ [Handler] Document ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Document ID is required"})
		return
	}

	h.logger.Info("🗑️ [Handler] Delete document request received",
		"user_id", userID,
		"document_id", documentID,
	)

	// Create gRPC request
	grpcReq := &pb.DeleteDocumentRequest{
		DocumentId: documentID,
	}

	// Create context with timeout and user ID metadata
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Add user ID to gRPC metadata headers
	ctx = metadata.AppendToOutgoingContext(ctx, "x-user-id", userIDStr)

	// Call gRPC service
	resp, err := h.services.KnowledgeBaseService.DeleteDocument(ctx, grpcReq)
	if err != nil {
		h.logger.Error("❌ [Handler] Failed to delete document via RAG service",
			"error", err,
			"document_id", documentID,
		)
		c.JSON(500, gin.H{"error": "Failed to delete document"})
		return
	}

	h.logger.Info("✅ [Handler] Document deleted successfully",
		"document_id", documentID,
		"status", resp.Status,
		"message", resp.Message,
	)

	c.JSON(200, gin.H{
		"status":  resp.Status,
		"message": resp.Message,
	})
}

// ListHandler: GET /api/v1/knowledge-base
func (h *KnowledgeBaseHandler) ListHandler(c *gin.Context) {
	// Extract authenticated user ID
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [Handler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Convert userID to string for gRPC metadata
	userIDStr := fmt.Sprintf("%v", userID)

	h.logger.Info("📋 [Handler] List documents request received",
		"user_id", userID,
	)

	// Create gRPC request
	grpcReq := &pb.ListDocumentsRequest{}

	// Create context with timeout and user ID metadata
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Add user ID to gRPC metadata headers
	ctx = metadata.AppendToOutgoingContext(ctx, "x-user-id", userIDStr)

	// Call gRPC service
	resp, err := h.services.KnowledgeBaseService.ListDocuments(ctx, grpcReq)
	if err != nil {
		h.logger.Error("❌ [Handler] Failed to list documents via RAG service",
			"error", err,
		)
		c.JSON(500, gin.H{"error": "Failed to list documents"})
		return
	}

	// Convert response to JSON-friendly format
	documents := make([]gin.H, 0, len(resp.Documents))
	for _, doc := range resp.Documents {
		documents = append(documents, gin.H{
			"document_id":      doc.DocumentId,
			"filename":         doc.Filename,
			"upload_timestamp": doc.UploadTimestamp,
			"chunks_count":     doc.ChunksCount,
		})
	}

	h.logger.Info("✅ [Handler] Documents listed successfully",
		"user_id", userID,
		"count", len(documents),
	)

	c.JSON(200, gin.H{
		"documents": documents,
	})
}
