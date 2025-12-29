package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv         string
	LogLevel       slog.Level
	ApiServicePort string
	AIServiceAddr  string
	MaxFileSize    int64
	ChatTimeout    int64
	UploadTimeout  int64
}

func LoadConfig() *Config {
	return &Config{
		AppEnv:         getEnv("APP_ENV", "development"), // Default development
		LogLevel:       getLogLevel(),
		ApiServicePort: getEnv("API_SERVICE_PORT", "8080"),           // Default port 8080
		AIServiceAddr:  getAIServiceAddr(),                           // Default backend-python:50051
		MaxFileSize:    getEnvAsInt64("MAX_FILE_SIZE", 10*1024*1024), // Default 10 MB
		ChatTimeout:    getEnvAsInt64("CHAT_TIMEOUT", 120),           // Default 120 seconds
		UploadTimeout:  getEnvAsInt64("UPLOAD_TIMEOUT", 300),         // Default 300 seconds
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
