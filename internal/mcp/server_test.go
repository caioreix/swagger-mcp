package mcp

import (
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/testutil"
)

func newTestServer(t *testing.T) *server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := NewServer(config.Config{WorkingDir: testutil.RepoRoot(t), LogLevel: "error"}, logger)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	return server
}

func decodeResponse(t *testing.T, payload []byte) map[string]any {
	t.Helper()
	var response map[string]any
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	return response
}

func TestInitialize(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 1, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1.0.0"},
	}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	if result["protocolVersion"] != latestProtocolVersion {
		t.Fatalf("expected protocol version %s, got %#v", latestProtocolVersion, result["protocolVersion"])
	}
}

func TestPing(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 1, "ping", map[string]any{}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}
	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	if len(result) != 0 {
		t.Fatalf("expected empty ping result, got %#v", result)
	}
}

func TestToolsList(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 2, "tools/list", map[string]any{}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 7 {
		t.Fatalf("expected 7 tools, got %d", len(tools))
	}

	expected := []string{"getSwaggerDefinition", "listEndpoints", "listEndpointModels", "generateModelCode", "generateEndpointToolCode", "generateServer", "version"}
	for index, toolName := range expected {
		tool := tools[index].(map[string]any)
		if tool["name"] != toolName {
			t.Fatalf("expected tool %d to be %q, got %#v", index, toolName, tool["name"])
		}
	}
}

func TestPromptsList(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 3, "prompts/list", map[string]any{}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	prompts := result["prompts"].([]any)
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if prompts[0].(map[string]any)["name"] != "add-endpoint" {
		t.Fatalf("expected add-endpoint prompt, got %#v", prompts[0])
	}
}

func TestVersionTool(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 4, "tools/call", map[string]any{
		"name":      "version",
		"arguments": map[string]any{},
	}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, config.Version) {
		t.Fatalf("expected version tool response to include %q, got %q", config.Version, text)
	}
}

func TestListEndpointsTool(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 5, "tools/call", map[string]any{
		"name": "listEndpoints",
		"arguments": map[string]any{
			"swaggerFilePath": testutil.FixturePath(t, "petstore.json"),
		},
	}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "/pets") {
		t.Fatalf("expected listEndpoints text to contain /pets, got %q", text)
	}
}

func TestUnknownToolReturnsErrorContent(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 6, "tools/call", map[string]any{
		"name":      "unknownTool",
		"arguments": map[string]any{},
	}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected result to be marked as error: %#v", result)
	}
}

func TestAddEndpointPrompt(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 7, "prompts/get", map[string]any{
		"name":      "add-endpoint",
		"arguments": map[string]string{"endpointPath": "/pets/{id}", "httpMethod": "GET"},
	}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	messages := result["messages"].([]any)
	if len(messages) == 0 {
		t.Fatalf("expected prompt messages")
	}
}

func TestInvalidJSONPayload(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON([]byte(`{"jsonrpc":`))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	errorObject := decodeResponse(t, responseBytes)["error"].(map[string]any)
	if errorObject["code"].(float64) != -32700 {
		t.Fatalf("expected parse error code -32700, got %#v", errorObject)
	}
}

func TestInvalidJSONRPCVersion(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON([]byte(`{"jsonrpc":"1.0","id":1,"method":"ping"}`))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	errorObject := decodeResponse(t, responseBytes)["error"].(map[string]any)
	if errorObject["code"].(float64) != -32600 {
		t.Fatalf("expected invalid request code -32600, got %#v", errorObject)
	}
}

func TestUnknownMethod(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON([]byte(`{"jsonrpc":"2.0","id":1,"method":"unknown/method"}`))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	errorObject := decodeResponse(t, responseBytes)["error"].(map[string]any)
	if errorObject["code"].(float64) != -32601 {
		t.Fatalf("expected method not found code -32601, got %#v", errorObject)
	}
}

func TestInvalidToolParams(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":123}}`))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	errorObject := decodeResponse(t, responseBytes)["error"].(map[string]any)
	if errorObject["code"].(float64) != -32602 {
		t.Fatalf("expected invalid params code -32602, got %#v", errorObject)
	}
}

func TestNotificationsAreIgnored(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}
	if responseBytes != nil {
		t.Fatalf("expected nil response for notifications, got %s", string(responseBytes))
	}
}
