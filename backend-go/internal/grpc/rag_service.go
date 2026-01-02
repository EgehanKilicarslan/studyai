package grpc

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	pb "github.com/EgehanKilicarslan/studyai/backend-go/pb"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
)

// RagServiceServer implements the gRPC RagService (Python -> Go).
// Python workers call this service to update document status after processing.
type RagServiceServer struct {
	pb.UnimplementedRagServiceServer
	docRepo repository.DocumentRepository
	orgRepo repository.OrganizationRepository
	logger  *slog.Logger
}

// NewRagServiceServer creates a new RagService server instance.
func NewRagServiceServer(
	docRepo repository.DocumentRepository,
	orgRepo repository.OrganizationRepository,
	logger *slog.Logger,
) *RagServiceServer {
	return &RagServiceServer{
		docRepo: docRepo,
		orgRepo: orgRepo,
		logger:  logger,
	}
}

// UpdateDocumentStatus is called by Python workers after document processing completes.
// It updates the document status in the database and handles quota refunds on ERROR.
func (s *RagServiceServer) UpdateDocumentStatus(
	ctx context.Context,
	req *pb.DocumentStatusRequest,
) (*pb.DocumentStatusResponse, error) {
	s.logger.Info("📊 [RagService] UpdateDocumentStatus called",
		"document_id", req.DocumentId,
		"status", req.Status.String(),
		"chunks_count", req.ChunksCount,
	)

	// Parse document ID
	docID, err := uuid.Parse(req.DocumentId)
	if err != nil {
		s.logger.Error("❌ [RagService] Invalid document ID",
			"document_id", req.DocumentId,
			"error", err,
		)
		return &pb.DocumentStatusResponse{
			Success: false,
			Message: "invalid document_id format",
		}, nil
	}

	// Map proto status to model status
	var status models.DocumentStatus
	switch req.Status {
	case pb.DocumentProcessingStatus_DOCUMENT_STATUS_PROCESSING:
		status = models.DocumentStatusProcessing
	case pb.DocumentProcessingStatus_DOCUMENT_STATUS_COMPLETED:
		status = models.DocumentStatusCompleted
	case pb.DocumentProcessingStatus_DOCUMENT_STATUS_ERROR:
		status = models.DocumentStatusError
	default:
		s.logger.Warn("⚠️ [RagService] Unknown status, defaulting to ERROR",
			"status", req.Status,
		)
		status = models.DocumentStatusError
	}

	// Prepare error message pointer
	var errorMsg *string
	if req.ErrorMessage != "" {
		errorMsg = &req.ErrorMessage
	}

	// If status is ERROR, we need to refund the quota
	if status == models.DocumentStatusError {
		if err := s.handleErrorStatus(ctx, docID); err != nil {
			s.logger.Error("❌ [RagService] Failed to handle error status (quota refund)",
				"document_id", docID,
				"error", err,
			)
			// Continue with status update even if quota refund fails
		}
	}

	// Update document status in the database
	if err := s.docRepo.UpdateStatus(docID, status, int(req.ChunksCount), errorMsg); err != nil {
		s.logger.Error("❌ [RagService] Failed to update document status",
			"document_id", docID,
			"status", status,
			"error", err,
		)
		return &pb.DocumentStatusResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	s.logger.Info("✅ [RagService] Document status updated successfully",
		"document_id", docID,
		"status", status,
		"chunks_count", req.ChunksCount,
	)

	return &pb.DocumentStatusResponse{
		Success: true,
		Message: "status updated successfully",
	}, nil
}

// handleErrorStatus handles quota refund when document processing fails.
// It fetches the document's size and decrements the organization's storage usage.
func (s *RagServiceServer) handleErrorStatus(_ context.Context, docID uuid.UUID) error {
	// Fetch the document to get its size and organization ID
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		s.logger.Error("❌ [RagService] Failed to find document for quota refund",
			"document_id", docID,
			"error", err,
		)
		return err
	}

	// Only refund quota for organization-scoped documents with non-zero file size
	if doc.OrganizationID == nil {
		s.logger.Debug("📝 [RagService] Document is user-scoped, no quota refund needed",
			"document_id", docID,
		)
		return nil
	}

	if doc.FileSize <= 0 {
		s.logger.Debug("📝 [RagService] Document has no file size, no quota refund needed",
			"document_id", docID,
		)
		return nil
	}

	// Decrement the organization's storage usage
	s.logger.Info("💰 [RagService] Refunding storage quota",
		"document_id", docID,
		"org_id", *doc.OrganizationID,
		"file_size", doc.FileSize,
	)

	if err := s.orgRepo.DecrementStorage(*doc.OrganizationID, doc.FileSize); err != nil {
		s.logger.Error("❌ [RagService] Failed to decrement storage",
			"org_id", *doc.OrganizationID,
			"file_size", doc.FileSize,
			"error", err,
		)
		return err
	}

	s.logger.Info("✅ [RagService] Storage quota refunded successfully",
		"document_id", docID,
		"org_id", *doc.OrganizationID,
		"refunded_bytes", doc.FileSize,
	)

	return nil
}
