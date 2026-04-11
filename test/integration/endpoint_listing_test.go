package integration_test

import (
	"strings"
	"testing"

	"github.com/caioreix/swagger-mcp/internal/testutil"
)

func TestBinaryUsesCLIURLForEndpointListing(t *testing.T) {
	server := fixtureServer(t, "petstore.json", "application/json")
	stdout, stderr := runBinary(t, []string{"--swagger-url=" + server.URL}, []string{
		string(testutil.JSONRPCRequest(t, 1, "initialize", map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "endpoint-test", "version": "1.0.0"},
		})),
		string(testutil.JSONRPCRequest(t, 2, "tools/call", map[string]any{
			"name":      "swagger_list_endpoints",
			"arguments": map[string]any{},
		})),
	}, map[string]string{"LOG_LEVEL": "info"})

	responses := testutil.DecodeJSONLines(t, stdout)
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d\nstdout:\n%s", len(responses), stdout)
	}
	result := responses[1]["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "/pets") {
		t.Fatalf("expected listEndpoints output to include /pets, got %q", text)
	}
	if strings.Contains(stderr, `"jsonrpc":"2.0"`) {
		t.Fatalf("stderr must not contain MCP payloads, got:\n%s", stderr)
	}
}
