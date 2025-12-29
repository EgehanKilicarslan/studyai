package tests

import (
	"log/slog"
	"os"
	"testing"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name      string
		envVars   map[string]string
		checkFunc func(*testing.T, *config.Config)
	}{
		{
			name: "success with all env vars",
			envVars: map[string]string{
				"API_SERVICE_PORT":         "9090",
				"AI_SERVICE_ADDR":          "backend-python:50051",
				"MAX_FILE_SIZE":            "10485760",
				"UPLOAD_TIMEOUT":           "300",
				"CHAT_TIMEOUT":             "30",
				"JWT_SECRET":               "test-secret",
				"ACCESS_TOKEN_EXPIRATION":  "1800",
				"REFRESH_TOKEN_EXPIRATION": "1209600",
			},
			checkFunc: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "9090", cfg.ApiServicePort)
				assert.Equal(t, "backend-python:50051", cfg.AIServiceAddr)
				assert.Equal(t, int64(10485760), cfg.MaxFileSize)
				assert.Equal(t, int64(300), cfg.UploadTimeout)
				assert.Equal(t, int64(30), cfg.ChatTimeout)
				assert.Equal(t, "test-secret", cfg.JWTSecret)
				assert.Equal(t, int64(1800), cfg.AccessTokenExpiration)
				assert.Equal(t, int64(1209600), cfg.RefreshTokenExpiration)
			},
		},
		{
			name:    "defaults when no env vars",
			envVars: map[string]string{},
			checkFunc: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "8080", cfg.ApiServicePort)
				assert.NotEmpty(t, cfg.AIServiceAddr)
				assert.Equal(t, int64(10*1024*1024), cfg.MaxFileSize)
			},
		},
		{
			name: "invalid numeric values use defaults",
			envVars: map[string]string{
				"MAX_FILE_SIZE":           "invalid",
				"UPLOAD_TIMEOUT":          "invalid",
				"CHAT_TIMEOUT":            "invalid",
				"ACCESS_TOKEN_EXPIRATION": "invalid",
			},
			checkFunc: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, int64(10*1024*1024), cfg.MaxFileSize)
				// Check defaults are applied for invalid values
				assert.NotZero(t, cfg.UploadTimeout)
				assert.NotZero(t, cfg.ChatTimeout)
			},
		},
		{
			name: "log level debug",
			envVars: map[string]string{
				"LOG_LEVEL": "debug",
			},
			checkFunc: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, slog.LevelDebug, cfg.LogLevel)
			},
		},
		{
			name: "log level info",
			envVars: map[string]string{
				"LOG_LEVEL": "info",
			},
			checkFunc: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, slog.LevelInfo, cfg.LogLevel)
			},
		},
		{
			name: "log level warn",
			envVars: map[string]string{
				"LOG_LEVEL": "warn",
			},
			checkFunc: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, slog.LevelWarn, cfg.LogLevel)
			},
		},
		{
			name: "log level error",
			envVars: map[string]string{
				"LOG_LEVEL": "error",
			},
			checkFunc: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, slog.LevelError, cfg.LogLevel)
			},
		},
		{
			name: "database config",
			envVars: map[string]string{
				"POSTGRESQL_HOST":     "localhost",
				"POSTGRESQL_PORT":     "5432",
				"POSTGRESQL_USER":     "testuser",
				"POSTGRESQL_PASSWORD": "testpass",
				"POSTGRESQL_DATABASE": "testdb",
			},
			checkFunc: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "localhost", cfg.PostgreSQLHost)
				assert.Equal(t, int64(5432), cfg.PostgreSQLPort)
				assert.Equal(t, "testuser", cfg.PostgreSQLUser)
				assert.Equal(t, "testpass", cfg.PostgreSQLPassword)
				assert.Equal(t, "testdb", cfg.PostgreSQLDatabase)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			cfg := config.LoadConfig()
			assert.NotNil(t, cfg)
			tt.checkFunc(t, cfg)
		})
	}
}
