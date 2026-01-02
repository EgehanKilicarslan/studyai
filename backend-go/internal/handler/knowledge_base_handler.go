package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/service"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/grpc"
	pb "github.com/EgehanKilicarslan/studyai/backend-go/pb"
	"github.com/google/uuid"
)

// KnowledgeBaseHandler handles HTTP requests for document management
// Go is the source of truth for documents, Python only processes them
type KnowledgeBaseHandler struct {
	grpcClient *grpc.Client
	docService service.DocumentService
	groupRepo  repository.GroupRepository
	cfg        *config.Config
	logger     *slog.Logger
}

// NewKnowledgeBaseHandler creates a new knowledge base handler
func NewKnowledgeBaseHandler(
	grpcClient *grpc.Client,
	docService service.DocumentService,
	groupRepo repository.GroupRepository,
	cfg *config.Config,
	logger *slog.Logger,
) *KnowledgeBaseHandler {
	return &KnowledgeBaseHandler{
		grpcClient: grpcClient,
		docService: docService,
		groupRepo:  groupRepo,
		cfg:        cfg,
		logger:     logger,
	}
}

// ==================== Request/Response DTOs ====================

type UploadDocumentRequest struct {
	OrganizationID uint  `form:"organization_id" binding:"required"`
	GroupID        *uint `form:"group_id"`
}

type DocumentResponse struct {
	ID             string  `json:"id"`
	OrganizationID *uint   `json:"organization_id,omitempty"`
	GroupID        *uint   `json:"group_id,omitempty"`
	OwnerID        uint    `json:"owner_id"`
	Name           string  `json:"name"`
	Status         string  `json:"status"`
	ChunksCount    int     `json:"chunks_count"`
	FileSize       int64   `json:"file_size"`
	ContentType    string  `json:"content_type"`
	ErrorMessage   *string `json:"error_message,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

// ==================== Upload Handler ====================

// UploadHandler handles POST /api/v1/upload
// 1. Receives file and saves to disk
// 2. Creates document record in Postgres with PENDING status
// 3. Calls Python gRPC ProcessDocument to parse and index
// 4. Updates status based on Python response
func (h *KnowledgeBaseHandler) UploadHandler(c *gin.Context) {
	// Extract authenticated user ID
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [KnowledgeBaseHandler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userIDUint := userID.(uint)

	// Parse organization_id and group_id (check headers first, then form data)
	// Both are optional for user-scoped documents
	var organizationID *uint
	orgIDStr := c.GetHeader("x-organization-id")
	if orgIDStr == "" {
		orgIDStr = c.PostForm("organization_id")
	}
	if orgIDStr != "" {
		orgID, err := strconv.ParseUint(orgIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization_id"})
			return
		}
		orgIDUint := uint(orgID)
		organizationID = &orgIDUint
	}

	var groupID *uint
	groupIDStr := c.GetHeader("x-group-id")
	if groupIDStr == "" {
		groupIDStr = c.PostForm("group_id")
	}
	if groupIDStr != "" {
		gid, err := strconv.ParseUint(groupIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group_id"})
			return
		}
		gidUint := uint(gid)
		groupID = &gidUint
	}

	// Validate scoping: group_id requires organization_id
	if groupID != nil && organizationID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "group_id requires organization_id"})
		return
	}

	h.logger.Info("📤 [KnowledgeBaseHandler] Upload request received",
		"user_id", userIDUint,
		"organization_id", organizationID,
		"group_id", groupID,
	)

	// Parse file from multipart form
	header, err := c.FormFile("file")
	if err != nil {
		h.logger.Error("❌ [KnowledgeBaseHandler] File not provided", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "File not provided"})
		return
	}

	h.logger.Info("📁 [KnowledgeBaseHandler] Processing file",
		"filename", header.Filename,
		"size_bytes", header.Size,
		"content_type", header.Header.Get("Content-Type"),
	)

	// Validate file size
	if header.Size > h.cfg.MaxFileSize {
		h.logger.Warn("⚠️ [KnowledgeBaseHandler] File size exceeds limit",
			"size_bytes", header.Size,
			"max_size_bytes", h.cfg.MaxFileSize,
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("File size exceeds the limit of %dMB", h.cfg.MaxFileSize/(1024*1024)),
		})
		return
	}

	// Open the file
	file, err := header.Open()
	if err != nil {
		h.logger.Error("❌ [KnowledgeBaseHandler] Could not open file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not open file"})
		return
	}
	defer file.Close()

	// Step 1 & 2: Save file to disk and create document record
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	doc, err := h.docService.CreateDocument(organizationID, groupID, userIDUint, header.Filename, contentType, file)
	if err != nil {
		if errors.Is(err, service.ErrDocumentAlreadyExists) {
			// Document already exists, return success with existing doc
			h.logger.Info("📄 [KnowledgeBaseHandler] Document already exists", "doc_id", doc.ID)
			c.JSON(http.StatusOK, gin.H{
				"status":      "success",
				"message":     "Document already exists",
				"document_id": doc.ID.String(),
				"document":    h.mapDocumentToResponse(doc),
			})
			return
		}
		h.logger.Error("❌ [KnowledgeBaseHandler] Failed to create document", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save document"})
		return
	}

	h.logger.Info("✅ [KnowledgeBaseHandler] Document saved to disk",
		"doc_id", doc.ID,
		"file_path", doc.FilePath,
	)

	// Step 3: Call Python gRPC to process the document
	grpcOrgID := uint32(0)
	if organizationID != nil {
		grpcOrgID = uint32(*organizationID)
	}

	grpcGroupID := uint32(0)
	if groupID != nil {
		grpcGroupID = uint32(*groupID)
	}

	grpcReq := &pb.ProcessDocumentRequest{
		DocumentId:     doc.ID.String(),
		FilePath:       doc.FilePath,
		Filename:       doc.Name,
		ContentType:    doc.ContentType,
		OrganizationId: grpcOrgID,
		GroupId:        grpcGroupID,
		OwnerId:        uint32(userIDUint),
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(h.cfg.UploadTimeout)*time.Second)
	defer cancel()

	h.logger.Info("🔄 [KnowledgeBaseHandler] Calling Python ProcessDocument",
		"doc_id", doc.ID,
	)

	resp, err := h.grpcClient.KnowledgeBaseService.ProcessDocument(ctx, grpcReq)
	if err != nil {
		// Python processing failed, update status to ERROR
		errMsg := err.Error()
		h.docService.UpdateDocumentStatus(doc.ID, models.DocumentStatusError, 0, &errMsg)

		h.logger.Error("❌ [KnowledgeBaseHandler] Python processing failed", "error", err, "doc_id", doc.ID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":       "Document processing failed",
			"document_id": doc.ID.String(),
			"status":      "error",
		})
		return
	}

	// Step 4: Update status based on Python response
	if resp.Status == "success" {
		h.docService.UpdateDocumentStatus(doc.ID, models.DocumentStatusCompleted, int(resp.ChunksCount), nil)

		h.logger.Info("✅ [KnowledgeBaseHandler] Document processed successfully",
			"doc_id", doc.ID,
			"chunks_count", resp.ChunksCount,
		)

		// Reload document to get updated fields
		doc, _ = h.docService.GetDocument(doc.ID)

		c.JSON(http.StatusCreated, gin.H{
			"status":       "success",
			"message":      resp.Message,
			"document_id":  doc.ID.String(),
			"chunks_count": resp.ChunksCount,
			"document":     h.mapDocumentToResponse(doc),
		})
	} else {
		errMsg := resp.Message
		h.docService.UpdateDocumentStatus(doc.ID, models.DocumentStatusError, 0, &errMsg)

		h.logger.Error("❌ [KnowledgeBaseHandler] Python returned error",
			"doc_id", doc.ID,
			"message", resp.Message,
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"status":      "error",
			"message":     resp.Message,
			"document_id": doc.ID.String(),
		})
	}
}

// ==================== List Handler ====================

// ListHandler handles GET /api/v1/knowledge-base
// Lists documents directly from Postgres (Go is source of truth)
func (h *KnowledgeBaseHandler) ListHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [KnowledgeBaseHandler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userIDUint := userID.(uint)

	// Get organization_id from query params
	orgIDStr := c.Query("organization_id")
	if orgIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization_id is required"})
		return
	}
	orgID, err := strconv.ParseUint(orgIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization_id"})
		return
	}

	// Pagination
	page := 1
	pageSize := 20
	if p, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && p > 0 {
		page = p
	}
	if ps, err := strconv.Atoi(c.DefaultQuery("page_size", "20")); err == nil && ps > 0 && ps <= 100 {
		pageSize = ps
	}

	h.logger.Info("📋 [KnowledgeBaseHandler] Listing documents",
		"user_id", userIDUint,
		"organization_id", orgID,
		"page", page,
		"page_size", pageSize,
	)

	// List documents from Postgres
	docs, total, err := h.docService.ListDocuments(uint(orgID), userIDUint, page, pageSize)
	if err != nil {
		h.logger.Error("❌ [KnowledgeBaseHandler] Failed to list documents", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list documents"})
		return
	}

	// Map to response
	response := make([]DocumentResponse, len(docs))
	for i, doc := range docs {
		response[i] = h.mapDocumentToResponse(&doc)
	}

	h.logger.Info("✅ [KnowledgeBaseHandler] Documents listed",
		"count", len(docs),
		"total", total,
	)

	c.JSON(http.StatusOK, gin.H{
		"documents":   response,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
	})
}

// ==================== Delete Handler ====================

// DeleteHandler handles DELETE /api/v1/knowledge-base/:document_id
// 1. Deletes from Postgres
// 2. Calls Python gRPC Delete to remove from vector store
func (h *KnowledgeBaseHandler) DeleteHandler(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		h.logger.Error("❌ [KnowledgeBaseHandler] User ID not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userIDUint := userID.(uint)

	documentIDStr := c.Param("document_id")
	if documentIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Document ID is required"})
		return
	}

	docID, err := uuid.Parse(documentIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid document ID format"})
		return
	}

	h.logger.Info("🗑️ [KnowledgeBaseHandler] Delete request",
		"user_id", userIDUint,
		"document_id", docID,
	)

	// Get document first to verify it exists
	doc, err := h.docService.GetDocument(docID)
	if err != nil {
		if errors.Is(err, repository.ErrDocumentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Document not found"})
			return
		}
		h.logger.Error("❌ [KnowledgeBaseHandler] Failed to get document", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get document"})
		return
	}

	// Delete from Postgres (this also checks permissions)
	if err := h.docService.DeleteDocument(docID, userIDUint); err != nil {
		if errors.Is(err, service.ErrDocumentPermissionDenied) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
			return
		}
		h.logger.Error("❌ [KnowledgeBaseHandler] Failed to delete document", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete document"})
		return
	}

	// Call Python gRPC to delete from vector store
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	grpcReq := &pb.DeleteDocumentRequest{
		DocumentId: doc.ID.String(),
	}

	resp, err := h.grpcClient.KnowledgeBaseService.DeleteDocument(ctx, grpcReq)
	if err != nil {
		// Log but don't fail - document is already deleted from Postgres
		h.logger.Warn("⚠️ [KnowledgeBaseHandler] Failed to delete from vector store", "error", err)
	} else {
		h.logger.Info("✅ [KnowledgeBaseHandler] Deleted from vector store",
			"status", resp.Status,
			"message", resp.Message,
		)
	}

	h.logger.Info("✅ [KnowledgeBaseHandler] Document deleted", "document_id", docID)

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Document deleted successfully",
	})
}

// ==================== Helper Methods ====================

func (h *KnowledgeBaseHandler) mapDocumentToResponse(doc *models.Document) DocumentResponse {
	resp := DocumentResponse{
		ID:             doc.ID.String(),
		OrganizationID: doc.OrganizationID,
		GroupID:        doc.GroupID,
		OwnerID:        doc.OwnerID,
		Name:           doc.Name,
		Status:         string(doc.Status),
		ChunksCount:    doc.ChunksCount,
		FileSize:       doc.FileSize,
		ContentType:    doc.ContentType,
		ErrorMessage:   doc.ErrorMessage,
		CreatedAt:      doc.CreatedAt.Format(time.RFC3339),
	}
	return resp
}
