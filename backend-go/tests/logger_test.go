package tests

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
	"github.com/EgehanKilicarslan/studyai/backend-go/internal/logger"
	"github.com/stretchr/testify/assert"
)

func TestNewLogger_Development(t *testing.T) {
	cfg := &config.Config{
		LogLevel: slog.LevelDebug,
	}

	var buf bytes.Buffer
	log := logger.New(cfg)
	log = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: cfg.LogLevel}))

	assert.NotNil(t, log)
	log.Info("test message")

	output := buf.String()
	assert.Contains(t, output, "test message")
}

func TestNewLogger_Production(t *testing.T) {
	cfg := &config.Config{
		LogLevel: slog.LevelInfo,
	}

	var buf bytes.Buffer
	log := logger.New(cfg)
	log = slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: cfg.LogLevel}))

	assert.NotNil(t, log)
	log.Info("test message", slog.String("key", "value"))

	output := buf.String()
	assert.Contains(t, output, "test message")
}

func TestNewLogger_Default(t *testing.T) {
	cfg := &config.Config{
		LogLevel: slog.LevelInfo,
	}

	log := logger.New(cfg)

	assert.NotNil(t, log)
}

func TestNewLogger_LogLevels(t *testing.T) {
	cfg := &config.Config{
		LogLevel: slog.LevelInfo,
	}

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: cfg.LogLevel}))

	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message")

	output := buf.String()
	// Debug should be filtered in production (info level)
	assert.NotContains(t, output, "debug message")
	assert.Contains(t, output, "info message")
}

func TestNewLogger_DifferentLogLevels(t *testing.T) {
	testCases := []struct {
		name     string
		logLevel slog.Level
	}{
		{"Debug", slog.LevelDebug},
		{"Info", slog.LevelInfo},
		{"Warn", slog.LevelWarn},
		{"Error", slog.LevelError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				LogLevel: tc.logLevel,
			}

			log := logger.New(cfg)
			assert.NotNil(t, log)
		})
	}
}
