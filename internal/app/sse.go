package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const sseClientBufferSize = 64

func serveSSE(handler jsonHandler, logger *slog.Logger, port string, sseHeaders string) int { //nolint:gocognit,funlen
	sseLogger := componentLogger(logger, "app.sse")

	headerNames := parseHeaderNames(sseHeaders)

	var mu sync.Mutex
	clients := make(map[string]chan []byte)
	var clientCounter atomic.Int64

	mux := http.NewServeMux()

	mux.HandleFunc("GET /sse", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		clientID := fmt.Sprintf("client-%d", clientCounter.Add(1))
		ch := make(chan []byte, sseClientBufferSize)
		mu.Lock()
		clients[clientID] = ch
		mu.Unlock()
		defer func() {
			mu.Lock()
			delete(clients, clientID)
			close(ch)
			mu.Unlock()
		}()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		sseLogger.Info("SSE client connected", "clientId", clientID)

		// Send endpoint event with the message URL
		fmt.Fprintf(w, "event: endpoint\ndata: /message?clientId=%s\n\n", clientID)
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case msg, msgOk := <-ch:
				if !msgOk {
					return
				}
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
				flusher.Flush()
			case <-ctx.Done():
				sseLogger.Info("SSE client disconnected", "clientId", clientID)
				return
			}
		}
	})

	mux.HandleFunc("POST /message", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		clientID := r.URL.Query().Get("clientId")
		if clientID == "" {
			http.Error(w, "missing clientId", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		sseLogger.Debug("handling SSE message", "clientId", clientID, "size", len(body))

		resp, err := handler.HandleJSONWithHeaders(body, extractHeaders(r, headerNames))
		if err != nil {
			sseLogger.Warn("handler error", "clientId", clientID, "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		mu.Lock()
		ch, ok := clients[clientID]
		mu.Unlock()

		if ok && len(resp) > 0 {
			select {
			case ch <- resp:
			default:
				sseLogger.Warn("SSE client buffer full, dropping message", "clientId", clientID)
			}
		}

		w.WriteHeader(http.StatusAccepted)
	})

	// CORS preflight
	mux.HandleFunc("OPTIONS /message", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(http.StatusNoContent)
	})

	sseLogger.Info("starting SSE server", "port", port)
	sseLogger.Warn("SSE transport is deprecated; migrate to streamable-http instead")
	server := &http.Server{ //nolint:gosec // timeout configured by caller
		Addr:    ":" + port,
		Handler: auditMiddleware(mux, sseLogger),
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
		sseLogger.Error("server error", "error", err)
		return 1
	case sig := <-quit:
		sseLogger.Info("received shutdown signal", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		sseLogger.Error("graceful shutdown failed", "error", err)
		return 1
	}

	sseLogger.Info("server stopped gracefully")
	return 0
}

// parseHeaderNames splits a comma-separated list of header names into a slice,
// trimming whitespace and skipping empty entries.
func parseHeaderNames(s string) []string {
	if s == "" {
		return nil
	}
	var names []string
	for name := range strings.SplitSeq(s, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// extractHeaders reads the specified header names from an HTTP request and
// returns them as a map. Headers not present in the request are omitted.
func extractHeaders(r *http.Request, names []string) map[string]string {
	if len(names) == 0 {
		return nil
	}
	headers := make(map[string]string, len(names))
	for _, name := range names {
		if v := r.Header.Get(name); v != "" {
			headers[name] = v
		}
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}
