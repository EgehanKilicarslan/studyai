package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
)

// RedisClient wraps the redis client with helper methods for chat history
type RedisClient struct {
	client *redis.Client
	logger *slog.Logger
	cfg    *config.Config
}

// NewRedisClient creates a new Redis client instance
func NewRedisClient(cfg *config.Config, logger *slog.Logger) (*RedisClient, error) {
	logger.Info("🔌 [Redis] Connecting to Redis...",
		"host", cfg.RedisHost,
		"port", cfg.RedisPort,
		"db", cfg.RedisDB,
	)

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       int(cfg.RedisDB),
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("✅ [Redis] Redis connection established")

	return &RedisClient{
		client: client,
		logger: logger,
		cfg:    cfg,
	}, nil
}

// NewRedisClientForTesting creates a Redis client with a provided redis.Client (for testing)
func NewRedisClientForTesting(client *redis.Client, cfg *config.Config, logger *slog.Logger) *RedisClient {
	return &RedisClient{
		client: client,
		logger: logger,
		cfg:    cfg,
	}
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// sessionKey generates a Redis key for a chat session
func sessionKey(sessionID uuid.UUID) string {
	return fmt.Sprintf("session:%s:history", sessionID.String())
}

// GetChatHistory retrieves chat history from Redis for a given session
// Returns an empty slice if the key doesn't exist or on error
func (r *RedisClient) GetChatHistory(ctx context.Context, sessionID uuid.UUID) ([]models.ChatMessage, error) {
	key := sessionKey(sessionID)

	// Get all messages from the list
	results, err := r.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		if err == redis.Nil {
			// Key doesn't exist, return empty slice
			return []models.ChatMessage{}, nil
		}
		r.logger.Error("❌ [Redis] Failed to get chat history",
			"session_id", sessionID,
			"error", err,
		)
		return []models.ChatMessage{}, err
	}

	// Deserialize messages
	messages := make([]models.ChatMessage, 0, len(results))
	for _, result := range results {
		var msg models.ChatMessage
		if err := json.Unmarshal([]byte(result), &msg); err != nil {
			r.logger.Warn("⚠️ [Redis] Failed to unmarshal message, skipping",
				"session_id", sessionID,
				"error", err,
			)
			continue
		}
		messages = append(messages, msg)
	}

	r.logger.Debug("📖 [Redis] Retrieved chat history",
		"session_id", sessionID,
		"message_count", len(messages),
	)

	return messages, nil
}

// SetChatHistory stores a complete chat history in Redis with TTL
func (r *RedisClient) SetChatHistory(ctx context.Context, sessionID uuid.UUID, messages []models.ChatMessage) error {
	if len(messages) == 0 {
		return nil
	}

	key := sessionKey(sessionID)

	// Use pipeline for efficiency
	pipe := r.client.Pipeline()

	// Delete existing key first
	pipe.Del(ctx, key)

	// Push all messages
	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			r.logger.Warn("⚠️ [Redis] Failed to marshal message, skipping",
				"session_id", sessionID,
				"message_id", msg.ID,
				"error", err,
			)
			continue
		}
		pipe.RPush(ctx, key, string(data))
	}

	// Set TTL from config
	ttl := time.Duration(r.cfg.ChatHistoryTTL) * time.Second
	pipe.Expire(ctx, key, ttl)

	// Execute pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		r.logger.Error("❌ [Redis] Failed to set chat history",
			"session_id", sessionID,
			"error", err,
		)
		return err
	}

	r.logger.Debug("💾 [Redis] Stored chat history",
		"session_id", sessionID,
		"message_count", len(messages),
		"ttl", time.Duration(r.cfg.ChatHistoryTTL)*time.Second,
	)

	return nil
}

// AppendChatMessage appends a single message to the chat history in Redis
func (r *RedisClient) AppendChatMessage(ctx context.Context, sessionID uuid.UUID, message models.ChatMessage) error {
	key := sessionKey(sessionID)

	// Serialize message
	data, err := json.Marshal(message)
	if err != nil {
		r.logger.Error("❌ [Redis] Failed to marshal message",
			"session_id", sessionID,
			"error", err,
		)
		return err
	}

	// Use pipeline for atomicity
	pipe := r.client.Pipeline()

	// Append message
	pipe.RPush(ctx, key, string(data))

	// Refresh TTL from config
	ttl := time.Duration(r.cfg.ChatHistoryTTL) * time.Second
	pipe.Expire(ctx, key, ttl)

	// Execute pipeline
	if _, err := pipe.Exec(ctx); err != nil {
		r.logger.Error("❌ [Redis] Failed to append message",
			"session_id", sessionID,
			"error", err,
		)
		return err
	}

	r.logger.Debug("➕ [Redis] Appended message to history",
		"session_id", sessionID,
		"role", message.Role,
	)

	return nil
}

// DeleteChatHistory removes a chat history from Redis
func (r *RedisClient) DeleteChatHistory(ctx context.Context, sessionID uuid.UUID) error {
	key := sessionKey(sessionID)

	if err := r.client.Del(ctx, key).Err(); err != nil {
		r.logger.Error("❌ [Redis] Failed to delete chat history",
			"session_id", sessionID,
			"error", err,
		)
		return err
	}

	r.logger.Debug("🗑️ [Redis] Deleted chat history",
		"session_id", sessionID,
	)

	return nil
}

// GetClient returns the underlying Redis client (for advanced use cases)
func (r *RedisClient) GetClient() *redis.Client {
	return r.client
}
