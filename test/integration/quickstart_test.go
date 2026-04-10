package integration_test

import (
	"testing"

	"github.com/caioreix/swagger-mcp/internal/testutil"
)

func TestQuickstartHandshake(t *testing.T) {
	stdout, stderr := runBinary(t, nil, []string{
		string(testutil.JSONRPCRequest(t, 1, "initialize", map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "quickstart", "version": "1.0.0"},
		})),
		string(testutil.JSONRPCRequest(t, 2, "tools/list", map[string]any{})),
	}, map[string]string{"LOG_LEVEL": "error"})

	responses := testutil.DecodeJSONLines(t, stdout)
	if len(responses) != 2 {
		t.Fatalf("expected 2 JSON-RPC responses, got %d\nstdout:\n%s", len(responses), stdout)
	}
	if responses[0]["jsonrpc"] != "2.0" || responses[1]["jsonrpc"] != "2.0" {
		t.Fatalf("expected JSON-RPC version 2.0 in both responses: %#v", responses)
	}
	result := responses[1]["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}
	if stderr != "" {
		t.Logf("binary emitted stderr during quickstart smoke test:\n%s", stderr)
	}
}
