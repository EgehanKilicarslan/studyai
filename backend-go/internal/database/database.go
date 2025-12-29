package database

import (
	"embed"
	"fmt"
	"log/slog"
	"time"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

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

	// Run migrations using goose
	logger.Info("🔄 [Database] Running migrations...")
	if err := runMigrations(db, logger); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Info("✅ [Database] Migrations completed successfully")

	return nil
}

func runMigrations(gormDB *gorm.DB, logger *slog.Logger) error {
	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("failed to run goose migrations: %w", err)
	}

	return nil
}

func GetDatabase() *gorm.DB {
	return DATABASE
}
