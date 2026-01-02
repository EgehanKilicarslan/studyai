package database

import (
	"context"

	"github.com/google/uuid"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
)

// ChatHistoryStore defines the interface for storing and retrieving chat history
type ChatHistoryStore interface {
	GetChatHistory(ctx context.Context, sessionID uuid.UUID) ([]models.ChatMessage, error)
	SetChatHistory(ctx context.Context, sessionID uuid.UUID, messages []models.ChatMessage) error
	AppendChatMessage(ctx context.Context, sessionID uuid.UUID, message models.ChatMessage) error
	DeleteChatHistory(ctx context.Context, sessionID uuid.UUID) error
	Close() error
}
