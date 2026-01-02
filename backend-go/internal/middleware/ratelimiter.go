package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
)

// RateLimiter handles rate limiting for various resources using Redis
type RateLimiter interface {
	// CheckDailyLimit checks if user has exceeded daily message limit
	// Returns: allowed bool, used int64, limit int64, error
	CheckDailyLimit(ctx context.Context, userID uint, orgID uint, limits config.PlanLimits) (bool, int64, int64, error)

	// IncrementDailyCount increments the daily message count for a user
	IncrementDailyCount(ctx context.Context, userID uint) error

	// GetRemainingMessages returns remaining messages for today
	GetRemainingMessages(ctx context.Context, userID uint, orgID uint, limits config.PlanLimits) (int64, error)

	// Close closes the Redis connection
	Close() error
}

type redisRateLimiter struct {
	client *redis.Client
	logger *slog.Logger
}

// NewRateLimiter creates a new Redis-based rate limiter
func NewRateLimiter(cfg *config.Config, logger *slog.Logger) (RateLimiter, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       int(cfg.RedisDB),
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		logger.Error("❌ [RateLimiter] Failed to connect to Redis", "error", err)
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("✅ [RateLimiter] Connected to Redis",
		"host", cfg.RedisHost,
		"port", cfg.RedisPort,
	)

	return &redisRateLimiter{
		client: client,
		logger: logger,
	}, nil
}

// dailyKey generates the Redis key for daily message count
// Format: rate:daily:{userID}:{YYYY-MM-DD}
func dailyKey(userID uint) string {
	today := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("rate:daily:%d:%s", userID, today)
}

func (r *redisRateLimiter) CheckDailyLimit(ctx context.Context, userID uint, orgID uint, limits config.PlanLimits) (bool, int64, int64, error) {
	// If limit is 0 or negative, unlimited
	if limits.DailyMessagesPerUser <= 0 {
		return true, 0, 0, nil
	}

	key := dailyKey(userID)
	count, err := r.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		// Key doesn't exist, user hasn't sent any messages today
		return true, 0, int64(limits.DailyMessagesPerUser), nil
	}
	if err != nil {
		r.logger.Error("❌ [RateLimiter] Failed to get daily count", "error", err, "user_id", userID)
		// On error, allow the request but log it
		return true, 0, int64(limits.DailyMessagesPerUser), err
	}

	allowed := count < int64(limits.DailyMessagesPerUser)
	return allowed, count, int64(limits.DailyMessagesPerUser), nil
}

func (r *redisRateLimiter) IncrementDailyCount(ctx context.Context, userID uint) error {
	key := dailyKey(userID)

	pipe := r.client.Pipeline()

	// Increment the counter
	pipe.Incr(ctx, key)

	// Set expiry to end of day (in UTC) if it's a new key
	// Calculate seconds until midnight UTC
	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	ttl := midnight.Sub(now)

	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	if err != nil {
		r.logger.Error("❌ [RateLimiter] Failed to increment daily count", "error", err, "user_id", userID)
		return err
	}

	return nil
}

func (r *redisRateLimiter) GetRemainingMessages(ctx context.Context, userID uint, orgID uint, limits config.PlanLimits) (int64, error) {
	// If limit is 0 or negative, unlimited
	if limits.DailyMessagesPerUser <= 0 {
		return -1, nil // -1 indicates unlimited
	}

	key := dailyKey(userID)
	count, err := r.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return int64(limits.DailyMessagesPerUser), nil
	}
	if err != nil {
		return 0, err
	}

	remaining := int64(limits.DailyMessagesPerUser) - count
	if remaining < 0 {
		remaining = 0
	}

	return remaining, nil
}

func (r *redisRateLimiter) Close() error {
	return r.client.Close()
}

// NoOpRateLimiter is a rate limiter that always allows requests
// Used when Redis is not available
type NoOpRateLimiter struct {
	logger *slog.Logger
}

// NewNoOpRateLimiter creates a no-op rate limiter
func NewNoOpRateLimiter(logger *slog.Logger) RateLimiter {
	logger.Warn("⚠️ [RateLimiter] Using no-op rate limiter - rate limiting is disabled")
	return &NoOpRateLimiter{logger: logger}
}

func (r *NoOpRateLimiter) CheckDailyLimit(ctx context.Context, userID uint, orgID uint, limits config.PlanLimits) (bool, int64, int64, error) {
	return true, 0, int64(limits.DailyMessagesPerUser), nil
}

func (r *NoOpRateLimiter) IncrementDailyCount(ctx context.Context, userID uint) error {
	return nil
}

func (r *NoOpRateLimiter) GetRemainingMessages(ctx context.Context, userID uint, orgID uint, limits config.PlanLimits) (int64, error) {
	return -1, nil
}

func (r *NoOpRateLimiter) Close() error {
	return nil
}
