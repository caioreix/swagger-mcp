package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/caioreix/swagger-mcp/internal/format"
)

const defaultMultiAPIConfigFile = ".swagger-mcp.yaml"

// DefaultMultiAPIConfigFile is the file name auto-discovered in the working directory.
const DefaultMultiAPIConfigFile = defaultMultiAPIConfigFile

// LoadMultiAPIConfig reads a swagger-mcp.yaml config file and returns the list
// of API profiles defined under the "apis" key.
// Values that contain ${VAR} placeholders are expanded using os.Getenv.
func LoadMultiAPIConfig(path string) ([]APIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read multi-api config %q: %w", path, err)
	}

	raw, err := format.ParseYAML(data)
	if err != nil {
		return nil, fmt.Errorf("parse multi-api config %q: %w", path, err)
	}

	doc, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("multi-api config %q: root must be a YAML mapping", path)
	}

	apisRaw, ok := doc["apis"]
	if !ok {
		return nil, nil
	}

	// "apis:" with no block items parses as an empty string — treat as no APIs.
	if s, isStr := apisRaw.(string); isStr && strings.TrimSpace(s) == "" {
		return nil, nil
	}

	apisList, ok := apisRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("multi-api config %q: \"apis\" must be a list", path)
	}

	configs := make([]APIConfig, 0, len(apisList))
	for i, item := range apisList {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("multi-api config %q: apis[%d] must be a mapping", path, i)
		}
		cfg, err := parseAPIConfig(m, path, i)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	return configs, nil
}

func parseAPIConfig(m map[string]any, path string, index int) (APIConfig, error) {
	name := expandEnvVars(stringField(m, "name"))
	if name == "" {
		return APIConfig{}, fmt.Errorf("multi-api config %q: apis[%d] missing required field \"name\"", path, index)
	}

	cfg := APIConfig{
		Name:       name,
		SwaggerURL: expandEnvVars(stringField(m, "swagger_url")),
		BaseURL:    expandEnvVars(stringField(m, "base_url")),
		Headers:    expandEnvVars(stringField(m, "headers")),
		Filter: FilterConfig{
			IncludePaths:   expandEnvVars(stringField(m, "include_paths")),
			ExcludePaths:   expandEnvVars(stringField(m, "exclude_paths")),
			IncludeMethods: expandEnvVars(stringField(m, "include_methods")),
			ExcludeMethods: expandEnvVars(stringField(m, "exclude_methods")),
		},
	}

	if authRaw, ok := m["auth"]; ok {
		authMap, ok := authRaw.(map[string]any)
		if !ok {
			return APIConfig{}, fmt.Errorf("multi-api config %q: apis[%d].auth must be a mapping", path, index)
		}
		cfg.Auth = parseAuthConfig(authMap)
	}

	return cfg, nil
}

func parseAuthConfig(m map[string]any) AuthConfig {
	apiKeyHeader := expandEnvVars(stringField(m, "api_key_header"))
	if apiKeyHeader == "" {
		apiKeyHeader = "X-API-Key"
	}
	apiKeyIn := expandEnvVars(stringField(m, "api_key_in"))
	if apiKeyIn == "" {
		apiKeyIn = "header"
	}
	return AuthConfig{
		APIKey:       expandEnvVars(stringField(m, "api_key")),
		APIKeyHeader: apiKeyHeader,
		APIKeyIn:     apiKeyIn,
		BearerToken:  expandEnvVars(stringField(m, "bearer_token")),
		BasicUser:    expandEnvVars(stringField(m, "basic_user")),
		BasicPass:    expandEnvVars(stringField(m, "basic_pass")),
		OAuth2URL:    expandEnvVars(stringField(m, "oauth2_token_url")),
		OAuth2ID:     expandEnvVars(stringField(m, "oauth2_client_id")),
		OAuth2Secret: expandEnvVars(stringField(m, "oauth2_client_secret")),
		OAuth2Scopes: expandEnvVars(stringField(m, "oauth2_scopes")),
	}
}

// stringField returns the string value for key in m, or "" if absent or not a string.
func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// expandEnvVars replaces ${VAR} occurrences with os.Getenv("VAR").
func expandEnvVars(s string) string {
	if !strings.Contains(s, "${") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		start := strings.Index(s[i:], "${")
		if start == -1 {
			b.WriteString(s[i:])
			break
		}
		b.WriteString(s[i : i+start])
		i += start + 2 // skip "${"
		end := strings.Index(s[i:], "}")
		if end == -1 {
			// unterminated placeholder — write as-is
			b.WriteString("${")
			b.WriteString(s[i:])
			break
		}
		varName := s[i : i+end]
		b.WriteString(os.Getenv(varName))
		i += end + 1 // skip past "}"
	}
	return b.String()
}
