package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv                 string
	LogLevel               slog.Level
	ApiServicePort         string
	ApiGrpcPort            string
	AIServiceAddr          string
	MaxFileSize            int64
	ChatTimeout            int64
	UploadTimeout          int64
	PostgreSQLHost         string
	PostgreSQLPort         int64
	PostgreSQLUser         string
	PostgreSQLPassword     string
	PostgreSQLDatabase     string
	JWTSecret              string
	AccessTokenExpiration  int64
	RefreshTokenExpiration int64
	RedisHost              string
	RedisPort              int64
	RedisPassword          string
	RedisDatabase          int64
	ChatHistoryTTL         int64 // Chat history TTL in seconds
}

func LoadConfig() *Config {
	return &Config{
		AppEnv:                 getEnv("APP_ENV", "development"),                  // Default development
		LogLevel:               getLogLevel(),                                     // Default INFO
		ApiServicePort:         getEnv("API_SERVICE_PORT", "8080"),                // Default 8080
		ApiGrpcPort:            getEnv("API_GRPC_PORT", "50052"),                  // Default 50052 (Go gRPC server)
		AIServiceAddr:          getAIServiceAddr(),                                // Default backend-python:50051
		MaxFileSize:            getEnvAsInt64("MAX_FILE_SIZE", 10*1024*1024),      // Default 10 MB
		ChatTimeout:            getEnvAsInt64("CHAT_TIMEOUT", 120),                // Default 120 seconds
		UploadTimeout:          getEnvAsInt64("UPLOAD_TIMEOUT", 300),              // Default 300 seconds
		PostgreSQLHost:         getEnv("POSTGRESQL_HOST", "db"),                   // Default db
		PostgreSQLPort:         getEnvAsInt64("POSTGRESQL_PORT", 5432),            // Default 5432
		PostgreSQLUser:         getEnv("POSTGRESQL_USER", "studyai_user"),         // Default user
		PostgreSQLPassword:     getEnv("POSTGRESQL_PASSWORD", "studyai_password"), // Default password
		PostgreSQLDatabase:     getEnv("POSTGRESQL_DATABASE", "studyai_db"),       // Default database name
		JWTSecret:              getEnv("JWT_SECRET", "studyai_secret"),            // Default secret key
		AccessTokenExpiration:  getEnvAsInt64("ACCESS_TOKEN_EXPIRATION", 900),     // Default 15 minutes
		RefreshTokenExpiration: getEnvAsInt64("REFRESH_TOKEN_EXPIRATION", 604800), // Default 7 days
		RedisHost:              getEnv("REDIS_HOST", "redis"),                     // Default redis
		RedisPort:              getEnvAsInt64("REDIS_PORT", 6379),                 // Default 6379
		RedisPassword:          getEnv("REDIS_PASSWORD", ""),                      // Default empty
		RedisDatabase:          getEnvAsInt64("REDIS_DATABASE", 0),                // Default 0
		ChatHistoryTTL:         getEnvAsInt64("CHAT_HISTORY_TTL", 3600),           // Default 1 hour (3600 seconds)
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt64(key string, fallback int64) int64 {
	if valueStr, exists := os.LookupEnv(key); exists {
		if value, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
			return value
		}
	}
	return fallback
}

func getLogLevel() slog.Level {
	levelStr := getEnv("LOG_LEVEL", "INFO")

	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getAIServiceAddr() string {
	host := getEnv("AI_SERVICE_HOST", "backend-python")
	port := getEnv("AI_SERVICE_PORT", "50051")
	return fmt.Sprintf("%s:%s", host, port)
}
