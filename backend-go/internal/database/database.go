package database

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/database/models"
)

var DATABASE *gorm.DB

func ConnectDatabase(cfg *config.Config, logger *slog.Logger) error {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=UTC",
		cfg.PostgreSQLHost,
		cfg.PostgreSQLUser,
		cfg.PostgreSQLPassword,
		cfg.PostgreSQLDatabase,
		cfg.PostgreSQLPort,
	)

	logger.Info("🔌 [Database] Connecting to PostgreSQL...",
		"host", cfg.PostgreSQLHost,
		"port", cfg.PostgreSQLPort,
		"database", cfg.PostgreSQLDatabase,
	)

	var db *gorm.DB
	var err error
	maxRetries := 30
	retryDelay := 2 * time.Second

	// Retry connection with exponential backoff
	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			// Test the connection
			sqlDB, err := db.DB()
			if err == nil {
				if err = sqlDB.Ping(); err == nil {
					break
				}
			}
		}

		if i < maxRetries-1 {
			logger.Warn("⏳ [Database] Connection failed, retrying...",
				"attempt", i+1,
				"max_retries", maxRetries,
				"retry_in", retryDelay,
				"error", err,
			)
			time.Sleep(retryDelay)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL after %d attempts: %w", maxRetries, err)
	}

	DATABASE = db

	logger.Info("✅ [Database] Database connection established")

	// Run migrations
	logger.Info("🔄 [Database] Running migrations...")
	if err := db.AutoMigrate(&models.User{}, &models.RefreshToken{}); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	logger.Info("✅ [Database] Migrations completed successfully")

	return nil
}

func GetDatabase() *gorm.DB {
	return DATABASE
}
