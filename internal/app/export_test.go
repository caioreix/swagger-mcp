package app

import (
	"io"
	"log/slog"
	"net/http"
)

// JSONHandler is an exported alias for the jsonHandler interface, for testing.
type JSONHandler = jsonHandler

// ServeStdio is a test export of serveStdio.
func ServeStdio(handler jsonHandler, logger *slog.Logger, stdin io.Reader, stdout io.Writer) int {
	return serveStdio(handler, logger, stdin, stdout)
}

// ParseHeaderNames is a test export of parseHeaderNames.
func ParseHeaderNames(s string) []string {
	return parseHeaderNames(s)
}

// ExtractHeaders is a test export of extractHeaders.
func ExtractHeaders(r *http.Request, names []string) map[string]string {
	return extractHeaders(r, names)
}
