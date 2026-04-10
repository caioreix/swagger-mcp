package mcp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/caioreix/swagger-mcp/internal/config"
	mcp "github.com/caioreix/swagger-mcp/internal/mcp"
	"github.com/caioreix/swagger-mcp/internal/openapi"
	"github.com/caioreix/swagger-mcp/internal/testutil"
)

func TestProxyInputSchemaArrayHasItems(t *testing.T) {
	document := map[string]any{}

	// Swagger 2.0 array param with items
	op := map[string]any{
		"parameters": []any{
			map[string]any{
				"name":             "filter",
				"in":               "query",
				"type":             "array",
				"collectionFormat": "csv",
				"items":            map[string]any{"type": "string"},
				"description":      "Filters as field:value pairs",
			},
			// Array param WITHOUT items (malformed spec)
			map[string]any{
				"name": "tags",
				"in":   "query",
				"type": "array",
			},
		},
	}

	schema := mcp.ProxyInputSchema(document, op)
	props := schema["properties"].(map[string]any)

	// filter should have items
	filterSchema := props["filter"].(map[string]any)
	if _, hasItems := filterSchema["items"]; !hasItems {
		b, _ := json.Marshal(filterSchema)
		t.Errorf("filter param missing items: %s", b)
	}

	// tags (no items in spec) should still have items fallback
	tagsSchema := props["tags"].(map[string]any)
	if _, hasItems := tagsSchema["items"]; !hasItems {
		b, _ := json.Marshal(tagsSchema)
		t.Errorf("tags param missing items fallback: %s", b)
	}
}

func TestCachedSwaggerProxyToolsHaveValidArraySchemas(t *testing.T) { //nolint:gocognit
	repoRoot := testutil.RepoRoot(t)
	cacheFile := filepath.Join(
		repoRoot,
		"swagger-cache",
		"7818dc7eea16165d9af678f18040762460601a02ebd5792d4326b9f3a87befc9.json",
	)
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Skip("cached swagger file not found")
	}

	document, err := openapi.ReadDefinitionFromFile(cacheFile)
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile: %v", err)
	}

	tools, err := mcp.BuildProxyTools(document, "http://localhost", nil, "", config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("buildProxyTools: %v", err)
	}

	var checkSchema func(schema map[string]any, path string)
	checkSchema = func(schema map[string]any, path string) {
		typ, _ := schema["type"].(string)
		if typ == "array" {
			if _, hasItems := schema["items"]; !hasItems {
				b, _ := json.Marshal(schema)
				t.Errorf("array without items at path %s: %s", path, string(b))
			}
		}
		if props, ok := schema["properties"].(map[string]any); ok {
			for k, v := range props {
				if subSchema, subOk := v.(map[string]any); subOk {
					checkSchema(subSchema, path+"."+k)
				}
			}
		}
		if items, ok := schema["items"].(map[string]any); ok {
			checkSchema(items, path+"[items]")
		}
	}

	for _, tool := range tools {
		if tool.Definition.InputSchema == nil {
			continue
		}
		checkSchema(tool.Definition.InputSchema, tool.Definition.Name)
	}
	t.Logf("validated %d proxy tools, all array schemas have items", len(tools))
}

func TestProxyInputSchemaSwagger2BodyExpandsReferencedSchema(t *testing.T) {
	document := map[string]any{
		"definitions": map[string]any{
			"Credentials": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"email":    map[string]any{"type": "string"},
					"password": map[string]any{"type": "string"},
				},
				"required": []any{"email", "password"},
			},
		},
	}

	op := map[string]any{
		"parameters": []any{
			map[string]any{
				"in":       "body",
				"name":     "body",
				"required": true,
				"schema": map[string]any{
					"$ref": "#/definitions/Credentials",
				},
			},
		},
	}

	schema := mcp.ProxyInputSchema(document, op)
	props := schema["properties"].(map[string]any)

	if _, ok := props["email"]; !ok {
		t.Fatalf("expected email in expanded body properties, got: %#v", props)
	}
	if _, ok := props["password"]; !ok {
		t.Fatalf("expected password in expanded body properties, got: %#v", props)
	}
	if _, hasBody := props["body"]; hasBody {
		t.Fatalf("did not expect literal body field when schema properties exist")
	}

	req := schema["required"].([]string)
	if len(req) != 2 || req[0] != "email" || req[1] != "password" {
		t.Fatalf("unexpected required fields: %#v", req)
	}
}

func TestBuildRequestBodySwagger2BodyArgumentAndFallback(t *testing.T) {
	op := map[string]any{
		"parameters": []any{
			map[string]any{"in": "path", "name": "orgId"},
			map[string]any{"in": "query", "name": "expand"},
			map[string]any{"in": "header", "name": "X-Trace"},
			map[string]any{"in": "body", "name": "body"},
		},
	}

	// Explicit body argument should win.
	fromBodyArg := mcp.BuildRequestBody(op, map[string]any{
		"orgId":   "1",
		"expand":  true,
		"X-Trace": "t",
		"body": map[string]any{
			"email":    "a@b.com",
			"password": "secret",
		},
	})
	bodyMap, ok := fromBodyArg.(map[string]any)
	if !ok {
		t.Fatalf("expected map body from explicit body arg, got %T", fromBodyArg)
	}
	if bodyMap["email"] != "a@b.com" {
		t.Fatalf("unexpected explicit body content: %#v", bodyMap)
	}

	// Flattened args should be used when explicit body is not present.
	fromFallback := mcp.BuildRequestBody(op, map[string]any{
		"orgId":    "1",
		"expand":   true,
		"X-Trace":  "t",
		"email":    "a@b.com",
		"password": "secret",
	})
	fallbackMap, ok := fromFallback.(map[string]any)
	if !ok {
		t.Fatalf("expected map body from fallback args, got %T", fromFallback)
	}
	if _, exists := fallbackMap["orgId"]; exists {
		t.Fatalf("path parameter leaked into request body: %#v", fallbackMap)
	}
	if fallbackMap["email"] != "a@b.com" || fallbackMap["password"] != "secret" {
		t.Fatalf("unexpected fallback body content: %#v", fallbackMap)
	}
}

func TestBuildRequestBodyOpenAPI3ExplicitBodyArgument(t *testing.T) {
	op := map[string]any{
		"requestBody": map[string]any{
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{"type": "object"},
				},
			},
		},
	}

	fromBodyArg := mcp.BuildRequestBody(op, map[string]any{
		"body": map[string]any{
			"email":    "a@b.com",
			"password": "secret",
		},
	})
	bodyMap, ok := fromBodyArg.(map[string]any)
	if !ok {
		t.Fatalf("expected map body from explicit body arg, got %T", fromBodyArg)
	}
	if bodyMap["email"] != "a@b.com" || bodyMap["password"] != "secret" {
		t.Fatalf("unexpected explicit body content: %#v", bodyMap)
	}
}

func TestBuildRequestBodyOpenAPI3ExplicitRequestBodyJSONArgument(t *testing.T) {
	op := map[string]any{
		"requestBody": map[string]any{
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{"type": "object"},
				},
			},
		},
	}

	fromBodyArg := mcp.BuildRequestBody(op, map[string]any{
		"requestBody": `{"email":"a@b.com","password":"secret"}`,
	})
	bodyMap, ok := fromBodyArg.(map[string]any)
	if !ok {
		t.Fatalf("expected decoded map body from requestBody JSON string, got %T", fromBodyArg)
	}
	if bodyMap["email"] != "a@b.com" || bodyMap["password"] != "secret" {
		t.Fatalf("unexpected requestBody content: %#v", bodyMap)
	}
}
