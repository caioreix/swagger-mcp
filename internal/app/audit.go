package app

import (
	"log/slog"
	"net/http"
	"time"
)

// auditMiddleware logs incoming HTTP requests for audit purposes.
func auditMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		remoteAddr := r.Header.Get("X-Forwarded-For")
		if remoteAddr == "" {
			remoteAddr = r.Header.Get("X-Real-IP")
		}
		if remoteAddr == "" {
			remoteAddr = r.RemoteAddr
		}

		logger.Info("audit.http_request",
			"event", "audit.http_request",
			"http_method", r.Method,
			"http_path", r.URL.Path,
			"remote_addr", remoteAddr,
			"user_agent", r.UserAgent(),
			"duration_ms", time.Since(start).Milliseconds(),
			"status_code", wrapped.statusCode,
		)
	})
}

// statusResponseWriter wraps http.ResponseWriter to capture the status code.
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
