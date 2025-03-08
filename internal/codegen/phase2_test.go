package codegen

import (
	"strings"
	"testing"

	"github.com/caioreix/swagger-mcp/internal/openapi"
)

func TestGenerateErrorHelpers(t *testing.T) {
	code := generateErrorHelpers()
	checks := []string{
		"func mcpTextResult(",
		"func mcpError(",
		"func httpStatusToMCPError(",
		`"isError": true`,
	}
	for _, check := range checks {
		if !strings.Contains(code, check) {
			t.Fatalf("expected error helpers to contain %q", check)
		}
	}
}

func TestGenerateHandlerWithErrors(t *testing.T) {
	code := generateHandlerWithErrors("addPet", "POST", "/pets")
	checks := []string{
		"func HandleAddPet(input map[string]any) (result map[string]any)",
		"defer func()",
		"recover()",
		"mcpTextResult(",
	}
	for _, check := range checks {
		if !strings.Contains(code, check) {
			t.Fatalf("expected handler with errors to contain %q\n%s", check, code)
		}
	}
}

func TestGenerateAuthHelpers(t *testing.T) {
	schemes := []openapi.SecurityScheme{
		{Name: "api_key", Type: "apiKey", In: "header", ParamName: "X-API-Key"},
		{Name: "bearer", Type: "http", Scheme: "bearer"},
		{Name: "basic", Type: "http", Scheme: "basic"},
		{Name: "oauth2", Type: "oauth2", FlowType: "clientCredentials"},
	}
	code := generateAuthHelpers(schemes)
	checks := []string{
		"func applyAuth(req *http.Request)",
		`os.Getenv("API_KEY")`,
		`"Authorization", "Bearer "`,
		"SetBasicAuth",
		"fetchOAuth2Token()",
	}
	for _, check := range checks {
		if !strings.Contains(code, check) {
			t.Fatalf("expected auth helpers to contain %q\n%s", check, code)
		}
	}
}

func TestGenerateAuthHelpersEmpty(t *testing.T) {
	code := generateAuthHelpers(nil)
	if code != "" {
		t.Fatalf("expected empty auth helpers for nil schemes, got: %s", code)
	}
}

func TestNeedsOAuth2Fetcher(t *testing.T) {
	schemes := []openapi.SecurityScheme{
		{Name: "bearer", Type: "http", Scheme: "bearer"},
	}
	if needsOAuth2Fetcher(schemes) {
		t.Fatal("expected no OAuth2 fetcher for bearer-only schemes")
	}
	schemes = append(schemes, openapi.SecurityScheme{Name: "oauth", Type: "oauth2"})
	if !needsOAuth2Fetcher(schemes) {
		t.Fatal("expected OAuth2 fetcher when oauth2 scheme is present")
	}
}

func TestDetectFileOperationUpload(t *testing.T) {
	// Swagger 2.0 file type
	op := map[string]any{
		"parameters": []any{
			map[string]any{"name": "file", "type": "file", "in": "formData"},
		},
	}
	if DetectFileOperation(op) != FileOpUpload {
		t.Fatal("expected file upload detection for type:file parameter")
	}

	// OpenAPI 3.x multipart
	op = map[string]any{
		"requestBody": map[string]any{
			"content": map[string]any{
				"multipart/form-data": map[string]any{},
			},
		},
	}
	if DetectFileOperation(op) != FileOpUpload {
		t.Fatal("expected file upload detection for multipart/form-data")
	}
}

func TestDetectFileOperationDownload(t *testing.T) {
	op := map[string]any{
		"responses": map[string]any{
			"200": map[string]any{
				"content": map[string]any{
					"application/octet-stream": map[string]any{},
				},
			},
		},
	}
	if DetectFileOperation(op) != FileOpDownload {
		t.Fatal("expected file download detection for octet-stream response")
	}
}

func TestDetectFileOperationNone(t *testing.T) {
	op := map[string]any{
		"responses": map[string]any{
			"200": map[string]any{
				"content": map[string]any{
					"application/json": map[string]any{},
				},
			},
		},
	}
	if DetectFileOperation(op) != FileOpNone {
		t.Fatal("expected no file operation for JSON-only endpoint")
	}
}

func TestGenerateValidationCode(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"required": []any{"name", "age"},
		"properties": map[string]any{
			"name": map[string]any{
				"type":      "string",
				"minLength": 1,
				"maxLength": 100,
			},
			"age": map[string]any{
				"type":    "integer",
				"minimum": 0,
				"maximum": 150,
			},
			"status": map[string]any{
				"type": "string",
				"enum": []any{"active", "inactive"},
			},
			"tags": map[string]any{
				"type":     "array",
				"minItems": 1,
			},
		},
	}
	code := generateValidationCode(schema)
	checks := []string{
		"func validateInput(",
		`"missing required field: name"`,
		`"missing required field: age"`,
		"must be a string",
		"must be a number",
		"must be an array",
		"must be one of",
		"at least 1 characters",
		"at most 100 characters",
	}
	for _, check := range checks {
		if !strings.Contains(code, check) {
			t.Fatalf("expected validation code to contain %q\n%s", check, code)
		}
	}
}

func TestGenerateValidationHelpers(t *testing.T) {
	code := generateValidationHelpers()
	if !strings.Contains(code, "func toFloat64(") {
		t.Fatal("expected toFloat64 helper function")
	}
}

func TestAuthImports(t *testing.T) {
	schemes := []openapi.SecurityScheme{
		{Name: "bearer", Type: "http", Scheme: "bearer"},
	}
	imports := authImports(schemes)
	if len(imports) != 2 {
		t.Fatalf("expected 2 imports for bearer auth, got %d", len(imports))
	}

	schemes = append(schemes, openapi.SecurityScheme{Name: "oauth", Type: "oauth2"})
	imports = authImports(schemes)
	if len(imports) < 4 {
		t.Fatalf("expected at least 4 imports for oauth2, got %d", len(imports))
	}
}

func TestFileOpsImports(t *testing.T) {
	imports := fileOpsImports(FileOpUpload)
	if len(imports) == 0 {
		t.Fatal("expected imports for file upload")
	}
	imports = fileOpsImports(FileOpDownload)
	if len(imports) == 0 {
		t.Fatal("expected imports for file download")
	}
	imports = fileOpsImports(FileOpNone)
	if len(imports) != 0 {
		t.Fatal("expected no imports for no file op")
	}
}
