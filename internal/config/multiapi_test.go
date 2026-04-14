package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	config "github.com/caioreix/swagger-mcp/internal/config"
)

func TestLoadMultiAPIConfig_Basic(t *testing.T) {
	dir := t.TempDir()
	content := `
apis:
  - name: petstore
    spec_url: https://petstore.swagger.io/v2/swagger.json
    base_url: https://petstore.swagger.io/v2
    auth:
      type: bearer
      token: my-token
    headers:
      X-Tenant: acme
    filter:
      include_paths:
        - "^/pet.*"
      exclude_methods:
        - DELETE
  - name: github
    spec_url: ./specs/github.yaml
    auth:
      type: api_key
      key: ghp_xxx
      header: Authorization
      in: header
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := config.LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}
	apis := result.APIs

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
    spec_url: https://api.example.com/swagger.json
    auth:
      type: bearer
      token: ${TEST_BEARER}
  - name: myapi2
    spec_url: https://api2.example.com/swagger.json
    auth:
      type: api_key
      key: ${TEST_API_KEY}
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := config.LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}
	apis := result.APIs

	if len(apis) != 2 {
		t.Fatalf("expected 2 APIs, got %d", len(apis))
	}

	if apis[0].Auth.BearerToken != "secret-token" {
		t.Errorf("BearerToken = %q, want %q", apis[0].Auth.BearerToken, "secret-token")
	}
	if apis[1].Auth.APIKey != "api-key-value" {
		t.Errorf("APIKey = %q, want %q", apis[1].Auth.APIKey, "api-key-value")
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

	_, err := config.LoadMultiAPIConfig(path)
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
	result, err := config.LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}
	if len(result.APIs) != 0 {
		t.Errorf("expected 0 APIs, got %d", len(result.APIs))
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

	result, err := config.LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}
	if result.APIs != nil {
		t.Errorf("expected nil APIs, got %v", result.APIs)
	}
}

func TestLoadMultiAPIConfig_FileNotFound(t *testing.T) {
	_, err := config.LoadMultiAPIConfig("/nonexistent/path/.swagger-mcp.yaml")
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
		got := config.ExpandEnvVars(tc.input)
		if got != tc.want {
			t.Errorf("config.ExpandEnvVars(%q) = %q, want %q", tc.input, got, tc.want)
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

	result, err := config.LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}
	apis := result.APIs

	auth := apis[0].Auth
	if auth.APIKeyHeader != "X-API-Key" {
		t.Errorf("default APIKeyHeader = %q, want %q", auth.APIKeyHeader, "X-API-Key")
	}
	if auth.APIKeyIn != "header" {
		t.Errorf("default APIKeyIn = %q, want %q", auth.APIKeyIn, "header")
	}
}

func TestLoadMultiAPIConfig_NewFormat(t *testing.T) {
	dir := t.TempDir()
	content := `
server:
  transport: streamable-http
  port: "9090"
  log_level: debug
  enable_ui: true
  proxy_headers:
    - Authorization
    - X-Tenant-ID

apis:
  - name: petstore
    spec_url: https://petstore.swagger.io/v2/swagger.json
    base_url: https://petstore.swagger.io/v2
    auth:
      type: bearer
      token: bearer-tok
    headers:
      X-Tenant: acme
      X-Version: v2
    filter:
      include_paths:
        - "^/pet.*"
      exclude_methods:
        - DELETE
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := config.LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}

	// Verify server block.
	srv := result.Server
	if srv.Transport != "streamable-http" {
		t.Errorf("Server.Transport = %q, want %q", srv.Transport, "streamable-http")
	}
	if srv.Port != "9090" {
		t.Errorf("Server.Port = %q, want %q", srv.Port, "9090")
	}
	if srv.LogLevel != "debug" {
		t.Errorf("Server.LogLevel = %q, want %q", srv.LogLevel, "debug")
	}
	if !srv.EnableUI {
		t.Error("Server.EnableUI = false, want true")
	}
	if len(srv.ProxyHeaders) != 2 || srv.ProxyHeaders[0] != "Authorization" || srv.ProxyHeaders[1] != "X-Tenant-ID" {
		t.Errorf("Server.ProxyHeaders = %v, want [Authorization X-Tenant-ID]", srv.ProxyHeaders)
	}

	// Verify API.
	if len(result.APIs) != 1 {
		t.Fatalf("expected 1 API, got %d", len(result.APIs))
	}
	pet := result.APIs[0]
	if pet.SwaggerURL != "https://petstore.swagger.io/v2/swagger.json" {
		t.Errorf("SwaggerURL = %q", pet.SwaggerURL)
	}
	if pet.Auth.BearerToken != "bearer-tok" {
		t.Errorf("BearerToken = %q", pet.Auth.BearerToken)
	}
	// Map headers should be sorted: X-Tenant=acme,X-Version=v2.
	if pet.Headers != "X-Tenant=acme,X-Version=v2" {
		t.Errorf("Headers = %q, want %q", pet.Headers, "X-Tenant=acme,X-Version=v2")
	}
	if pet.Filter.IncludePaths != "^/pet.*" {
		t.Errorf("Filter.IncludePaths = %q", pet.Filter.IncludePaths)
	}
	if pet.Filter.ExcludeMethods != "DELETE" {
		t.Errorf("Filter.ExcludeMethods = %q", pet.Filter.ExcludeMethods)
	}
}

func TestLoadMultiAPIConfig_HeadersMap(t *testing.T) {
	dir := t.TempDir()
	content := `
apis:
  - name: mapheaders
    spec_url: https://api.example.com/swagger.json
    headers:
      X-Tenant: acme
      X-Version: v2
  - name: stringheaders
    spec_url: https://api.example.com/swagger.json
    headers: "X-Old=val1,X-Another=val2"
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := config.LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}
	apis := result.APIs

	// Map headers normalised to sorted CSV.
	if apis[0].Headers != "X-Tenant=acme,X-Version=v2" {
		t.Errorf("map headers = %q, want %q", apis[0].Headers, "X-Tenant=acme,X-Version=v2")
	}
	// Old string headers passed through unchanged.
	if apis[1].Headers != "X-Old=val1,X-Another=val2" {
		t.Errorf("string headers = %q, want %q", apis[1].Headers, "X-Old=val1,X-Another=val2")
	}
}

func TestLoadMultiAPIConfig_FilterLists(t *testing.T) {
	dir := t.TempDir()
	content := `
apis:
  - name: listfilter
    spec_url: https://api.example.com/swagger.json
    filter:
      include_paths:
        - "^/pet.*"
        - "^/user.*"
      exclude_methods:
        - DELETE
        - PUT
  - name: legacyfilter
    spec_url: https://api.example.com/swagger.json
    include_paths: "^/pet.*"
    exclude_methods: "DELETE"
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := config.LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}
	apis := result.APIs

	// New list format → CSV.
	if apis[0].Filter.IncludePaths != "^/pet.*,^/user.*" {
		t.Errorf("list IncludePaths = %q, want %q", apis[0].Filter.IncludePaths, "^/pet.*,^/user.*")
	}
	if apis[0].Filter.ExcludeMethods != "DELETE,PUT" {
		t.Errorf("list ExcludeMethods = %q, want %q", apis[0].Filter.ExcludeMethods, "DELETE,PUT")
	}
	// Legacy string format.
	if apis[1].Filter.IncludePaths != "^/pet.*" {
		t.Errorf("legacy IncludePaths = %q", apis[1].Filter.IncludePaths)
	}
	if apis[1].Filter.ExcludeMethods != "DELETE" {
		t.Errorf("legacy ExcludeMethods = %q", apis[1].Filter.ExcludeMethods)
	}
}

func TestLoadMultiAPIConfig_AuthTypes(t *testing.T) {
	dir := t.TempDir()
	content := `
apis:
  - name: bearer-api
    spec_url: https://api.example.com/swagger.json
    auth:
      type: bearer
      token: tok123
  - name: apikey-api
    spec_url: https://api.example.com/swagger.json
    auth:
      type: api_key
      key: key456
      header: X-Custom-Key
      in: query
  - name: basic-api
    spec_url: https://api.example.com/swagger.json
    auth:
      type: basic
      user: alice
      pass: s3cr3t
  - name: oauth2-api
    spec_url: https://api.example.com/swagger.json
    auth:
      type: oauth2
      token_url: https://auth.example.com/token
      client_id: cid
      client_secret: csecret
      scopes:
        - "read:data"
        - "write:data"
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := config.LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}
	apis := result.APIs

	if len(apis) != 4 {
		t.Fatalf("expected 4 APIs, got %d", len(apis))
	}

	// bearer
	if apis[0].Auth.BearerToken != "tok123" {
		t.Errorf("bearer token = %q", apis[0].Auth.BearerToken)
	}
	if apis[0].Auth.APIKeyHeader != "X-API-Key" {
		t.Errorf("bearer default header = %q", apis[0].Auth.APIKeyHeader)
	}

	// api_key
	if apis[1].Auth.APIKey != "key456" {
		t.Errorf("api_key = %q", apis[1].Auth.APIKey)
	}
	if apis[1].Auth.APIKeyHeader != "X-Custom-Key" {
		t.Errorf("api_key header = %q", apis[1].Auth.APIKeyHeader)
	}
	if apis[1].Auth.APIKeyIn != "query" {
		t.Errorf("api_key in = %q", apis[1].Auth.APIKeyIn)
	}

	// basic
	if apis[2].Auth.BasicUser != "alice" {
		t.Errorf("basic user = %q", apis[2].Auth.BasicUser)
	}
	if apis[2].Auth.BasicPass != "s3cr3t" {
		t.Errorf("basic pass = %q", apis[2].Auth.BasicPass)
	}

	// oauth2
	if apis[3].Auth.OAuth2URL != "https://auth.example.com/token" {
		t.Errorf("oauth2 url = %q", apis[3].Auth.OAuth2URL)
	}
	if apis[3].Auth.OAuth2ID != "cid" {
		t.Errorf("oauth2 client_id = %q", apis[3].Auth.OAuth2ID)
	}
	if apis[3].Auth.OAuth2Secret != "csecret" {
		t.Errorf("oauth2 secret = %q", apis[3].Auth.OAuth2Secret)
	}
	if apis[3].Auth.OAuth2Scopes != "read:data,write:data" {
		t.Errorf("oauth2 scopes = %q, want %q", apis[3].Auth.OAuth2Scopes, "read:data,write:data")
	}
}

func TestLoadMultiAPIConfig_BackwardCompat(t *testing.T) {
	dir := t.TempDir()
	// Old flat format should still parse correctly.
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

	result, err := config.LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}
	apis := result.APIs

	if len(apis) != 2 {
		t.Fatalf("expected 2 APIs, got %d", len(apis))
	}
	if apis[0].SwaggerURL != "https://petstore.swagger.io/v2/swagger.json" {
		t.Errorf("SwaggerURL = %q", apis[0].SwaggerURL)
	}
	if apis[0].Auth.BearerToken != "my-token" {
		t.Errorf("BearerToken = %q", apis[0].Auth.BearerToken)
	}
	if apis[0].Headers != "X-Tenant=acme" {
		t.Errorf("Headers = %q", apis[0].Headers)
	}
	if apis[0].Filter.IncludePaths != "^/pet.*" {
		t.Errorf("IncludePaths = %q", apis[0].Filter.IncludePaths)
	}
	if apis[0].Filter.ExcludeMethods != "DELETE" {
		t.Errorf("ExcludeMethods = %q", apis[0].Filter.ExcludeMethods)
	}
	if apis[1].Auth.APIKey != "ghp_xxx" {
		t.Errorf("APIKey = %q", apis[1].Auth.APIKey)
	}
	if apis[1].Auth.APIKeyHeader != "Authorization" {
		t.Errorf("APIKeyHeader = %q", apis[1].Auth.APIKeyHeader)
	}
}

func TestLoadMultiAPIConfig_ServerBlock(t *testing.T) {
	dir := t.TempDir()
	content := `
server:
  transport: sse
  port: "9000"
  log_level: warn
  enable_ui: true
  proxy_headers:
    - Authorization
    - X-Request-ID

apis:
  - name: dummy
    spec_url: https://api.example.com/swagger.json
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := config.LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}

	srv := result.Server
	if srv.Transport != "sse" {
		t.Errorf("Transport = %q, want sse", srv.Transport)
	}
	if srv.Port != "9000" {
		t.Errorf("Port = %q, want 9000", srv.Port)
	}
	if srv.LogLevel != "warn" {
		t.Errorf("LogLevel = %q, want warn", srv.LogLevel)
	}
	if !srv.EnableUI {
		t.Error("EnableUI = false, want true")
	}
	wantHeaders := []string{"Authorization", "X-Request-ID"}
	if len(srv.ProxyHeaders) != len(wantHeaders) {
		t.Fatalf("ProxyHeaders = %v, want %v", srv.ProxyHeaders, wantHeaders)
	}
	for i, h := range wantHeaders {
		if srv.ProxyHeaders[i] != h {
			t.Errorf("ProxyHeaders[%d] = %q, want %q", i, srv.ProxyHeaders[i], h)
		}
	}
}

func TestLoadMultiAPIConfig_SpecURLAlias(t *testing.T) {
	dir := t.TempDir()
	content := `
apis:
  - name: new-style
    spec_url: https://api.example.com/v1/swagger.json
  - name: old-style
    swagger_url: https://api.example.com/v2/swagger.json
`
	path := filepath.Join(dir, ".swagger-mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := config.LoadMultiAPIConfig(path)
	if err != nil {
		t.Fatalf("LoadMultiAPIConfig: %v", err)
	}
	apis := result.APIs

	if apis[0].SwaggerURL != "https://api.example.com/v1/swagger.json" {
		t.Errorf("spec_url not loaded: %q", apis[0].SwaggerURL)
	}
	if apis[1].SwaggerURL != "https://api.example.com/v2/swagger.json" {
		t.Errorf("swagger_url (alias) not loaded: %q", apis[1].SwaggerURL)
	}
}

