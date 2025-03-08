package integration_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/caioreix/swagger-mcp/internal/testutil"
)

func TestBinaryStartupInitializeAndToolsList(t *testing.T) {
	stdout, _ := runBinary(t, nil, []string{
		string(testutil.JSONRPCRequest(t, 1, "initialize", map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "binary-test", "version": "1.0.0"},
		})),
		string(testutil.JSONRPCRequest(t, 2, "tools/list", map[string]any{})),
	}, map[string]string{"LOG_LEVEL": "error"})

	responses := testutil.DecodeJSONLines(t, stdout)
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d\nstdout:\n%s", len(responses), stdout)
	}
	result := responses[1]["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 7 {
		t.Fatalf("expected 7 tools, got %d", len(tools))
	}
}

func TestBinaryStartupExplainsStdioWait(t *testing.T) {
	stdout, stderr := runBinary(t, nil, nil, map[string]string{"LOG_LEVEL": "info"})
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout without MCP input, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "waiting for MCP client input") {
		t.Fatalf("expected stderr to explain stdio wait, got:\n%s", stderr)
	}
}

func TestBinaryStartupPreloadsCLIURL(t *testing.T) {
	payload, err := os.ReadFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	stdout, stderr := runBinary(t, []string{"--swagger-url=" + server.URL}, nil, map[string]string{"LOG_LEVEL": "error"})
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("expected no stdout without MCP input, got:\n%s", stdout)
	}
	if requests != 1 {
		t.Fatalf("expected startup preload to fetch swagger exactly once, got %d", requests)
	}
	if strings.Contains(stderr, "failed to initialize server") {
		t.Fatalf("expected successful startup preload, got stderr:\n%s", stderr)
	}
}
