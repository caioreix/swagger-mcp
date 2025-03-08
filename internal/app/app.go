package app

import (
	"io"
	"log/slog"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/logging"
	"github.com/caioreix/swagger-mcp/internal/mcp"
)

type jsonHandler interface {
	HandleJSON(line []byte) ([]byte, error)
	HandleJSONWithHeaders(line []byte, headers map[string]string) ([]byte, error)
}

func Run(cfg config.Config, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	baseLogger := logging.Setup(cfg.LogLevel)
	logger := logging.WithComponent(baseLogger, "app")
	logger.Info("starting swagger mcp server", "working_dir", cfg.WorkingDir, "swagger_url_configured", cfg.SwaggerURL != "")

	server, err := mcp.NewServer(cfg, baseLogger)
	if err != nil {
		logger.Error("failed to initialize server", "error", err)
		return 1
	}

	logger.Info("waiting for MCP client input", "transport", cfg.Transport, "protocol", "JSON-RPC 2.0", "ui_enabled", cfg.EnableUI)

	if cfg.EnableUI && cfg.Transport == "stdio" {
		// In stdio mode with UI enabled, start UI on the configured port
		startWebUIBackground(server, logger, cfg.Port)
	}

	switch cfg.Transport {
	case "sse":
		return serveSSE(server, logger, cfg.Port, cfg.SseHeaders)
	case "streamable-http":
		return serveStreamableHTTP(server, logger, cfg.Port, cfg.HttpHeaders)
	default:
		return serveStdio(server, logger, stdin, stdout)
	}
}

func componentLogger(logger *slog.Logger, component string) *slog.Logger {
	return logging.WithComponent(logger, component)
}
