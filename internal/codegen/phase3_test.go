package codegen

import (
	"strings"
	"testing"

	"github.com/caioreix/swagger-mcp/internal/openapi"
	"github.com/caioreix/swagger-mcp/internal/testutil"
)

func TestGenerateProxyHandler(t *testing.T) {
	code := GenerateProxyHandler("addPet", "POST", "/pets", "https://api.example.com", map[string]any{
		"parameters": []any{},
	}, nil)
	checks := []string{
		"func HandleAddPet(input map[string]any) (result map[string]any)",
		`baseURL := "https://api.example.com"`,
		`path := "/pets"`,
		"json.Marshal(input)",
		"client.Do(req)",
		"mcpTextResult(string(respBody))",
	}
	for _, check := range checks {
		if !strings.Contains(code, check) {
			t.Fatalf("expected proxy handler to contain %q\n%s", check, code)
		}
	}
}

func TestGenerateProxyHandlerWithPathParams(t *testing.T) {
	code := GenerateProxyHandler("getPet", "GET", "/pets/{id}", "https://api.example.com", map[string]any{
		"parameters": []any{
			map[string]any{"name": "id", "in": "path", "type": "string"},
		},
	}, nil)
	if !strings.Contains(code, `strings.ReplaceAll(path, "{id}"`) {
		t.Fatalf("expected path parameter replacement\n%s", code)
	}
}

func TestGenerateProxyHandlerWithQueryParams(t *testing.T) {
	code := GenerateProxyHandler("listPets", "GET", "/pets", "https://api.example.com", map[string]any{
		"parameters": []any{
			map[string]any{"name": "limit", "in": "query", "type": "integer"},
		},
	}, nil)
	if !strings.Contains(code, "queryValues.Set(") {
		t.Fatalf("expected query parameter handling\n%s", code)
	}
}

func TestGenerateProxyHandlerWithAuth(t *testing.T) {
	schemes := []openapi.SecurityScheme{
		{Name: "bearer", Type: "http", Scheme: "bearer"},
	}
	code := GenerateProxyHandler("addPet", "POST", "/pets", "https://api.example.com", map[string]any{}, schemes)
	if !strings.Contains(code, "applyAuth(req)") {
		t.Fatalf("expected auth application in proxy handler\n%s", code)
	}
}

func TestGenerateCompleteServer(t *testing.T) {
	document, err := openapi.ReadDefinitionFromFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	files, err := GenerateCompleteServer(document, ServerGenParams{
		ModuleName:     "github.com/test/petstore-mcp",
		TransportModes: []string{"stdio"},
		ProxyMode:      true,
	})
	if err != nil {
		t.Fatalf("GenerateCompleteServer returned error: %v", err)
	}

	expectedFiles := []string{"go.mod", "main.go", "server.go", "tools.go", "handlers.go", "helpers.go"}
	for _, f := range expectedFiles {
		if _, ok := files[f]; !ok {
			t.Fatalf("expected file %q in generated server", f)
		}
	}

	// Check go.mod
	if !strings.Contains(files["go.mod"], "github.com/test/petstore-mcp") {
		t.Fatal("go.mod should contain the module name")
	}

	// Check main.go has stdio transport
	if !strings.Contains(files["main.go"], "serveStdio(") {
		t.Fatal("main.go should contain stdio transport")
	}

	// Check server.go has JSON-RPC handling
	if !strings.Contains(files["server.go"], "handleJSON") {
		t.Fatal("server.go should contain handleJSON")
	}

	// Check tools.go has tool definitions
	if !strings.Contains(files["tools.go"], "toolDefinitions()") {
		t.Fatal("tools.go should contain toolDefinitions")
	}

	// Check handlers.go has proxy handlers
	if !strings.Contains(files["handlers.go"], "func Handle") {
		t.Fatal("handlers.go should contain handler functions")
	}

	// Check helpers.go has error helpers
	if !strings.Contains(files["helpers.go"], "mcpError") {
		t.Fatal("helpers.go should contain mcpError")
	}
}

func TestGenerateCompleteServerWithAllTransports(t *testing.T) {
	document, err := openapi.ReadDefinitionFromFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	files, err := GenerateCompleteServer(document, ServerGenParams{
		TransportModes: []string{"stdio", "sse", "streamable-http"},
		ProxyMode:      false,
	})
	if err != nil {
		t.Fatalf("GenerateCompleteServer returned error: %v", err)
	}

	if !strings.Contains(files["main.go"], "serveSSE(") {
		t.Fatal("main.go should contain SSE transport")
	}
	if !strings.Contains(files["main.go"], "serveStreamableHTTP(") {
		t.Fatal("main.go should contain StreamableHTTP transport")
	}
}

func TestGenerateCompleteServerWithEndpointFilter(t *testing.T) {
	document, err := openapi.ReadDefinitionFromFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	files, err := GenerateCompleteServer(document, ServerGenParams{
		Endpoints: []string{"/pets"},
		ProxyMode: false,
	})
	if err != nil {
		t.Fatalf("GenerateCompleteServer returned error: %v", err)
	}

	// Should only have handlers for /pets endpoints (GET and POST)
	handlerCount := strings.Count(files["handlers.go"], "func Handle")
	if handlerCount != 2 {
		t.Fatalf("expected 2 handlers for /pets, got %d", handlerCount)
	}
}

func TestExtractPathAndQueryParams(t *testing.T) {
	operation := map[string]any{
		"parameters": []any{
			map[string]any{"name": "id", "in": "path"},
			map[string]any{"name": "limit", "in": "query"},
			map[string]any{"name": "offset", "in": "query"},
			map[string]any{"name": "body", "in": "body"},
		},
	}
	pathParams := extractPathParams(operation)
	if len(pathParams) != 1 || pathParams[0] != "id" {
		t.Fatalf("expected [id] path params, got %v", pathParams)
	}
	queryParams := extractQueryParams(operation)
	if len(queryParams) != 2 {
		t.Fatalf("expected 2 query params, got %v", queryParams)
	}
}
