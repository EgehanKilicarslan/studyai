package repository

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
)

// DocumentRepository defines the interface for document data operations
type DocumentRepository interface {
	// CRUD operations
	Create(doc *models.Document) error
	FindByID(id uuid.UUID) (*models.Document, error)
	Update(doc *models.Document) error
	Delete(id uuid.UUID) error

	// Status updates
	UpdateStatus(id uuid.UUID, status models.DocumentStatus, chunksCount int, errorMsg *string) error

	// Query operations
	ListByOrganization(orgID uint, offset, limit int) ([]models.Document, int64, error)
	ListByOrganizationAndUser(orgID, userID uint, offset, limit int) ([]models.Document, int64, error)
	ListByGroup(groupID uint, offset, limit int) ([]models.Document, int64, error)
	ListByOwner(ownerID uint, offset, limit int) ([]models.Document, int64, error)
	FindByHash(orgID uint, fileHash string) (*models.Document, error)
	CountByOrganization(orgID uint) (int64, error)

	// Permission check helpers
	IsOwner(docID uuid.UUID, userID uint) (bool, error)
	BelongsToOrganization(docID uuid.UUID, orgID uint) (bool, error)
}

type documentRepository struct {
	db *gorm.DB
}

// NewDocumentRepository creates a new document repository instance
func NewDocumentRepository(db *gorm.DB) DocumentRepository {
	return &documentRepository{db: db}
}

// ==================== CRUD Operations ====================

func (r *documentRepository) Create(doc *models.Document) error {
	return r.db.Create(doc).Error
}

func (r *documentRepository) FindByID(id uuid.UUID) (*models.Document, error) {
	var doc models.Document
	err := r.db.Preload("Organization").
		Preload("Group").
		Preload("Owner").
		First(&doc, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}
	return &doc, nil
}

func (r *documentRepository) Update(doc *models.Document) error {
	result := r.db.Save(doc)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

func (r *documentRepository) Delete(id uuid.UUID) error {
	result := r.db.Delete(&models.Document{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

// ==================== Status Updates ====================

func (r *documentRepository) UpdateStatus(id uuid.UUID, status models.DocumentStatus, chunksCount int, errorMsg *string) error {
	updates := map[string]interface{}{
		"status":       status,
		"chunks_count": chunksCount,
	}
	if errorMsg != nil {
		updates["error_message"] = *errorMsg
	}

	result := r.db.Model(&models.Document{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

// ==================== Query Operations ====================

func (r *documentRepository) ListByOrganization(orgID uint, offset, limit int) ([]models.Document, int64, error) {
	var docs []models.Document
	var total int64

	baseQuery := r.db.Model(&models.Document{}).Where("organization_id = ?", orgID)

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Where("organization_id = ?", orgID).
		Preload("Owner").
		Preload("Group").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&docs).Error

	return docs, total, err
}

func (r *documentRepository) ListByOrganizationAndUser(orgID, userID uint, offset, limit int) ([]models.Document, int64, error) {
	var docs []models.Document
	var total int64

	// Documents the user can see: owned by them OR in a group they belong to OR org-wide (no group)
	baseQuery := r.db.Model(&models.Document{}).
		Where("organization_id = ?", orgID).
		Where("owner_id = ? OR group_id IS NULL OR group_id IN (SELECT group_id FROM group_members WHERE user_id = ?)", userID, userID)

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Where("organization_id = ?", orgID).
		Where("owner_id = ? OR group_id IS NULL OR group_id IN (SELECT group_id FROM group_members WHERE user_id = ?)", userID, userID).
		Preload("Owner").
		Preload("Group").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&docs).Error

	return docs, total, err
}

func (r *documentRepository) ListByGroup(groupID uint, offset, limit int) ([]models.Document, int64, error) {
	var docs []models.Document
	var total int64

	baseQuery := r.db.Model(&models.Document{}).Where("group_id = ?", groupID)

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Where("group_id = ?", groupID).
		Preload("Owner").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&docs).Error

	return docs, total, err
}

func (r *documentRepository) ListByOwner(ownerID uint, offset, limit int) ([]models.Document, int64, error) {
	var docs []models.Document
	var total int64

	baseQuery := r.db.Model(&models.Document{}).Where("owner_id = ?", ownerID)

	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Where("owner_id = ?", ownerID).
		Preload("Group").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&docs).Error

	return docs, total, err
}

func (r *documentRepository) FindByHash(orgID uint, fileHash string) (*models.Document, error) {
	var doc models.Document
	err := r.db.Where("organization_id = ? AND file_hash = ?", orgID, fileHash).First(&doc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Not found is not an error in this case
		}
		return nil, err
	}
	return &doc, nil
}

func (r *documentRepository) CountByOrganization(orgID uint) (int64, error) {
	var count int64
	err := r.db.Model(&models.Document{}).Where("organization_id = ?", orgID).Count(&count).Error
	return count, err
}

// ==================== Permission Helpers ====================

func (r *documentRepository) IsOwner(docID uuid.UUID, userID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.Document{}).
		Where("id = ? AND owner_id = ?", docID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *documentRepository) BelongsToOrganization(docID uuid.UUID, orgID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.Document{}).
		Where("id = ? AND organization_id = ?", docID, orgID).
		Count(&count).Error
	return count > 0, err
}

// Repository errors for documents
var (
	ErrDocumentNotFound = errors.New("document not found")
)
