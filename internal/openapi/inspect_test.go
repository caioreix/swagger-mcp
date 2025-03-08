package openapi

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/caioreix/swagger-mcp/internal/testutil"
)

func TestListEndpointsGoldenPetstore(t *testing.T) {
	document, err := ReadDefinitionFromFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	endpoints, err := ListEndpoints(document)
	if err != nil {
		t.Fatalf("ListEndpoints returned error: %v", err)
	}
	formatted, err := json.MarshalIndent(endpoints, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent returned error: %v", err)
	}
	expected := testutil.ReadGolden(t, "petstore-endpoints.golden.json")
	if string(formatted)+"\n" != expected {
		t.Fatalf("unexpected golden output\nexpected:\n%s\nactual:\n%s", expected, string(formatted)+"\n")
	}
}

func TestListEndpointModelsPetstore(t *testing.T) {
	document, err := ReadDefinitionFromFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	models, err := ListEndpointModels(document, "/pets", "POST")
	if err != nil {
		t.Fatalf("ListEndpointModels returned error: %v", err)
	}
	if len(models) == 0 {
		t.Fatalf("expected endpoint models for POST /pets")
	}

	names := make([]string, 0, len(models))
	for _, model := range models {
		names = append(names, model.Name)
	}
	sort.Strings(names)
	found := map[string]bool{}
	for _, name := range names {
		found[name] = true
	}
	if !found["NewPet"] || !found["Pet"] {
		t.Fatalf("expected NewPet and Pet models, got %+v", names)
	}
}

func TestListEndpointsOpenAPI31(t *testing.T) {
	document, err := ReadDefinitionFromFile(testutil.FixturePath(t, "openapi-3.1.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	endpoints, err := ListEndpoints(document)
	if err != nil {
		t.Fatalf("ListEndpoints returned error: %v", err)
	}
	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}
	if endpoints[0].OperationID == "" || endpoints[1].OperationID == "" {
		t.Fatalf("expected operation IDs to be populated: %+v", endpoints)
	}
}

func TestListEndpointModelsOpenAPI31(t *testing.T) {
	document, err := ReadDefinitionFromFile(testutil.FixturePath(t, "openapi-3.1.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	models, err := ListEndpointModels(document, "/widgets", "POST")
	if err != nil {
		t.Fatalf("ListEndpointModels returned error: %v", err)
	}
	names := make([]string, 0, len(models))
	for _, model := range models {
		names = append(names, model.Name)
	}
	sort.Strings(names)
	if len(names) != 2 || names[0] != "Widget" || names[1] != "WidgetCreate" {
		t.Fatalf("expected Widget and WidgetCreate models, got %+v", names)
	}
}

func TestExtractBaseURLSwagger2(t *testing.T) {
	document, err := ReadDefinitionFromFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}
	url := ExtractBaseURL(document)
	if url != "http://petstore.swagger.io/api" {
		t.Fatalf("expected http://petstore.swagger.io/api, got %q", url)
	}
}

func TestExtractBaseURLOpenAPI3(t *testing.T) {
	document := map[string]any{
		"openapi": "3.0.0",
		"servers": []any{
			map[string]any{"url": "https://api.example.com/v1"},
		},
	}
	url := ExtractBaseURL(document)
	if url != "https://api.example.com/v1" {
		t.Fatalf("expected https://api.example.com/v1, got %q", url)
	}
}

func TestExtractBaseURLOpenAPI3WithVariables(t *testing.T) {
	document := map[string]any{
		"openapi": "3.0.0",
		"servers": []any{
			map[string]any{
				"url": "https://{host}/api/{version}",
				"variables": map[string]any{
					"host":    map[string]any{"default": "api.example.com"},
					"version": map[string]any{"default": "v2"},
				},
			},
		},
	}
	url := ExtractBaseURL(document)
	if url != "https://api.example.com/api/v2" {
		t.Fatalf("expected https://api.example.com/api/v2, got %q", url)
	}
}

func TestExtractBaseURLNoHost(t *testing.T) {
	document := map[string]any{"swagger": "2.0"}
	url := ExtractBaseURL(document)
	if url != "" {
		t.Fatalf("expected empty string, got %q", url)
	}
}

func TestExtractSecuritySchemesSwagger2(t *testing.T) {
	document := map[string]any{
		"swagger": "2.0",
		"securityDefinitions": map[string]any{
			"api_key": map[string]any{
				"type": "apiKey",
				"name": "api_key",
				"in":   "header",
			},
			"petstore_auth": map[string]any{
				"type":             "oauth2",
				"flow":             "implicit",
				"authorizationUrl": "https://petstore.swagger.io/oauth/authorize",
				"scopes": map[string]any{
					"read:pets":  "read your pets",
					"write:pets": "modify pets",
				},
			},
		},
	}
	schemes := ExtractSecuritySchemes(document)
	if len(schemes) != 2 {
		t.Fatalf("expected 2 schemes, got %d", len(schemes))
	}
	// Sorted by name: api_key, petstore_auth
	if schemes[0].Name != "api_key" || schemes[0].Type != "apiKey" || schemes[0].ParamName != "api_key" {
		t.Fatalf("unexpected api_key scheme: %+v", schemes[0])
	}
	if schemes[1].Name != "petstore_auth" || schemes[1].Type != "oauth2" || schemes[1].FlowType != "implicit" {
		t.Fatalf("unexpected petstore_auth scheme: %+v", schemes[1])
	}
}

func TestExtractSecuritySchemesOpenAPI3(t *testing.T) {
	document := map[string]any{
		"openapi": "3.0.0",
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
				"basicAuth": map[string]any{
					"type":   "http",
					"scheme": "basic",
				},
			},
		},
	}
	schemes := ExtractSecuritySchemes(document)
	if len(schemes) != 2 {
		t.Fatalf("expected 2 schemes, got %d", len(schemes))
	}
	if schemes[0].Name != "basicAuth" || schemes[0].Scheme != "basic" {
		t.Fatalf("unexpected basicAuth scheme: %+v", schemes[0])
	}
	if schemes[1].Name != "bearerAuth" || schemes[1].Scheme != "bearer" || schemes[1].BearerFmt != "JWT" {
		t.Fatalf("unexpected bearerAuth scheme: %+v", schemes[1])
	}
}

func TestExtractEndpointSecurityOperationLevel(t *testing.T) {
	document := map[string]any{
		"openapi": "3.0.0",
		"security": []any{
			map[string]any{"globalAuth": []any{}},
		},
		"paths": map[string]any{
			"/pets": map[string]any{
				"get": map[string]any{
					"security": []any{
						map[string]any{"petAuth": []any{"read:pets"}},
					},
				},
			},
		},
	}
	reqs, err := ExtractEndpointSecurity(document, "/pets", "GET")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 1 || reqs[0].SchemeName != "petAuth" {
		t.Fatalf("expected petAuth requirement, got %+v", reqs)
	}
	if len(reqs[0].Scopes) != 1 || reqs[0].Scopes[0] != "read:pets" {
		t.Fatalf("expected read:pets scope, got %+v", reqs[0].Scopes)
	}
}

func TestExtractEndpointSecurityFallbackGlobal(t *testing.T) {
	document := map[string]any{
		"openapi": "3.0.0",
		"security": []any{
			map[string]any{"globalAuth": []any{}},
		},
		"paths": map[string]any{
			"/pets": map[string]any{
				"get": map[string]any{
					"summary": "List pets",
				},
			},
		},
	}
	reqs, err := ExtractEndpointSecurity(document, "/pets", "GET")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reqs) != 1 || reqs[0].SchemeName != "globalAuth" {
		t.Fatalf("expected globalAuth requirement, got %+v", reqs)
	}
}

func TestExtractSecuritySchemesEmpty(t *testing.T) {
	document := map[string]any{"swagger": "2.0"}
	schemes := ExtractSecuritySchemes(document)
	if len(schemes) != 0 {
		t.Fatalf("expected 0 schemes, got %d", len(schemes))
	}
}
