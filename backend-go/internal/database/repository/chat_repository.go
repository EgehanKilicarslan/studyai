package repository

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
)

var (
	ErrSessionNotFound = errors.New("chat session not found")
	ErrMessageNotFound = errors.New("chat message not found")
)

// ChatRepository defines the interface for chat data operations
type ChatRepository interface {
	// Session operations
	CreateSession(session *models.ChatSession) error
	GetSession(sessionID uuid.UUID) (*models.ChatSession, error)
	GetUserSessions(userID uint, organizationID uint, limit int) ([]models.ChatSession, error)
	GetOrCreateSession(sessionID uuid.UUID, userID uint, organizationID uint) (*models.ChatSession, bool, error)

	// Message operations
	CreateMessage(message *models.ChatMessage) error
	GetSessionMessages(sessionID uuid.UUID, limit int) ([]models.ChatMessage, error)
	GetRecentMessages(sessionID uuid.UUID, limit int) ([]models.ChatMessage, error)
	DeleteSessionMessages(sessionID uuid.UUID) error
}

type chatRepository struct {
	db *gorm.DB
}

// NewChatRepository creates a new chat repository instance
func NewChatRepository(db *gorm.DB) ChatRepository {
	return &chatRepository{db: db}
}

// CreateSession creates a new chat session
func (r *chatRepository) CreateSession(session *models.ChatSession) error {
	return r.db.Create(session).Error
}

// GetSession retrieves a chat session by ID
func (r *chatRepository) GetSession(sessionID uuid.UUID) (*models.ChatSession, error) {
	var session models.ChatSession
	err := r.db.Where("id = ?", sessionID).First(&session).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &session, nil
}

// GetUserSessions retrieves sessions for a user in an organization
func (r *chatRepository) GetUserSessions(userID uint, organizationID uint, limit int) ([]models.ChatSession, error) {
	var sessions []models.ChatSession
	query := r.db.Where("user_id = ? AND organization_id = ?", userID, organizationID).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&sessions).Error
	return sessions, err
}

// CreateMessage creates a new chat message
func (r *chatRepository) CreateMessage(message *models.ChatMessage) error {
	return r.db.Create(message).Error
}

// GetSessionMessages retrieves all messages for a session
func (r *chatRepository) GetSessionMessages(sessionID uuid.UUID, limit int) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	query := r.db.Where("session_id = ?", sessionID).
		Order("created_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&messages).Error
	return messages, err
}

// GetRecentMessages retrieves the most recent N messages for a session
func (r *chatRepository) GetRecentMessages(sessionID uuid.UUID, limit int) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage

	// Get the last N messages in descending order, then reverse
	err := r.db.Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error

	if err != nil {
		return nil, err
	}

	// Reverse the slice to get chronological order
	for i := 0; i < len(messages)/2; i++ {
		j := len(messages) - 1 - i
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// DeleteSessionMessages deletes all messages for a session
func (r *chatRepository) DeleteSessionMessages(sessionID uuid.UUID) error {
	return r.db.Where("session_id = ?", sessionID).Delete(&models.ChatMessage{}).Error
}

// GetOrCreateSession gets an existing session or creates a new one if it doesn't exist
func (r *chatRepository) GetOrCreateSession(sessionID uuid.UUID, userID uint, organizationID uint) (*models.ChatSession, bool, error) {
	// Try to get existing session
	session, err := r.GetSession(sessionID)
	if err == nil {
		return session, false, nil // Session exists
	}

	if !errors.Is(err, ErrSessionNotFound) {
		return nil, false, err // Unexpected error
	}

	// Session doesn't exist, create it
	newSession := &models.ChatSession{
		ID:             sessionID,
		UserID:         userID,
		OrganizationID: organizationID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := r.CreateSession(newSession); err != nil {
		return nil, false, err
	}

	return newSession, true, nil // Session created
}
