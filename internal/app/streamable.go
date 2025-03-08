package app

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
)

func serveStreamableHTTP(handler jsonHandler, logger *slog.Logger, port string, httpHeaders string) int {
	stLogger := componentLogger(logger, "app.streamable")

	headerNames := parseHeaderNames(httpHeaders)

	var mu sync.Mutex
	sessions := make(map[string]bool)
	var sessionCounter atomic.Int64

	mux := http.NewServeMux()

	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Mcp-Session-Id")
		w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")

		switch r.Method {
		case http.MethodOptions:
			w.Header().Set("Access-Control-Allow-Methods", "POST, DELETE, OPTIONS")
			w.WriteHeader(http.StatusNoContent)

		case http.MethodDelete:
			sessionID := r.Header.Get("Mcp-Session-Id")
			if sessionID != "" {
				mu.Lock()
				delete(sessions, sessionID)
				mu.Unlock()
				stLogger.Info("session terminated", "sessionId", sessionID)
			}
			w.WriteHeader(http.StatusNoContent)

		case http.MethodPost:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read body", http.StatusBadRequest)
				return
			}

			sessionID := r.Header.Get("Mcp-Session-Id")
			if sessionID == "" {
				sessionID = fmt.Sprintf("session-%d", sessionCounter.Add(1))
				mu.Lock()
				sessions[sessionID] = true
				mu.Unlock()
				stLogger.Info("new session created", "sessionId", sessionID)
			} else {
				mu.Lock()
				if !sessions[sessionID] {
					sessions[sessionID] = true
				}
				mu.Unlock()
			}

			stLogger.Debug("handling StreamableHTTP request", "sessionId", sessionID, "size", len(body))

			resp, err := handler.HandleJSONWithHeaders(body, extractHeaders(r, headerNames))
			if err != nil {
				stLogger.Warn("handler error", "sessionId", sessionID, "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", sessionID)
			if len(resp) > 0 {
				w.Write(resp)
			}

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	stLogger.Info("starting StreamableHTTP server", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		stLogger.Error("StreamableHTTP server error", "error", err)
		return 1
	}
	return 0
}
