package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeArgs(t *testing.T) {
	args := normalizeArgs([]string{"--swaggerUrl=https://example.com/swagger.json", "--other"})
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "--swagger-url=https://example.com/swagger.json" {
		t.Fatalf("expected normalized swagger-url arg, got %q", args[0])
	}
}

func TestLoadDefaultsWithoutDotEnv(t *testing.T) {
	temporaryDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(temporaryDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(previousDir)
	}()

	t.Setenv("LOG_LEVEL", "")
	config, err := Load(nil)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if config.LogLevel != "info" {
		t.Fatalf("expected default log level info, got %q", config.LogLevel)
	}
	if config.WorkingDir != temporaryDir {
		t.Fatalf("expected working dir %q, got %q", temporaryDir, config.WorkingDir)
	}
}

func TestLoadReadsDotEnvWithoutOverridingExistingEnv(t *testing.T) {
	temporaryDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(temporaryDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(previousDir)
	}()

	dotEnvPath := filepath.Join(temporaryDir, ".env")
	if err := os.WriteFile(dotEnvPath, []byte("LOG_LEVEL=debug\n"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("LOG_LEVEL", "")
	config, err := Load(nil)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if config.LogLevel != "debug" {
		t.Fatalf("expected LOG_LEVEL from .env, got %q", config.LogLevel)
	}

	t.Setenv("LOG_LEVEL", "warn")
	config, err = Load(nil)
	if err != nil {
		t.Fatalf("Load returned error with preset env: %v", err)
	}
	if config.LogLevel != "warn" {
		t.Fatalf("expected preset env to win, got %q", config.LogLevel)
	}
}

func TestLoadParsesSwaggerURLFlag(t *testing.T) {
	temporaryDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(temporaryDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(previousDir)
	}()

	config, err := Load([]string{"--swagger-url=https://example.com/swagger.json"})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if config.SwaggerURL != "https://example.com/swagger.json" {
		t.Fatalf("expected swagger URL to be parsed, got %q", config.SwaggerURL)
	}
}

func TestLoadAuthConfigFromEnv(t *testing.T) {
	temporaryDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(temporaryDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(previousDir)
	}()

	t.Setenv("API_KEY", "test-key-123")
	t.Setenv("API_KEY_HEADER", "Authorization")
	t.Setenv("API_KEY_IN", "query")
	t.Setenv("BEARER_TOKEN", "jwt-token")
	t.Setenv("BASIC_AUTH_USER", "admin")
	t.Setenv("BASIC_AUTH_PASS", "secret")
	t.Setenv("OAUTH2_TOKEN_URL", "https://auth.example.com/token")
	t.Setenv("OAUTH2_CLIENT_ID", "my-client")
	t.Setenv("OAUTH2_CLIENT_SECRET", "my-secret")
	t.Setenv("OAUTH2_SCOPES", "read,write")

	config, err := Load(nil)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if config.Auth.APIKey != "test-key-123" {
		t.Fatalf("expected API_KEY test-key-123, got %q", config.Auth.APIKey)
	}
	if config.Auth.APIKeyHeader != "Authorization" {
		t.Fatalf("expected API_KEY_HEADER Authorization, got %q", config.Auth.APIKeyHeader)
	}
	if config.Auth.APIKeyIn != "query" {
		t.Fatalf("expected API_KEY_IN query, got %q", config.Auth.APIKeyIn)
	}
	if config.Auth.BearerToken != "jwt-token" {
		t.Fatalf("expected BEARER_TOKEN jwt-token, got %q", config.Auth.BearerToken)
	}
	if config.Auth.BasicUser != "admin" {
		t.Fatalf("expected BASIC_AUTH_USER admin, got %q", config.Auth.BasicUser)
	}
	if config.Auth.OAuth2URL != "https://auth.example.com/token" {
		t.Fatalf("expected OAUTH2_TOKEN_URL, got %q", config.Auth.OAuth2URL)
	}
	if config.Auth.OAuth2Scopes != "read,write" {
		t.Fatalf("expected OAUTH2_SCOPES read,write, got %q", config.Auth.OAuth2Scopes)
	}
}

func TestLoadAuthConfigDefaults(t *testing.T) {
	temporaryDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(temporaryDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(previousDir)
	}()

	t.Setenv("API_KEY", "")
	t.Setenv("API_KEY_HEADER", "")
	t.Setenv("API_KEY_IN", "")
	t.Setenv("BEARER_TOKEN", "")
	t.Setenv("BASIC_AUTH_USER", "")
	t.Setenv("BASIC_AUTH_PASS", "")
	t.Setenv("OAUTH2_TOKEN_URL", "")
	t.Setenv("OAUTH2_CLIENT_ID", "")
	t.Setenv("OAUTH2_CLIENT_SECRET", "")
	t.Setenv("OAUTH2_SCOPES", "")

	config, err := Load(nil)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if config.Auth.APIKeyHeader != "X-API-Key" {
		t.Fatalf("expected default API_KEY_HEADER X-API-Key, got %q", config.Auth.APIKeyHeader)
	}
	if config.Auth.APIKeyIn != "header" {
		t.Fatalf("expected default API_KEY_IN header, got %q", config.Auth.APIKeyIn)
	}
}

func TestLoadProxyModeFlags(t *testing.T) {
	temporaryDir := t.TempDir()
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(temporaryDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(previousDir)
	}()

	config, err := Load([]string{
		"--proxy-mode",
		"--base-url=https://api.example.com",
		"--headers=X-Custom=value,X-Tenant=123",
		"--include-paths=^/pets.*,^/users.*",
		"--exclude-paths=.*admin.*",
		"--include-methods=GET,POST",
		"--exclude-methods=DELETE",
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if !config.ProxyMode {
		t.Fatal("expected ProxyMode to be true")
	}
	if config.BaseURL != "https://api.example.com" {
		t.Fatalf("expected BaseURL, got %q", config.BaseURL)
	}
	if config.Headers != "X-Custom=value,X-Tenant=123" {
		t.Fatalf("expected Headers, got %q", config.Headers)
	}
	if config.Filter.IncludePaths != "^/pets.*,^/users.*" {
		t.Fatalf("expected IncludePaths, got %q", config.Filter.IncludePaths)
	}
	if config.Filter.ExcludePaths != ".*admin.*" {
		t.Fatalf("expected ExcludePaths, got %q", config.Filter.ExcludePaths)
	}
	if config.Filter.IncludeMethods != "GET,POST" {
		t.Fatalf("expected IncludeMethods, got %q", config.Filter.IncludeMethods)
	}
	if config.Filter.ExcludeMethods != "DELETE" {
		t.Fatalf("expected ExcludeMethods, got %q", config.Filter.ExcludeMethods)
	}
}
