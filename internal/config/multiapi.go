package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/caioreix/swagger-mcp/internal/format"
)

const defaultMultiAPIConfigFile = ".swagger-mcp.yaml"

const templateVarPrefixLen = 2 // length of "${" prefix

// DefaultMultiAPIConfigFile is the file name auto-discovered in the working directory.
const DefaultMultiAPIConfigFile = defaultMultiAPIConfigFile

// ServerConfig holds global server settings from the YAML "server:" block.
type ServerConfig struct {
	Transport    string   // "stdio" | "sse" | "streamable-http"
	Port         string
	LogLevel     string
	EnableUI     bool
	ProxyHeaders []string // header names to forward from client to upstream APIs
}

// MultiAPIFile is the parsed content of the swagger-mcp YAML config file.
type MultiAPIFile struct {
	Server ServerConfig
	APIs   []APIConfig
}

// LoadMultiAPIConfig reads a swagger-mcp.yaml config file and returns the parsed content.
// Values that contain ${VAR} placeholders are expanded using os.Getenv.
func LoadMultiAPIConfig(path string) (*MultiAPIFile, error) {
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

	var serverCfg ServerConfig
	if serverRaw, ok := doc["server"]; ok {
		if sm, ok2 := serverRaw.(map[string]any); ok2 {
			serverCfg = parseServerConfig(sm)
		}
	}

	apisRaw, ok := doc["apis"]
	if !ok {
		return &MultiAPIFile{Server: serverCfg}, nil
	}

	// "apis:" with no block items parses as an empty string — treat as no APIs.
	if s, isStr := apisRaw.(string); isStr && strings.TrimSpace(s) == "" {
		return &MultiAPIFile{Server: serverCfg}, nil
	}

	apisList, ok := apisRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("multi-api config %q: \"apis\" must be a list", path)
	}

	configs := make([]APIConfig, 0, len(apisList))
	for i, item := range apisList {
		m, itemOk := item.(map[string]any)
		if !itemOk {
			return nil, fmt.Errorf("multi-api config %q: apis[%d] must be a mapping", path, i)
		}
		cfg, cfgErr := parseAPIConfig(m, path, i)
		if cfgErr != nil {
			return nil, cfgErr
		}
		configs = append(configs, cfg)
	}
	return &MultiAPIFile{Server: serverCfg, APIs: configs}, nil
}

func parseServerConfig(m map[string]any) ServerConfig {
	sc := ServerConfig{
		Transport: expandEnvVars(stringField(m, "transport")),
		Port:      expandEnvVars(numericStringField(m, "port")),
		LogLevel:  expandEnvVars(stringField(m, "log_level")),
	}
	if v, ok := m["enable_ui"]; ok {
		if b, ok2 := v.(bool); ok2 {
			sc.EnableUI = b
		}
	}
	if raw, ok := m["proxy_headers"]; ok {
		if list, ok2 := raw.([]any); ok2 {
			for _, item := range list {
				if s, ok3 := item.(string); ok3 && s != "" {
					sc.ProxyHeaders = append(sc.ProxyHeaders, strings.TrimSpace(s))
				}
			}
		}
	}
	return sc
}

func parseAPIConfig(m map[string]any, path string, index int) (APIConfig, error) {
	name := expandEnvVars(stringField(m, "name"))
	if name == "" {
		return APIConfig{}, fmt.Errorf("multi-api config %q: apis[%d] missing required field \"name\"", path, index)
	}

	specURL := expandEnvVars(stringField(m, "spec_url"))
	if specURL == "" {
		specURL = expandEnvVars(stringField(m, "swagger_url"))
	}

	cfg := APIConfig{
		Name:       name,
		SwaggerURL: specURL,
		BaseURL:    expandEnvVars(stringField(m, "base_url")),
		Headers:    parseHeadersField(m["headers"]),
		Filter:     parseFilterConfig(m),
	}

	if authRaw, ok := m["auth"]; ok {
		authMap, authMapOk := authRaw.(map[string]any)
		if !authMapOk {
			return APIConfig{}, fmt.Errorf("multi-api config %q: apis[%d].auth must be a mapping", path, index)
		}
		cfg.Auth = parseAuthConfig(authMap)
	}

	return cfg, nil
}

// parseHeadersField accepts either a map[string]any (new: {X-Tenant: acme}) or a
// plain string (old: "X-Tenant=acme,X-Version=v2") and normalises to the internal
// "key=value,key2=value2" CSV format.
func parseHeadersField(raw any) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return expandEnvVars(strings.TrimSpace(v))
	case map[string]any:
		parts := make([]string, 0, len(v))
		for k, val := range v {
			s, _ := val.(string)
			parts = append(parts, expandEnvVars(k)+"="+expandEnvVars(strings.TrimSpace(s)))
		}
		sort.Strings(parts) // deterministic
		return strings.Join(parts, ",")
	default:
		return ""
	}
}

// parseFilterConfig reads filter settings from either the new "filter:" block
// (with list values) or the legacy top-level string fields.
func parseFilterConfig(m map[string]any) FilterConfig {
	if raw, ok := m["filter"]; ok {
		if fm, ok2 := raw.(map[string]any); ok2 {
			return FilterConfig{
				IncludePaths:   parseStringOrList(fm["include_paths"]),
				ExcludePaths:   parseStringOrList(fm["exclude_paths"]),
				IncludeMethods: parseStringOrList(fm["include_methods"]),
				ExcludeMethods: parseStringOrList(fm["exclude_methods"]),
			}
		}
	}
	// Fall back to old top-level fields.
	return FilterConfig{
		IncludePaths:   expandEnvVars(stringField(m, "include_paths")),
		ExcludePaths:   expandEnvVars(stringField(m, "exclude_paths")),
		IncludeMethods: expandEnvVars(stringField(m, "include_methods")),
		ExcludeMethods: expandEnvVars(stringField(m, "exclude_methods")),
	}
}

// parseStringOrList normalises a YAML value that is either a plain string
// or a list of strings to the internal comma-separated format.
func parseStringOrList(raw any) string {
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return expandEnvVars(strings.TrimSpace(v))
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, expandEnvVars(strings.TrimSpace(s)))
			}
		}
		return strings.Join(parts, ",")
	default:
		return ""
	}
}

func parseAuthConfig(m map[string]any) AuthConfig {
	authType := strings.ToLower(strings.TrimSpace(expandEnvVars(stringField(m, "type"))))

	switch authType {
	case "bearer":
		return AuthConfig{
			APIKeyHeader: "X-API-Key", //nolint:gosec // G101: header name, not a credential
			APIKeyIn:     "header",
			BearerToken:  expandEnvVars(stringField(m, "token")),
		}
	case "api_key":
		apiKeyHeader := expandEnvVars(stringField(m, "header"))
		if apiKeyHeader == "" {
			apiKeyHeader = "X-API-Key" //nolint:gosec // G101: header name, not a credential
		}
		apiKeyIn := expandEnvVars(stringField(m, "in"))
		if apiKeyIn == "" {
			apiKeyIn = "header"
		}
		return AuthConfig{
			APIKey:       expandEnvVars(stringField(m, "key")),
			APIKeyHeader: apiKeyHeader,
			APIKeyIn:     apiKeyIn,
		}
	case "basic":
		return AuthConfig{
			APIKeyHeader: "X-API-Key", //nolint:gosec // G101: header name, not a credential
			APIKeyIn:     "header",
			BasicUser:    expandEnvVars(stringField(m, "user")),
			BasicPass:    expandEnvVars(stringField(m, "pass")),
		}
	case "oauth2":
		return AuthConfig{
			APIKeyHeader: "X-API-Key", //nolint:gosec // G101: header name, not a credential
			APIKeyIn:     "header",
			OAuth2URL:    expandEnvVars(stringField(m, "token_url")),
			OAuth2ID:     expandEnvVars(stringField(m, "client_id")),
			OAuth2Secret: expandEnvVars(stringField(m, "client_secret")),
			OAuth2Scopes: parseStringOrList(m["scopes"]),
		}
	default:
		// Legacy flat format (no "type:" field).
		apiKeyHeader := expandEnvVars(stringField(m, "api_key_header"))
		if apiKeyHeader == "" {
			apiKeyHeader = "X-API-Key" //nolint:gosec // G101: header name, not a credential
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

// numericStringField returns the value for key in m as a string, handling
// both string and integer YAML scalars (e.g. port: 8080).
func numericStringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case int:
		return strconv.Itoa(val)
	default:
		return ""
	}
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
		i += start + templateVarPrefixLen // skip "${"
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
