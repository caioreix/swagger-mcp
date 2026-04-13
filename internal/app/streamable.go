package app

import (
	"log/slog"
	"net/http"

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
	if err := server.ListenAndServe(); err != nil {
		stLogger.Error("StreamableHTTP server error", "error", err)
		return 1
	}
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
