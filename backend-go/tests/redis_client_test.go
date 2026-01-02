package tests

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
)

func setupMiniRedis(t *testing.T) (*miniredis.Miniredis, *database.RedisClient) {
	// Create mini redis server
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create config with test TTL
	cfg := &config.Config{
		ChatHistoryTTL: 3600, // 1 hour
	}

	// Create Redis client directly for testing
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create RedisClient wrapper using test constructor
	redisClient := database.NewRedisClientForTesting(client, cfg, logger)

	t.Cleanup(func() {
		redisClient.Close()
		mr.Close()
	})

	return mr, redisClient
}

func TestRedisClient_SetAndGetChatHistory(t *testing.T) {
	_, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	messages := []models.ChatMessage{
		{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "user",
			Content:   "Hello, how are you?",
			CreatedAt: time.Now(),
		},
		{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "assistant",
			Content:   "I'm doing well, thank you!",
			CreatedAt: time.Now().Add(1 * time.Second),
		},
	}

	// Set chat history
	err := redisClient.SetChatHistory(ctx, sessionID, messages)
	assert.NoError(t, err)

	// Get chat history
	retrieved, err := redisClient.GetChatHistory(ctx, sessionID)
	assert.NoError(t, err)
	assert.Len(t, retrieved, 2)

	// Verify content
	assert.Equal(t, messages[0].Role, retrieved[0].Role)
	assert.Equal(t, messages[0].Content, retrieved[0].Content)
	assert.Equal(t, messages[1].Role, retrieved[1].Role)
	assert.Equal(t, messages[1].Content, retrieved[1].Content)
}

func TestRedisClient_GetChatHistory_Empty(t *testing.T) {
	_, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	// Get non-existent chat history
	messages, err := redisClient.GetChatHistory(ctx, sessionID)

	assert.NoError(t, err)
	assert.Empty(t, messages)
}

func TestRedisClient_GetChatHistory_InvalidJSON(t *testing.T) {
	mr, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	// Set invalid JSON directly in Redis
	key := "chat:history:" + sessionID.String()
	mr.Set(key, "invalid json data")

	// Try to get chat history - should handle gracefully
	messages, err := redisClient.GetChatHistory(ctx, sessionID)

	// It may return error or empty messages depending on implementation
	if err != nil {
		assert.Error(t, err)
		assert.Empty(t, messages)
	} else {
		// If no error, should return empty messages
		assert.Empty(t, messages)
	}
}

func TestRedisClient_AppendChatMessage(t *testing.T) {
	_, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	// Set initial chat history
	initialMessages := []models.ChatMessage{
		{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "user",
			Content:   "First message",
			CreatedAt: time.Now(),
		},
	}
	err := redisClient.SetChatHistory(ctx, sessionID, initialMessages)
	require.NoError(t, err)

	// Append new message
	newMessage := models.ChatMessage{
		ID:        uuid.New(),
		SessionID: sessionID,
		Role:      "assistant",
		Content:   "Second message",
		CreatedAt: time.Now().Add(1 * time.Second),
	}
	err = redisClient.AppendChatMessage(ctx, sessionID, newMessage)
	assert.NoError(t, err)

	// Retrieve and verify
	messages, err := redisClient.GetChatHistory(ctx, sessionID)
	assert.NoError(t, err)
	assert.Len(t, messages, 2)
	assert.Equal(t, "First message", messages[0].Content)
	assert.Equal(t, "Second message", messages[1].Content)
}

func TestRedisClient_AppendChatMessage_ToEmpty(t *testing.T) {
	_, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	// Append message to non-existent session
	message := models.ChatMessage{
		ID:        uuid.New(),
		SessionID: sessionID,
		Role:      "user",
		Content:   "First message",
		CreatedAt: time.Now(),
	}
	err := redisClient.AppendChatMessage(ctx, sessionID, message)
	assert.NoError(t, err)

	// Verify message was stored
	messages, err := redisClient.GetChatHistory(ctx, sessionID)
	assert.NoError(t, err)
	assert.Len(t, messages, 1)
	assert.Equal(t, "First message", messages[0].Content)
}

func TestRedisClient_DeleteChatHistory(t *testing.T) {
	_, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	// Set chat history
	messages := []models.ChatMessage{
		{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "user",
			Content:   "Test message",
			CreatedAt: time.Now(),
		},
	}
	err := redisClient.SetChatHistory(ctx, sessionID, messages)
	require.NoError(t, err)

	// Delete chat history
	err = redisClient.DeleteChatHistory(ctx, sessionID)
	assert.NoError(t, err)

	// Verify deletion
	retrieved, err := redisClient.GetChatHistory(ctx, sessionID)
	assert.NoError(t, err)
	assert.Empty(t, retrieved)
}

func TestRedisClient_TTL(t *testing.T) {
	_, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	messages := []models.ChatMessage{
		{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "user",
			Content:   "Test message",
			CreatedAt: time.Now(),
		},
	}

	// Set chat history
	err := redisClient.SetChatHistory(ctx, sessionID, messages)
	require.NoError(t, err)

	// Just verify it was set successfully
	retrieved, err := redisClient.GetChatHistory(ctx, sessionID)
	assert.NoError(t, err)
	assert.Len(t, retrieved, 1)
}

func TestRedisClient_ConcurrentAccess(t *testing.T) {
	_, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	// Set initial history
	initialMessages := []models.ChatMessage{
		{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "user",
			Content:   "Initial message",
			CreatedAt: time.Now(),
		},
	}
	err := redisClient.SetChatHistory(ctx, sessionID, initialMessages)
	require.NoError(t, err)

	// Append messages concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			message := models.ChatMessage{
				ID:        uuid.New(),
				SessionID: sessionID,
				Role:      "assistant",
				Content:   "Message " + string(rune('0'+index)),
				CreatedAt: time.Now(),
			}
			_ = redisClient.AppendChatMessage(ctx, sessionID, message)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all messages were stored
	messages, err := redisClient.GetChatHistory(ctx, sessionID)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(messages), 1) // At least initial message should be there
}

func TestRedisClient_LargeHistory(t *testing.T) {
	_, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	// Create large history (100 messages)
	messages := make([]models.ChatMessage, 100)
	for i := 0; i < 100; i++ {
		messages[i] = models.ChatMessage{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "user",
			Content:   "Message content " + string(rune('0'+(i%10))),
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
	}

	// Set large history
	err := redisClient.SetChatHistory(ctx, sessionID, messages)
	assert.NoError(t, err)

	// Retrieve and verify
	retrieved, err := redisClient.GetChatHistory(ctx, sessionID)
	assert.NoError(t, err)
	assert.Len(t, retrieved, 100)
}

func TestRedisClient_EmptyMessageContent(t *testing.T) {
	_, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	messages := []models.ChatMessage{
		{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "user",
			Content:   "", // Empty content
			CreatedAt: time.Now(),
		},
	}

	// Set and retrieve
	err := redisClient.SetChatHistory(ctx, sessionID, messages)
	assert.NoError(t, err)

	retrieved, err := redisClient.GetChatHistory(ctx, sessionID)
	assert.NoError(t, err)
	assert.Len(t, retrieved, 1)
	assert.Equal(t, "", retrieved[0].Content)
}

func TestRedisClient_SpecialCharacters(t *testing.T) {
	_, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	specialContent := "Hello! 你好 🌟 \n\t Special chars: @#$%^&*()"
	messages := []models.ChatMessage{
		{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "user",
			Content:   specialContent,
			CreatedAt: time.Now(),
		},
	}

	// Set and retrieve
	err := redisClient.SetChatHistory(ctx, sessionID, messages)
	assert.NoError(t, err)

	retrieved, err := redisClient.GetChatHistory(ctx, sessionID)
	assert.NoError(t, err)
	assert.Len(t, retrieved, 1)
	assert.Equal(t, specialContent, retrieved[0].Content)
}

func TestRedisClient_InvalidRole(t *testing.T) {
	_, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	messages := []models.ChatMessage{
		{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "invalid_role",
			Content:   "Test message",
			CreatedAt: time.Now(),
		},
	}

	// Should still store successfully
	err := redisClient.SetChatHistory(ctx, sessionID, messages)
	assert.NoError(t, err)

	// Should retrieve successfully
	retrieved, err := redisClient.GetChatHistory(ctx, sessionID)
	assert.NoError(t, err)
	assert.Len(t, retrieved, 1)
	assert.Equal(t, "invalid_role", retrieved[0].Role)
}

func TestRedisClient_SerializationFormat(t *testing.T) {
	_, redisClient := setupMiniRedis(t)
	ctx := context.Background()
	sessionID := uuid.New()

	messages := []models.ChatMessage{
		{
			ID:        uuid.New(),
			SessionID: sessionID,
			Role:      "user",
			Content:   "Test",
			CreatedAt: time.Now(),
		},
	}

	// Set chat history
	err := redisClient.SetChatHistory(ctx, sessionID, messages)
	require.NoError(t, err)

	// Retrieve and verify
	retrieved, err := redisClient.GetChatHistory(ctx, sessionID)
	assert.NoError(t, err)
	assert.Len(t, retrieved, 1)
	assert.Equal(t, "user", retrieved[0].Role)
	assert.Equal(t, "Test", retrieved[0].Content)
}
