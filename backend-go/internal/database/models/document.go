package models

import (
	"database/sql/driver"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DocumentStatus represents the processing status of a document
type DocumentStatus string

const (
	DocumentStatusPending    DocumentStatus = "PENDING"
	DocumentStatusProcessing DocumentStatus = "PROCESSING"
	DocumentStatusCompleted  DocumentStatus = "COMPLETED"
	DocumentStatusError      DocumentStatus = "ERROR"
)

// Scan implements the sql.Scanner interface for DocumentStatus
func (s *DocumentStatus) Scan(value interface{}) error {
	if value == nil {
		*s = DocumentStatusPending
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*s = DocumentStatus(v)
	case string:
		*s = DocumentStatus(v)
	default:
		return errors.New("invalid document status type")
	}
	return nil
}

// Value implements the driver.Valuer interface for DocumentStatus
func (s DocumentStatus) Value() (driver.Value, error) {
	return string(s), nil
}

// Document represents a document in the knowledge base
// Go is the source of truth for document metadata
// Supports three scoping modes:
// - User-scoped (private): OrganizationID=NULL, GroupID=NULL
// - Organization-scoped: OrganizationID set, GroupID=NULL
// - Group-scoped: OrganizationID set, GroupID set
type Document struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrganizationID *uint          `gorm:"index" json:"organization_id,omitempty"`
	GroupID        *uint          `gorm:"index" json:"group_id,omitempty"`
	OwnerID        uint           `gorm:"not null;index" json:"owner_id"`
	Name           string         `gorm:"not null;size:512" json:"name"`
	FilePath       string         `gorm:"not null;size:1024" json:"file_path"`
	FileHash       *string        `gorm:"size:64;index" json:"file_hash,omitempty"`
	FileSize       int64          `gorm:"default:0" json:"file_size"`
	ContentType    string         `gorm:"default:application/octet-stream;size:255" json:"content_type"`
	Status         DocumentStatus `gorm:"type:document_status;not null;default:PENDING;index" json:"status"`
	ChunksCount    int            `gorm:"default:0" json:"chunks_count"`
	ErrorMessage   *string        `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Group        *Group        `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	Owner        User          `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
}

// TableName overrides the table name
func (Document) TableName() string {
	return "documents"
}

// BeforeCreate hook to generate UUID if not set
func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

// IsProcessing returns true if the document is still being processed
func (d *Document) IsProcessing() bool {
	return d.Status == DocumentStatusPending || d.Status == DocumentStatusProcessing
}

// IsReady returns true if the document is ready for use
func (d *Document) IsReady() bool {
	return d.Status == DocumentStatusCompleted
}

// HasError returns true if the document processing failed
func (d *Document) HasError() bool {
	return d.Status == DocumentStatusError
}
