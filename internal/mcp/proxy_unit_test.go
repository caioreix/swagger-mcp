package mcp

import (
	"strings"
	"testing"

	"github.com/caioreix/swagger-mcp/internal/openapi"
)

func TestToSnakeCase(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"simple lowercase", "hello", "hello"},
		{"camel case", "getPetById", "get_pet_by_id"},
		{"all caps acronym", "HTTPMethod", "http_method"},
		{"mixed acronym", "getHTTPStatus", "get_http_status"},
		{"already snake", "get_pet", "get_pet"},
		{"hyphens", "get-pet-by-id", "get_pet_by_id"},
		{"spaces", "Get Pet", "get_pet"},
		{"empty string", "", ""},
		{"single char", "A", "a"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toSnakeCase(tc.input)
			if got != tc.want {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizeSchema(t *testing.T) {
	emptyDoc := map[string]any{}

	t.Run("preserve integer type", func(t *testing.T) {
		schema := map[string]any{"type": "integer"}
		got := normalizeSchema(emptyDoc, schema, "")
		if got["type"] != "integer" {
			t.Errorf("expected type=integer, got %v", got["type"])
		}
	})

	t.Run("preserve boolean type", func(t *testing.T) {
		schema := map[string]any{"type": "boolean"}
		got := normalizeSchema(emptyDoc, schema, "")
		if got["type"] != "boolean" {
			t.Errorf("expected type=boolean, got %v", got["type"])
		}
	})

	t.Run("preserve number type", func(t *testing.T) {
		schema := map[string]any{"type": "number"}
		got := normalizeSchema(emptyDoc, schema, "")
		if got["type"] != "number" {
			t.Errorf("expected type=number, got %v", got["type"])
		}
	})

	t.Run("preserve array with items", func(t *testing.T) {
		schema := map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "string"},
		}
		got := normalizeSchema(emptyDoc, schema, "")
		if got["type"] != "array" {
			t.Errorf("expected type=array, got %v", got["type"])
		}
		items, ok := got["items"].(map[string]any)
		if !ok {
			t.Fatalf("expected items map, got %T", got["items"])
		}
		if items["type"] != "string" {
			t.Errorf("expected items type=string, got %v", items["type"])
		}
	})

	t.Run("preserve object with properties", func(t *testing.T) {
		schema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"age":  map[string]any{"type": "integer"},
			},
		}
		got := normalizeSchema(emptyDoc, schema, "")
		if got["type"] != "object" {
			t.Errorf("expected type=object, got %v", got["type"])
		}
		props, ok := got["properties"].(map[string]any)
		if !ok {
			t.Fatalf("expected properties map, got %T", got["properties"])
		}
		nameProp, ok := props["name"].(map[string]any)
		if !ok {
			t.Fatalf("expected name property map")
		}
		if nameProp["type"] != "string" {
			t.Errorf("expected name type=string, got %v", nameProp["type"])
		}
	})

	t.Run("preserve enum values", func(t *testing.T) {
		schema := map[string]any{
			"type": "string",
			"enum": []any{"a", "b", "c"},
		}
		got := normalizeSchema(emptyDoc, schema, "")
		enum, ok := got["enum"].([]any)
		if !ok {
			t.Fatalf("expected enum slice, got %T", got["enum"])
		}
		if len(enum) != 3 {
			t.Errorf("expected 3 enum values, got %d", len(enum))
		}
	})

	t.Run("preserve format", func(t *testing.T) {
		schema := map[string]any{"type": "integer", "format": "int32"}
		got := normalizeSchema(emptyDoc, schema, "")
		if got["format"] != "int32" {
			t.Errorf("expected format=int32, got %v", got["format"])
		}

		dateSchema := map[string]any{"type": "string", "format": "date-time"}
		got2 := normalizeSchema(emptyDoc, dateSchema, "")
		if got2["format"] != "date-time" {
			t.Errorf("expected format=date-time, got %v", got2["format"])
		}
	})

	t.Run("infer object type from properties", func(t *testing.T) {
		schema := map[string]any{
			"properties": map[string]any{
				"id": map[string]any{"type": "integer"},
			},
		}
		got := normalizeSchema(emptyDoc, schema, "")
		if got["type"] != "object" {
			t.Errorf("expected inferred type=object, got %v", got["type"])
		}
	})

	t.Run("nested object with nested properties", func(t *testing.T) {
		schema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"address": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"street": map[string]any{"type": "string"},
						"zip":    map[string]any{"type": "string"},
					},
				},
			},
		}
		got := normalizeSchema(emptyDoc, schema, "")
		props := got["properties"].(map[string]any)
		addrSchema := props["address"].(map[string]any)
		if addrSchema["type"] != "object" {
			t.Errorf("expected nested address type=object, got %v", addrSchema["type"])
		}
		nestedProps := addrSchema["properties"].(map[string]any)
		street := nestedProps["street"].(map[string]any)
		if street["type"] != "string" {
			t.Errorf("expected street type=string, got %v", street["type"])
		}
	})

	t.Run("$ref resolution via definitions", func(t *testing.T) {
		doc := map[string]any{
			"definitions": map[string]any{
				"Pet": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":   map[string]any{"type": "integer"},
						"name": map[string]any{"type": "string"},
					},
				},
			},
		}
		schema := map[string]any{"$ref": "#/definitions/Pet"}
		got := normalizeSchema(doc, schema, "")
		if got["type"] != "object" {
			t.Errorf("expected resolved type=object, got %v", got["type"])
		}
		props, ok := got["properties"].(map[string]any)
		if !ok {
			t.Fatalf("expected properties after $ref resolution")
		}
		if _, ok := props["id"]; !ok {
			t.Errorf("expected id property after $ref resolution")
		}
	})

	t.Run("description propagation from fallback", func(t *testing.T) {
		schema := map[string]any{"type": "string"}
		got := normalizeSchema(emptyDoc, schema, "The pet's name")
		if got["description"] != "The pet's name" {
			t.Errorf("expected description from fallback, got %v", got["description"])
		}
	})

	t.Run("schema description takes precedence over fallback", func(t *testing.T) {
		schema := map[string]any{"type": "string", "description": "from schema"}
		got := normalizeSchema(emptyDoc, schema, "fallback desc")
		if got["description"] != "from schema" {
			t.Errorf("expected schema description to win, got %v", got["description"])
		}
	})
}

func TestProxyToolDescription(t *testing.T) {
	t.Run("includes Args section with required and optional params", func(t *testing.T) {
		ep := openapi.Endpoint{
			Path:    "/pets/{id}",
			Method:  "GET",
			Summary: "Get a pet by ID",
		}
		schema := map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":     map[string]any{"type": "string", "description": "The pet ID"},
				"expand": map[string]any{"type": "boolean", "description": "Expand relations"},
			},
			"required": []string{"id"},
		}
		desc := proxyToolDescription(ep, schema)

		if !strings.Contains(desc, "Args:") {
			t.Error("expected Args section")
		}
		if !strings.Contains(desc, "id (string, required)") {
			t.Error("expected id as required param")
		}
		if !strings.Contains(desc, "expand (boolean, optional)") {
			t.Error("expected expand as optional param")
		}
	})

	t.Run("uses METHOD PATH as fallback summary", func(t *testing.T) {
		ep := openapi.Endpoint{Path: "/pets", Method: "post"}
		desc := proxyToolDescription(ep, map[string]any{})
		if !strings.Contains(desc, "POST /pets") {
			t.Errorf("expected fallback summary POST /pets, got: %s", desc)
		}
	})

	t.Run("skips Args section when no properties", func(t *testing.T) {
		ep := openapi.Endpoint{Path: "/health", Method: "GET", Summary: "Health check"}
		desc := proxyToolDescription(ep, map[string]any{})
		if strings.Contains(desc, "Args:") {
			t.Error("expected no Args section for empty schema")
		}
	})

	t.Run("includes Returns and Error Handling sections", func(t *testing.T) {
		ep := openapi.Endpoint{Path: "/ping", Method: "GET", Summary: "Ping"}
		desc := proxyToolDescription(ep, map[string]any{})
		if !strings.Contains(desc, "Returns:") {
			t.Error("expected Returns section")
		}
		if !strings.Contains(desc, "Error Handling:") {
			t.Error("expected Error Handling section")
		}
	})

	t.Run("includes Description paragraph when present", func(t *testing.T) {
		ep := openapi.Endpoint{
			Path:        "/pets",
			Method:      "GET",
			Summary:     "List pets",
			Description: "Returns all available pets in the store.",
		}
		desc := proxyToolDescription(ep, map[string]any{})
		if !strings.Contains(desc, "Returns all available pets in the store.") {
			t.Error("expected Description paragraph in output")
		}
	})
}

func TestInferProxyAnnotationsTitle(t *testing.T) {
	ann := inferProxyAnnotations("GET", "List All Pets")
	if ann.Title != "List All Pets" {
		t.Errorf("expected Title 'List All Pets', got %q", ann.Title)
	}
	if ann.ReadOnlyHint == nil || !*ann.ReadOnlyHint {
		t.Error("expected ReadOnlyHint=true for GET")
	}

	ann2 := inferProxyAnnotations("DELETE", "Remove Pet")
	if ann2.Title != "Remove Pet" {
		t.Errorf("expected Title 'Remove Pet', got %q", ann2.Title)
	}
	if ann2.DestructiveHint == nil || !*ann2.DestructiveHint {
		t.Error("expected DestructiveHint=true for DELETE")
	}
}

func TestHttpMethodToVerb(t *testing.T) {
cases := []struct{ method, want string }{
{"GET", "get"},
{"get", "get"},
{"POST", "create"},
{"post", "create"},
{"PUT", "update"},
{"PATCH", "update"},
{"DELETE", "delete"},
{"HEAD", "head"},
{"OPTIONS", "options"},
{"UNKNOWN", "unknown"},
}
for _, tc := range cases {
t.Run(tc.method, func(t *testing.T) {
got := httpMethodToVerb(tc.method)
if got != tc.want {
t.Errorf("httpMethodToVerb(%q) = %q, want %q", tc.method, got, tc.want)
}
})
}
}

func TestPathToolBaseName(t *testing.T) {
cases := []struct{ method, path, want string }{
{"POST", "/pets", "create_pets"},
{"PUT", "/pets/{id}", "update_pets_id"},
{"PATCH", "/users/{id}", "update_users_id"},
{"GET", "/pets", "get_pets"},
{"DELETE", "/pets/{id}", "delete_pets_id"},
}
for _, tc := range cases {
t.Run(tc.method+"_"+tc.path, func(t *testing.T) {
got := pathToolBaseName(tc.method, tc.path)
if got != tc.want {
t.Errorf("pathToolBaseName(%q, %q) = %q, want %q", tc.method, tc.path, got, tc.want)
}
})
}
}

func TestProxyToolName(t *testing.T) {
cases := []struct {
name     string
ep       openapi.Endpoint
apiName  string
apiTitle string
want     string
}{
{
name:     "apiName takes priority over title",
ep:       openapi.Endpoint{Path: "/pets", Method: "GET", OperationID: "listPets"},
apiName:  "myservice",
apiTitle: "Petstore API",
want:     "myservice_list_pets",
},
{
name:     "apiTitle used when apiName empty",
ep:       openapi.Endpoint{Path: "/pets", Method: "GET", OperationID: "listPets"},
apiName:  "",
apiTitle: "Petstore API",
want:     "petstore_api_list_pets",
},
{
name:     "fallback to api when both empty",
ep:       openapi.Endpoint{Path: "/pets", Method: "GET", OperationID: "listPets"},
apiName:  "",
apiTitle: "",
want:     "api_list_pets",
},
{
name:     "path fallback uses action verb",
ep:       openapi.Endpoint{Path: "/pets", Method: "POST"},
apiName:  "",
apiTitle: "My Service",
want:     "my_service_create_pets",
},
}
for _, tc := range cases {
t.Run(tc.name, func(t *testing.T) {
got := proxyToolName(tc.ep, tc.apiName, tc.apiTitle)
if got != tc.want {
t.Errorf("proxyToolName() = %q, want %q", got, tc.want)
}
})
}
}
