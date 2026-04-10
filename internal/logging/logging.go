package logging

import (
	"log/slog"
	"os"
	"strings"
)

func Setup(levelName string) *slog.Logger {
	level := new(slog.LevelVar)
	level.Set(parseLevel(levelName))

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})).
		With("service", "swagger-mcp")
	slog.SetDefault(logger)
	return logger
}

func WithComponent(logger *slog.Logger, component string) *slog.Logger {
	if logger == nil {
		logger = slog.Default()
	}
	component = strings.TrimSpace(component)
	if component == "" {
		return logger
	}
	return logger.With("component", component)
}

func parseLevel(levelName string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(levelName)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
