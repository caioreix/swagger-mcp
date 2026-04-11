package app

import (
	"io"
	"log/slog"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/logging"
	"github.com/caioreix/swagger-mcp/internal/mcp"
)

// jsonHandler is implemented by mcp.ServerAdapter and the test mockHandler.
// It is used by the stdio, SSE, and web UI transports.
type jsonHandler interface {
	HandleJSON(line []byte) ([]byte, error)
	HandleJSONWithHeaders(line []byte, headers map[string]string) ([]byte, error)
}

func Run(cfg config.Config, stdin io.Reader, stdout io.Writer, _ io.Writer) int {
	baseLogger := logging.Setup(cfg.LogLevel)
	logger := logging.WithComponent(baseLogger, "app")
	logger.Info(
		"starting swagger mcp server",
		"working_dir",
		cfg.WorkingDir,
		"swagger_url_configured",
		cfg.SwaggerURL != "",
	)

	mcpServer, err := mcp.NewServer(cfg, baseLogger)
	if err != nil {
		logger.Error("failed to initialize server", "error", err)
		return 1
	}

	// Adapter exposes HandleJSON / HandleJSONWithHeaders for stdio, SSE, and web UI.
	adapter := mcp.NewServerAdapter(mcpServer)

	logger.Info(
		"waiting for MCP client input",
		"transport",
		cfg.Transport,
		"protocol",
		"JSON-RPC 2.0",
		"ui_enabled",
		cfg.EnableUI,
	)

	if cfg.EnableUI && cfg.Transport == "stdio" {
		startWebUIBackground(adapter, logger, cfg.Port)
	}

	switch cfg.Transport {
	case "sse":
		return serveSSE(adapter, logger, cfg.Port, cfg.SseHeaders)
	case "streamable-http":
		return serveStreamableHTTP(mcpServer, logger, cfg.Port, cfg.HTTPHeaders)
	default:
		return serveStdio(adapter, logger, stdin, stdout)
	}
}

func componentLogger(logger *slog.Logger, component string) *slog.Logger {
	return logging.WithComponent(logger, component)
}
