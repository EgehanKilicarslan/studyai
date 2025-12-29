package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/EgehanKilicarslan/studyai/backend-go/internal/config"
)

func New(cfg *config.Config) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}

	if strings.ToLower(cfg.AppEnv) == "production" {
		// JSON format
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		// Human-readable format
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)

	slog.SetDefault(logger)

	return logger
}
