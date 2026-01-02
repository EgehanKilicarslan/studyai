package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/repository"
)

// DocumentService defines the interface for document business logic
type DocumentService interface {
	// Core operations
	CreateDocument(orgID *uint, groupID *uint, ownerID uint, filename string, contentType string, fileReader io.Reader) (*models.Document, error)
	GetDocument(docID uuid.UUID) (*models.Document, error)
	DeleteDocument(docID uuid.UUID, requesterID uint) error

	// Status management (called by Python callback or async)
	UpdateDocumentStatus(docID uuid.UUID, status models.DocumentStatus, chunksCount int, errorMsg *string) error

	// List operations
	ListDocuments(orgID, userID uint, page, pageSize int) ([]models.Document, int64, error)
	ListGroupDocuments(groupID uint, page, pageSize int) ([]models.Document, int64, error)

	// File path helper
	GetFilePath(docID uuid.UUID) (string, error)
}

type documentService struct {
	docRepo   repository.DocumentRepository
	groupRepo repository.GroupRepository
	orgRepo   repository.OrganizationRepository
	cfg       *config.Config
	logger    *slog.Logger
	uploadDir string
}

// NewDocumentService creates a new document service instance
func NewDocumentService(
	docRepo repository.DocumentRepository,
	groupRepo repository.GroupRepository,
	orgRepo repository.OrganizationRepository,
	cfg *config.Config,
	logger *slog.Logger,
) DocumentService {
	// Create upload directory if it doesn't exist
	uploadDir := "/tmp/studyai/uploads"

	return &documentService{
		docRepo:   docRepo,
		groupRepo: groupRepo,
		orgRepo:   orgRepo,
		cfg:       cfg,
		logger:    logger,
		uploadDir: uploadDir,
	}
}

// ==================== Core Operations ====================

func (s *documentService) CreateDocument(orgID *uint, groupID *uint, ownerID uint, filename string, contentType string, fileReader io.Reader) (*models.Document, error) {
	s.logger.Info("📄 [DocumentService] Creating document",
		"org_id", orgID,
		"group_id", groupID,
		"owner_id", ownerID,
		"filename", filename,
	)

	// Check plan limits only if organization-scoped
	if orgID != nil {
		org, err := s.orgRepo.FindByID(*orgID)
		if err != nil {
			s.logger.Error("❌ [DocumentService] Failed to get organization", "org_id", *orgID, "error", err)
			return nil, fmt.Errorf("failed to get organization: %w", err)
		}

		limits := org.GetPlanLimits()
		_ = limits // TODO: Add actual quota checks
	}

	// Generate a new document ID
	docID := uuid.New()

	// Create directory structure based on scope
	now := time.Now()
	var dirPath string
	if orgID != nil {
		// Organization/group-scoped: /uploads/{org_id}/{year}/{month}/{doc_id}/
		dirPath = filepath.Join(
			s.uploadDir,
			fmt.Sprintf("%d", *orgID),
			fmt.Sprintf("%d", now.Year()),
			fmt.Sprintf("%02d", now.Month()),
			docID.String(),
		)
	} else {
		// User-scoped: /uploads/users/{owner_id}/{year}/{month}/{doc_id}/
		dirPath = filepath.Join(
			s.uploadDir,
			"users",
			fmt.Sprintf("%d", ownerID),
			fmt.Sprintf("%d", now.Year()),
			fmt.Sprintf("%02d", now.Month()),
			docID.String(),
		)
	}

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		s.logger.Error("❌ [DocumentService] Failed to create directory", "path", dirPath, "error", err)
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Sanitize filename and create file path
	safeFilename := filepath.Base(filename)
	filePath := filepath.Join(dirPath, safeFilename)

	// Create the file
	file, err := os.Create(filePath)
	if err != nil {
		s.logger.Error("❌ [DocumentService] Failed to create file", "path", filePath, "error", err)
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write file content and compute hash
	hasher := sha256.New()
	multiWriter := io.MultiWriter(file, hasher)

	bytesWritten, err := io.Copy(multiWriter, fileReader)
	if err != nil {
		// Clean up on error
		os.Remove(filePath)
		s.logger.Error("❌ [DocumentService] Failed to write file", "path", filePath, "error", err)
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Check file size and storage limits only for organization-scoped documents
	if orgID != nil {
		org, err := s.orgRepo.FindByID(*orgID)
		if err != nil {
			os.Remove(filePath)
			return nil, fmt.Errorf("failed to get organization for quota check: %w", err)
		}

		limits := org.GetPlanLimits()

		// Check file size limit
		if limits.MaxFileSize > 0 && bytesWritten > limits.MaxFileSize {
			os.Remove(filePath)
			os.RemoveAll(dirPath)
			s.logger.Warn("⚠️ [DocumentService] File size exceeds limit",
				"file_size", bytesWritten,
				"max_size", limits.MaxFileSize,
				"plan", org.PlanTier,
			)
			return nil, config.NewQuotaError(
				"file_size",
				bytesWritten,
				limits.MaxFileSize,
				fmt.Sprintf("file size %d bytes exceeds plan limit of %d bytes", bytesWritten, limits.MaxFileSize),
			)
		}

		// Check storage quota (before incrementing)
		if limits.MaxStorageBytes > 0 && (org.UsedStorageBytes+bytesWritten) > limits.MaxStorageBytes {
			os.Remove(filePath)
			os.RemoveAll(dirPath)
			s.logger.Warn("⚠️ [DocumentService] Storage quota exceeded",
				"used_storage", org.UsedStorageBytes,
				"file_size", bytesWritten,
				"max_storage", limits.MaxStorageBytes,
				"plan", org.PlanTier,
			)
			return nil, config.NewQuotaError(
				"storage",
				org.UsedStorageBytes+bytesWritten,
				limits.MaxStorageBytes,
				fmt.Sprintf("uploading this file would exceed storage quota (%d + %d > %d bytes)",
					org.UsedStorageBytes, bytesWritten, limits.MaxStorageBytes),
			)
		}
	}

	fileHash := hex.EncodeToString(hasher.Sum(nil))

	// Check for duplicate file (only for organization-scoped documents)
	if orgID != nil {
		existingDoc, err := s.docRepo.FindByHash(*orgID, fileHash)
		if err != nil {
			os.Remove(filePath)
			return nil, err
		}

		if existingDoc != nil {
			// File already exists, clean up the new file
			os.Remove(filePath)
			os.RemoveAll(dirPath) // Remove empty directory

			s.logger.Info("📄 [DocumentService] Duplicate file detected, returning existing document",
				"existing_id", existingDoc.ID,
				"filename", filename,
			)

			// Return the existing document (caller can decide what to do)
			return existingDoc, ErrDocumentAlreadyExists
		}
	}

	// Create document record in database
	doc := &models.Document{
		ID:             docID,
		OrganizationID: orgID,
		GroupID:        groupID,
		OwnerID:        ownerID,
		Name:           filename,
		FilePath:       filePath,
		FileHash:       &fileHash,
		FileSize:       bytesWritten,
		ContentType:    contentType,
		Status:         models.DocumentStatusPending,
	}

	if err := s.docRepo.Create(doc); err != nil {
		// Clean up file on database error
		os.Remove(filePath)
		s.logger.Error("❌ [DocumentService] Failed to create document record", "error", err)
		return nil, err
	}

	// Atomically increment organization storage usage (only for organization-scoped documents)
	if orgID != nil {
		if err := s.orgRepo.IncrementStorage(*orgID, bytesWritten); err != nil {
			// Log but don't fail - document was created successfully
			s.logger.Error("❌ [DocumentService] Failed to update storage usage", "org_id", *orgID, "bytes", bytesWritten, "error", err)
		}
	}

	s.logger.Info("✅ [DocumentService] Document created successfully",
		"doc_id", doc.ID,
		"file_path", filePath,
		"file_size", bytesWritten,
	)

	return doc, nil
}

func (s *documentService) GetDocument(docID uuid.UUID) (*models.Document, error) {
	return s.docRepo.FindByID(docID)
}

func (s *documentService) DeleteDocument(docID uuid.UUID, requesterID uint) error {
	s.logger.Info("🗑️ [DocumentService] Deleting document", "doc_id", docID, "requester_id", requesterID)

	// Get the document
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return err
	}

	// Check if requester is the owner
	if doc.OwnerID != requesterID {
		// Check if requester has permission via group membership
		if doc.GroupID != nil {
			perms, err := s.groupRepo.GetUserPermissionsInGroup(requesterID, *doc.GroupID)
			if err != nil {
				return ErrDocumentPermissionDenied
			}
			hasDeletePerm := false
			for _, p := range perms {
				if p == models.PermDocDelete || p == models.PermGroupAdmin {
					hasDeletePerm = true
					break
				}
			}
			if !hasDeletePerm {
				return ErrDocumentPermissionDenied
			}
		} else {
			return ErrDocumentPermissionDenied
		}
	}

	// Delete file from disk
	if doc.FilePath != "" {
		if err := os.Remove(doc.FilePath); err != nil && !os.IsNotExist(err) {
			s.logger.Warn("⚠️ [DocumentService] Failed to delete file from disk", "path", doc.FilePath, "error", err)
		}
		// Try to remove parent directory if empty
		parentDir := filepath.Dir(doc.FilePath)
		os.Remove(parentDir) // Ignore error, directory might not be empty
	}

	// Delete from database
	if err := s.docRepo.Delete(docID); err != nil {
		return err
	}

	// Atomically decrement organization storage usage (only for organization-scoped documents)
	if doc.FileSize > 0 && doc.OrganizationID != nil {
		if err := s.orgRepo.DecrementStorage(*doc.OrganizationID, doc.FileSize); err != nil {
			// Log but don't fail - document was deleted successfully
			s.logger.Error("❌ [DocumentService] Failed to update storage usage after delete",
				"org_id", *doc.OrganizationID, "bytes", doc.FileSize, "error", err)
		}
	}

	s.logger.Info("✅ [DocumentService] Document deleted successfully", "doc_id", docID)
	return nil
}

// ==================== Status Management ====================

func (s *documentService) UpdateDocumentStatus(docID uuid.UUID, status models.DocumentStatus, chunksCount int, errorMsg *string) error {
	s.logger.Info("📊 [DocumentService] Updating document status",
		"doc_id", docID,
		"status", status,
		"chunks_count", chunksCount,
	)

	return s.docRepo.UpdateStatus(docID, status, chunksCount, errorMsg)
}

// ==================== List Operations ====================

func (s *documentService) ListDocuments(orgID, userID uint, page, pageSize int) ([]models.Document, int64, error) {
	offset := (page - 1) * pageSize
	return s.docRepo.ListByOrganizationAndUser(orgID, userID, offset, pageSize)
}

func (s *documentService) ListGroupDocuments(groupID uint, page, pageSize int) ([]models.Document, int64, error) {
	offset := (page - 1) * pageSize
	return s.docRepo.ListByGroup(groupID, offset, pageSize)
}

// ==================== File Path Helper ====================

func (s *documentService) GetFilePath(docID uuid.UUID) (string, error) {
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return "", err
	}
	return doc.FilePath, nil
}

// Service errors
var (
	ErrDocumentAlreadyExists    = errors.New("document with the same content already exists")
	ErrDocumentPermissionDenied = errors.New("permission denied to access this document")
)
