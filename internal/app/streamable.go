package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caioreix/swagger-mcp/internal/mcp"
	mcpgoserver "github.com/mark3labs/mcp-go/server"
)

func serveStreamableHTTP(
	mcpServer *mcpgoserver.MCPServer,
	logger *slog.Logger,
	port string,
	httpHeaders string,
) int {
	stLogger := componentLogger(logger, "app.streamable")

	headerNames := parseHeaderNames(httpHeaders)

	// Use mcp-go's streamable HTTP server.
	mcpHandler := mcpgoserver.NewStreamableHTTPServer(mcpServer)

	var handler http.Handler = mcpHandler
	if len(headerNames) > 0 {
		handler = headerInjectMiddleware(mcpHandler, headerNames)
	}
	handler = auditMiddleware(handler, stLogger)

	stLogger.Info("starting StreamableHTTP server", "port", port)
	server := &http.Server{ //nolint:gosec // timeout configured by caller
		Addr:    ":" + port,
		Handler: handler,
	}

	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-serverErr:
		stLogger.Error("server error", "error", err)
		return 1
	case sig := <-quit:
		stLogger.Info("received shutdown signal", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		stLogger.Error("graceful shutdown failed", "error", err)
		return 1
	}

	stLogger.Info("server stopped gracefully")
	return 0
}

// headerInjectMiddleware injects configured HTTP headers into the request context
// so that proxy tool handlers can forward them to upstream APIs.
func headerInjectMiddleware(next http.Handler, headerNames []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := extractHeaders(r, headerNames)
		if len(headers) > 0 {
			ctx := mcp.WithProxyHeaders(r.Context(), headers)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}
