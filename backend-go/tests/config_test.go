package tests

import (
	"os"
	"testing"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig_Success(t *testing.T) {
	// Set environment variables
	os.Setenv("API_SERVICE_PORT", "9090")
	os.Setenv("AI_SERVICE_ADDR", "backend-python:50051")
	os.Setenv("MAX_FILE_SIZE", "10485760")
	os.Setenv("UPLOAD_TIMEOUT", "300")
	os.Setenv("CHAT_TIMEOUT", "30")
	defer os.Clearenv()

	cfg := config.LoadConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, "9090", cfg.ApiServicePort)
	assert.Equal(t, "backend-python:50051", cfg.AIServiceAddr)
	assert.Equal(t, int64(10485760), cfg.MaxFileSize)
	assert.Equal(t, int64(300), cfg.UploadTimeout)
	assert.Equal(t, int64(30), cfg.ChatTimeout)
}

func TestLoadConfig_Defaults(t *testing.T) {
	os.Clearenv()

	cfg := config.LoadConfig()

	assert.NotNil(t, cfg)
	// Test that defaults are applied
	assert.NotEmpty(t, cfg.ApiServicePort)
	assert.NotEmpty(t, cfg.AIServiceAddr)
	assert.Equal(t, "8080", cfg.ApiServicePort)
}

func TestLoadConfig_InvalidValues(t *testing.T) {
	os.Setenv("MAX_FILE_SIZE", "invalid")
	defer os.Clearenv()

	cfg := config.LoadConfig()

	// Should use default when invalid
	assert.NotNil(t, cfg)
	assert.Equal(t, int64(10*1024*1024), cfg.MaxFileSize)
}

func TestLoadConfig_LogLevel(t *testing.T) {
	os.Setenv("LOG_LEVEL", "debug")
	defer os.Clearenv()

	cfg := config.LoadConfig()

	assert.NotNil(t, cfg)
	assert.NotNil(t, cfg.LogLevel)
}
