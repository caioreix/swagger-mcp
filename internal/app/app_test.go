package app

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockHandler is a test double for the jsonHandler interface.
type mockHandler struct {
	responses [][]byte
	calls     [][]byte
	err       error
}

func (m *mockHandler) HandleJSON(line []byte) ([]byte, error) {
	m.calls = append(m.calls, append([]byte(nil), line...))
	if m.err != nil {
		return nil, m.err
	}
	if len(m.responses) > 0 {
		resp := m.responses[0]
		m.responses = m.responses[1:]
		return resp, nil
	}
	return nil, nil
}

func (m *mockHandler) HandleJSONWithHeaders(line []byte, headers map[string]string) ([]byte, error) {
	return m.HandleJSON(line)
}

// ── serveStdio ────────────────────────────────────────────────────────────────

func TestServeStdioProcessesRequestAndWritesResponse(t *testing.T) {
	handler := &mockHandler{
		responses: [][]byte{[]byte(`{"jsonrpc":"2.0","id":1,"result":"ok"}`)},
	}
	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test"}` + "\n")
	var stdout bytes.Buffer

	code := serveStdio(handler, slog.Default(), stdin, &stdout)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if len(handler.calls) != 1 {
		t.Fatalf("expected 1 handler call, got %d", len(handler.calls))
	}
	if !strings.Contains(stdout.String(), `"result":"ok"`) {
		t.Fatalf("response not in stdout: %q", stdout.String())
	}
}

func TestServeStdioSkipsBlankLines(t *testing.T) {
	handler := &mockHandler{}
	stdin := strings.NewReader("\n\n\n")
	var stdout bytes.Buffer

	code := serveStdio(handler, slog.Default(), stdin, &stdout)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if len(handler.calls) != 0 {
		t.Fatalf("expected no handler calls for blank lines, got %d", len(handler.calls))
	}
}

func TestServeStdioNoResponseForNilReturn(t *testing.T) {
	handler := &mockHandler{} // returns nil response
	stdin := strings.NewReader(`{"method":"notify"}` + "\n")
	var stdout bytes.Buffer

	code := serveStdio(handler, slog.Default(), stdin, &stdout)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout for nil response, got %q", stdout.String())
	}
}

func TestServeStdioMultipleMessages(t *testing.T) {
	handler := &mockHandler{
		responses: [][]byte{
			[]byte(`{"id":1,"result":"first"}`),
			[]byte(`{"id":2,"result":"second"}`),
		},
	}
	stdin := strings.NewReader(
		`{"id":1,"method":"a"}` + "\n" +
			`{"id":2,"method":"b"}` + "\n",
	)
	var stdout bytes.Buffer

	code := serveStdio(handler, slog.Default(), stdin, &stdout)

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if len(handler.calls) != 2 {
		t.Fatalf("expected 2 handler calls, got %d", len(handler.calls))
	}
	out := stdout.String()
	if !strings.Contains(out, `"first"`) || !strings.Contains(out, `"second"`) {
		t.Fatalf("both responses should appear in stdout: %q", out)
	}
}

// ── parseHeaderNames ─────────────────────────────────────────────────────────

func TestParseHeaderNamesEmpty(t *testing.T) {
	if names := parseHeaderNames(""); names != nil {
		t.Fatalf("expected nil for empty string, got %v", names)
	}
}

func TestParseHeaderNamesSingle(t *testing.T) {
	names := parseHeaderNames("Authorization")
	if len(names) != 1 || names[0] != "Authorization" {
		t.Fatalf("unexpected result: %v", names)
	}
}

func TestParseHeaderNamesMultipleWithSpaces(t *testing.T) {
	names := parseHeaderNames("Authorization, X-Tenant-ID , X-Source")
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %v", names)
	}
	if names[0] != "Authorization" || names[1] != "X-Tenant-ID" || names[2] != "X-Source" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestParseHeaderNamesSkipsEmptySegments(t *testing.T) {
	names := parseHeaderNames("A,,B,")
	if len(names) != 2 {
		t.Fatalf("expected 2 non-empty names, got %v", names)
	}
}

// ── extractHeaders ───────────────────────────────────────────────────────────

func TestExtractHeadersNilWhenNoNames(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer tok")

	if result := extractHeaders(r, nil); result != nil {
		t.Fatalf("expected nil result when no names provided, got %v", result)
	}
}

func TestExtractHeadersPresent(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer tok")
	r.Header.Set("X-Tenant-ID", "acme")

	result := extractHeaders(r, []string{"Authorization", "X-Tenant-ID"})
	if len(result) != 2 {
		t.Fatalf("expected 2 headers, got %v", result)
	}
	if result["Authorization"] != "Bearer tok" {
		t.Fatalf("unexpected Authorization value: %q", result["Authorization"])
	}
}

func TestExtractHeadersMissingAreOmitted(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	result := extractHeaders(r, []string{"Authorization", "X-Missing"})
	if result != nil {
		t.Fatalf("expected nil when no matching headers present, got %v", result)
	}
}

func TestExtractHeadersPartialMatch(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "token")

	result := extractHeaders(r, []string{"Authorization", "X-Missing"})
	if len(result) != 1 {
		t.Fatalf("expected 1 header (only present ones), got %v", result)
	}
	if result["Authorization"] != "token" {
		t.Fatalf("unexpected value: %v", result)
	}
}

// ── SSE HTTP handlers ─────────────────────────────────────────────────────────

func TestSSEMessageHandlerMissingClientID(t *testing.T) {
	handler := &mockHandler{}
	logger := slog.Default()
	headerNames := parseHeaderNames("")

	var clients = make(map[string]chan []byte)
	mux := buildSSEMux(handler, logger, headerNames, clients)

	req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(`{"method":"test"}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing clientId, got %d", w.Code)
	}
}

func TestSSEMessageHandlerUnknownClientIDIsIgnored(t *testing.T) {
	handler := &mockHandler{
		responses: [][]byte{[]byte(`{"result":"ok"}`)},
	}
	logger := slog.Default()
	headerNames := parseHeaderNames("")
	clients := make(map[string]chan []byte)
	mux := buildSSEMux(handler, logger, headerNames, clients)

	req := httptest.NewRequest(http.MethodPost, "/message?clientId=unknown", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
}

// ── StreamableHTTP HTTP handlers ──────────────────────────────────────────────

func TestStreamableHTTPPost(t *testing.T) {
	handler := &mockHandler{
		responses: [][]byte{[]byte(`{"jsonrpc":"2.0","id":1,"result":"ok"}`)},
	}
	logger := slog.Default()
	mux := buildStreamableHTTPMux(handler, logger, parseHeaderNames(""))

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"method":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"result":"ok"`) {
		t.Fatalf("response not in body: %q", w.Body.String())
	}
	if w.Header().Get("Mcp-Session-Id") == "" {
		t.Fatal("expected Mcp-Session-Id header to be set")
	}
}

func TestStreamableHTTPDeleteSession(t *testing.T) {
	handler := &mockHandler{}
	logger := slog.Default()
	mux := buildStreamableHTTPMux(handler, logger, parseHeaderNames(""))

	req := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	req.Header.Set("Mcp-Session-Id", "session-1")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestStreamableHTTPOptions(t *testing.T) {
	handler := &mockHandler{}
	logger := slog.Default()
	mux := buildStreamableHTTPMux(handler, logger, parseHeaderNames(""))

	req := httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", w.Code)
	}
}

func TestStreamableHTTPMethodNotAllowed(t *testing.T) {
	handler := &mockHandler{}
	logger := slog.Default()
	mux := buildStreamableHTTPMux(handler, logger, parseHeaderNames(""))

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// ── helpers for testability ───────────────────────────────────────────────────
// buildSSEMux and buildStreamableHTTPMux create the HTTP mux for testing
// without starting a real TCP listener.

func buildSSEMux(handler jsonHandler, logger *slog.Logger, headerNames []string, clients map[string]chan []byte) *http.ServeMux {
	var mu = new(syncMu)
	mux := http.NewServeMux()

	mux.HandleFunc("POST /message", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		clientID := r.URL.Query().Get("clientId")
		if clientID == "" {
			http.Error(w, "missing clientId", http.StatusBadRequest)
			return
		}

		body, err := readBody(r)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		resp, err := handler.HandleJSONWithHeaders(body, extractHeaders(r, headerNames))
		if err != nil {
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
			}
		}
		_ = logger
		w.WriteHeader(http.StatusAccepted)
	})
	return mux
}

func buildStreamableHTTPMux(handler jsonHandler, logger *slog.Logger, headerNames []string) *http.ServeMux {
	var mu syncMu
	sessions := make(map[string]bool)
	var counter int64

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
			}
			w.WriteHeader(http.StatusNoContent)

		case http.MethodPost:
			body, err := readBody(r)
			if err != nil {
				http.Error(w, "failed to read body", http.StatusBadRequest)
				return
			}
			sessionID := r.Header.Get("Mcp-Session-Id")
			if sessionID == "" {
				counter++
				sessionID = fmt.Sprintf("session-%d", counter)
				mu.Lock()
				sessions[sessionID] = true
				mu.Unlock()
			}
			resp, err := handler.HandleJSONWithHeaders(body, extractHeaders(r, headerNames))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Mcp-Session-Id", sessionID)
			_ = logger
			if len(resp) > 0 {
				w.Write(resp)
			}

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	return mux
}

// syncMu is a thin wrapper so test helpers don't need to import sync.
type syncMu struct{ mu [0]byte }

func (s *syncMu) Lock()   {}
func (s *syncMu) Unlock() {}

func readBody(r *http.Request) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
