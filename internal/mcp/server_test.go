package mcp_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/caioreix/swagger-mcp/internal/config"
	mcp "github.com/caioreix/swagger-mcp/internal/mcp"
	"github.com/caioreix/swagger-mcp/internal/openapi"
	"github.com/caioreix/swagger-mcp/internal/testutil"
)

func newTestServer(t *testing.T) *mcp.ServerAdapter {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mcpServer, err := mcp.NewServer(config.Config{WorkingDir: testutil.RepoRoot(t), LogLevel: "error"}, logger)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	return mcp.NewServerAdapter(mcpServer)
}

func newProxyTestServer(t *testing.T) *mcp.ServerAdapter {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	workingDir := t.TempDir()
	mapping := []byte("SWAGGER_FILEPATH=" + testutil.FixturePath(t, "petstore.json"))
	if err := os.WriteFile(filepath.Join(workingDir, ".swagger-mcp"), mapping, 0o600); err != nil {
		t.Fatalf("write .swagger-mcp: %v", err)
	}
	mcpServer, err := mcp.NewServer(config.Config{
		WorkingDir: workingDir,
		LogLevel:   "error",
		ProxyMode:  true,
	}, logger)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	return mcp.NewServerAdapter(mcpServer)
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
		"protocolVersion": mcpgo.LATEST_PROTOCOL_VERSION,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1.0.0"},
	}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	if result["protocolVersion"] != mcpgo.LATEST_PROTOCOL_VERSION {
		t.Fatalf("expected protocol version %s, got %#v", mcpgo.LATEST_PROTOCOL_VERSION, result["protocolVersion"])
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
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}

	// Collect tool names (mcp-go returns tools alphabetically sorted).
	toolNames := make(map[string]bool, len(tools))
	for _, rawTool := range tools {
		tool := rawTool.(map[string]any)
		toolNames[tool["name"].(string)] = true
	}

	expected := []string{
		"swagger_get_definition",
		"swagger_get_version",
		"swagger_list_endpoint_models",
		"swagger_list_endpoints",
	}
	for _, name := range expected {
		if !toolNames[name] {
			t.Fatalf("expected tool %q in tools/list response", name)
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
	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(prompts))
	}

	promptNames := make(map[string]bool, len(prompts))
	for _, rawPrompt := range prompts {
		prompt := rawPrompt.(map[string]any)
		promptNames[prompt["name"].(string)] = true
	}
	for _, name := range []string{"add-endpoint", "swagger_add_endpoint"} {
		if !promptNames[name] {
			t.Fatalf("expected prompt %q in prompts/list response", name)
		}
	}
}

func TestVersionTool(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 4, "tools/call", map[string]any{
		"name":      "swagger_get_version",
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
	structured := result["structuredContent"].(map[string]any)
	if structured["version"] != config.Version {
		t.Fatalf("expected structured version %q, got %#v", config.Version, structured)
	}
}

func TestListEndpointsTool(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 5, "tools/call", map[string]any{
		"name": "swagger_list_endpoints",
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

func TestListEndpointsToolJSONFormat(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 5, "tools/call", map[string]any{
		"name": "swagger_list_endpoints",
		"arguments": map[string]any{
			"swaggerFilePath": testutil.FixturePath(t, "petstore.json"),
			"response_format": "json",
		},
	}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)

	var parsed map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &parsed); jsonErr != nil {
		t.Fatalf("expected JSON response, got parse error: %v\ntext: %s", jsonErr, text)
	}
	if _, ok := parsed["total"]; !ok {
		t.Fatalf("expected JSON response to contain 'total' field, got: %s", text)
	}
	if _, ok := parsed["endpoints"]; !ok {
		t.Fatalf("expected JSON response to contain 'endpoints' field, got: %s", text)
	}
	if _, ok := result["structuredContent"].(map[string]any)["endpoints"]; !ok {
		t.Fatalf("expected structuredContent to contain 'endpoints', got: %#v", result["structuredContent"])
	}
}

func TestListEndpointsToolPagination(t *testing.T) {
	server := newTestServer(t)

	// First page
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 5, "tools/call", map[string]any{
		"name": "swagger_list_endpoints",
		"arguments": map[string]any{
			"swaggerFilePath": testutil.FixturePath(t, "petstore.json"),
			"response_format": "json",
			"limit":           1,
			"offset":          0,
		},
	}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)

	var parsed map[string]any
	if jsonErr := json.Unmarshal([]byte(text), &parsed); jsonErr != nil {
		t.Fatalf("expected JSON response, got: %v", jsonErr)
	}
	count, ok := parsed["count"].(float64)
	if !ok || int(count) != 1 {
		t.Fatalf("expected count=1, got %v", parsed["count"])
	}
	endpoints, ok := parsed["endpoints"].([]any)
	if !ok || len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %v", parsed["endpoints"])
	}
}

func TestListEndpointModelsToolMarkdown(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 8, "tools/call", map[string]any{
		"name": "swagger_list_endpoint_models",
		"arguments": map[string]any{
			"swaggerFilePath": testutil.FixturePath(t, "petstore.json"),
			"path":            "/pets",
			"method":          "GET",
			"response_format": "markdown",
		},
	}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "GET") || !strings.Contains(text, "/pets") {
		t.Fatalf("expected markdown to contain method and path, got: %s", text)
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

	// mcp-go returns a JSON-RPC error (INVALID_PARAMS) when a tool is not found.
	errorObject := decodeResponse(t, responseBytes)["error"].(map[string]any)
	if errorObject["code"].(float64) != -32602 {
		t.Fatalf("expected invalid params code -32602 for unknown tool, got %#v", errorObject)
	}
}

func TestAddEndpointPrompt(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 7, "prompts/get", map[string]any{
		"name": "swagger_add_endpoint",
		"arguments": map[string]string{
			"endpoint_path": "/pets/{id}",
			"http_method":   "GET",
		},
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
	responseBytes, err := server.HandleJSON(
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":123}}`),
	)
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	// mcp-go returns INVALID_REQUEST (-32600) when the params cannot be unmarshaled
	// (e.g. "name" is an integer instead of a string).
	errorObject := decodeResponse(t, responseBytes)["error"].(map[string]any)
	code := errorObject["code"].(float64)
	if code != -32600 && code != -32602 {
		t.Fatalf("expected parse/invalid error code, got %#v", errorObject)
	}
}

func TestNotificationsAreIgnored(t *testing.T) {
	server := newTestServer(t)
	responseBytes, err := server.HandleJSON(
		[]byte(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`),
	)
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}
	if responseBytes != nil {
		t.Fatalf("expected nil response for notifications, got %s", string(responseBytes))
	}
}

func TestProxyToolNameWithAPIPrefix(t *testing.T) {
	ep := openapi.Endpoint{
		Path:        "/pets/{id}",
		Method:      "GET",
		OperationID: "getPetById",
	}

	cases := []struct {
		apiName string
		want    string
	}{
		{"", "api_get_pet_by_id"},
		{"petstore", "petstore_get_pet_by_id"},
		{"my-api", "my_api_get_pet_by_id"},
	}

	for _, tc := range cases {
		got := mcp.ProxyToolName(ep, tc.apiName, "")
		if got != tc.want {
			t.Errorf("mcp.ProxyToolName(%q) = %q, want %q", tc.apiName, got, tc.want)
		}
	}
}

func TestProxyToolNameWithoutOperationID(t *testing.T) {
	ep := openapi.Endpoint{
		Path:   "/pets/{id}",
		Method: "GET",
	}

	got := mcp.ProxyToolName(ep, "store", "")
	want := "store_get_pets_id"
	if got != want {
		t.Errorf("proxyToolName = %q, want %q", got, want)
	}
}

func TestProxyToolsExposeTypedInputSchema(t *testing.T) {
	server := newProxyTestServer(t)
	responseBytes, err := server.HandleJSON(testutil.JSONRPCRequest(t, 9, "tools/list", map[string]any{}))
	if err != nil {
		t.Fatalf("HandleJSON returned error: %v", err)
	}

	result := decodeResponse(t, responseBytes)["result"].(map[string]any)
	tools := result["tools"].([]any)

	var findPetsTool map[string]any
	for _, rawTool := range tools {
		tool := rawTool.(map[string]any)
		if tool["name"] == "swagger_petstore_find_pets" {
			findPetsTool = tool
			break
		}
	}
	if findPetsTool == nil {
		t.Fatalf("expected swagger_petstore_find_pets tool in proxy tools list")
	}

	inputSchema := findPetsTool["inputSchema"].(map[string]any)
	properties := inputSchema["properties"].(map[string]any)
	if properties["limit"].(map[string]any)["type"] != "integer" {
		t.Fatalf("expected limit param type integer, got %#v", properties["limit"])
	}
	tagsSchema := properties["tags"].(map[string]any)
	if tagsSchema["type"] != "array" {
		t.Fatalf("expected tags param type array, got %#v", tagsSchema)
	}
	if items, ok := tagsSchema["items"].(map[string]any); !ok || items["type"] != "string" {
		t.Fatalf("expected tags array items type string, got %#v", tagsSchema["items"])
	}
}
