package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChatSession represents a conversation session between a user and the AI
type ChatSession struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID         uint           `gorm:"not null;index:idx_user_org_session" json:"user_id"`
	OrganizationID *uint          `gorm:"index:idx_user_org_session" json:"organization_id,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	User     User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Messages []ChatMessage `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}

// TableName overrides the table name
func (ChatSession) TableName() string {
	return "chat_sessions"
}

// BeforeCreate hook to generate UUID before creating a new session
func (cs *ChatSession) BeforeCreate(tx *gorm.DB) error {
	if cs.ID == uuid.Nil {
		cs.ID = uuid.New()
	}
	return nil
}

// ChatMessage represents a single message in a chat session
type ChatMessage struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	SessionID uuid.UUID      `gorm:"type:uuid;not null;index:idx_session_created" json:"session_id"`
	Role      string         `gorm:"type:varchar(20);not null" json:"role"` // "user" or "assistant"
	Content   string         `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time      `gorm:"index:idx_session_created" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	Session ChatSession `gorm:"foreignKey:SessionID" json:"session,omitempty"`
}

// TableName overrides the table name
func (ChatMessage) TableName() string {
	return "chat_messages"
}

// BeforeCreate hook to generate UUID before creating a new message
func (cm *ChatMessage) BeforeCreate(tx *gorm.DB) error {
	if cm.ID == uuid.Nil {
		cm.ID = uuid.New()
	}
	return nil
}
