package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMultiAPIConfig_Basic(t *testing.T) {
	dir := t.TempDir()
	content := `
apis:
  - name: petstore
    swagger_url: https://petstore.swagger.io/v2/swagger.json
    base_url: https://petstore.swagger.io/v2
    auth:
      bearer_token: my-token
    headers: "X-Tenant=acme"
    include_paths: "^/pet.*"
    exclude_methods: "DELETE"
  - name: github
    swagger_url: ./specs/github.yaml
    auth:
      api_key: ghp_xxx
      api_key_header: Authorization
      api_key_in: header
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	apis, err := LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}

	if len(apis) != 2 {
		t.Fatalf("expected 2 APIs, got %d", len(apis))
	}

	pet := apis[0]
	if pet.Name != "petstore" {
		t.Errorf("apis[0].Name = %q, want %q", pet.Name, "petstore")
	}
	if pet.SwaggerURL != "https://petstore.swagger.io/v2/swagger.json" {
		t.Errorf("apis[0].SwaggerURL = %q", pet.SwaggerURL)
	}
	if pet.BaseURL != "https://petstore.swagger.io/v2" {
		t.Errorf("apis[0].BaseURL = %q", pet.BaseURL)
	}
	if pet.Auth.BearerToken != "my-token" {
		t.Errorf("apis[0].Auth.BearerToken = %q", pet.Auth.BearerToken)
	}
	if pet.Headers != "X-Tenant=acme" {
		t.Errorf("apis[0].Headers = %q", pet.Headers)
	}
	if pet.Filter.IncludePaths != "^/pet.*" {
		t.Errorf("apis[0].Filter.IncludePaths = %q", pet.Filter.IncludePaths)
	}
	if pet.Filter.ExcludeMethods != "DELETE" {
		t.Errorf("apis[0].Filter.ExcludeMethods = %q", pet.Filter.ExcludeMethods)
	}

	gh := apis[1]
	if gh.Name != "github" {
		t.Errorf("apis[1].Name = %q", gh.Name)
	}
	if gh.Auth.APIKey != "ghp_xxx" {
		t.Errorf("apis[1].Auth.APIKey = %q", gh.Auth.APIKey)
	}
	if gh.Auth.APIKeyHeader != "Authorization" {
		t.Errorf("apis[1].Auth.APIKeyHeader = %q", gh.Auth.APIKeyHeader)
	}
	if gh.Auth.APIKeyIn != "header" {
		t.Errorf("apis[1].Auth.APIKeyIn = %q", gh.Auth.APIKeyIn)
	}
}

func TestLoadMultiAPIConfig_EnvVarInterpolation(t *testing.T) {
	t.Setenv("TEST_BEARER", "secret-token")
	t.Setenv("TEST_API_KEY", "api-key-value")

	dir := t.TempDir()
	content := `
apis:
  - name: myapi
    swagger_url: https://api.example.com/swagger.json
    auth:
      bearer_token: ${TEST_BEARER}
      api_key: ${TEST_API_KEY}
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	apis, err := LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}

	if len(apis) != 1 {
		t.Fatalf("expected 1 API, got %d", len(apis))
	}

	if apis[0].Auth.BearerToken != "secret-token" {
		t.Errorf("BearerToken = %q, want %q", apis[0].Auth.BearerToken, "secret-token")
	}
	if apis[0].Auth.APIKey != "api-key-value" {
		t.Errorf("APIKey = %q, want %q", apis[0].Auth.APIKey, "api-key-value")
	}
}

func TestLoadMultiAPIConfig_MissingName(t *testing.T) {
	dir := t.TempDir()
	content := `
apis:
  - swagger_url: https://api.example.com/swagger.json
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMultiAPIConfig(path)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
	if !strings.Contains(err.Error(), "missing required field \"name\"") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadMultiAPIConfig_EmptyAPIs(t *testing.T) {
	dir := t.TempDir()
	// Block-style empty list (no items under "apis:").
	content := `
apis:
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	// "apis:" with no block items parses as empty string — treated as no APIs configured.
	apis, err := LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}
	if len(apis) != 0 {
		t.Errorf("expected 0 APIs, got %d", len(apis))
	}
}

func TestLoadMultiAPIConfig_NoAPIsKey(t *testing.T) {
	dir := t.TempDir()
	content := `
other_key: value
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	apis, err := LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}
	if apis != nil {
		t.Errorf("expected nil APIs, got %v", apis)
	}
}

func TestLoadMultiAPIConfig_FileNotFound(t *testing.T) {
	_, err := LoadMultiAPIConfig("/nonexistent/path/.swagger-mcp.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestExpandEnvVars(t *testing.T) {
	t.Setenv("MY_VAR", "hello")
	t.Setenv("ANOTHER", "world")

	cases := []struct {
		input string
		want  string
	}{
		{"no vars here", "no vars here"},
		{"${MY_VAR}", "hello"},
		{"prefix_${MY_VAR}_suffix", "prefix_hello_suffix"},
		{"${MY_VAR} and ${ANOTHER}", "hello and world"},
		{"${UNDEFINED_VAR_XYZ}", ""},
		{"${unterminated", "${unterminated"},
	}

	for _, tc := range cases {
		got := expandEnvVars(tc.input)
		if got != tc.want {
			t.Errorf("expandEnvVars(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestLoadMultiAPIConfig_AuthDefaults(t *testing.T) {
	dir := t.TempDir()
	content := `
apis:
  - name: myapi
    swagger_url: https://api.example.com/swagger.json
    auth:
      bearer_token: tok
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	apis, err := LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}

	auth := apis[0].Auth
	if auth.APIKeyHeader != "X-API-Key" {
		t.Errorf("default APIKeyHeader = %q, want %q", auth.APIKeyHeader, "X-API-Key")
	}
	if auth.APIKeyIn != "header" {
		t.Errorf("default APIKeyIn = %q, want %q", auth.APIKeyIn, "header")
	}
}
