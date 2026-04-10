package config

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const Version = "1.0.1"

// AuthConfig holds authentication settings loaded from environment variables.
type AuthConfig struct {
	APIKey       string // API_KEY
	APIKeyHeader string // API_KEY_HEADER (default: "X-API-Key")
	APIKeyIn     string // API_KEY_IN ("header" or "query", default: "header")
	BearerToken  string // BEARER_TOKEN
	BasicUser    string // BASIC_AUTH_USER
	BasicPass    string // BASIC_AUTH_PASS
	OAuth2URL    string // OAUTH2_TOKEN_URL
	OAuth2ID     string // OAUTH2_CLIENT_ID
	OAuth2Secret string // OAUTH2_CLIENT_SECRET
	OAuth2Scopes string // OAUTH2_SCOPES (comma-separated)
}

// FilterConfig holds endpoint filtering settings.
type FilterConfig struct {
	IncludePaths   string // comma-separated regex patterns for paths to include
	ExcludePaths   string // comma-separated regex patterns for paths to exclude
	IncludeMethods string // comma-separated HTTP methods to include (e.g. "GET,POST")
	ExcludeMethods string // comma-separated HTTP methods to exclude
}

// APIConfig defines a named API profile used in multi-API mode.
// Each profile corresponds to one entry in the "apis" array of swagger-mcp.yaml.
type APIConfig struct {
	Name       string
	SwaggerURL string
	BaseURL    string
	Auth       AuthConfig
	Headers    string
	Filter     FilterConfig
}

type Config struct {
	SwaggerURL string
	LogLevel   string
	WorkingDir string
	Auth       AuthConfig
	Filter     FilterConfig
	Transport  string // "stdio", "sse", "streamable-http"
	Port       string // HTTP port (default: "8080")
	EnableUI   bool   // enable web UI
	ProxyMode   bool   // enable dynamic proxy mode
	BaseURL     string // override base URL from swagger spec
	Headers     string // custom headers (name1=value1,name2=value2)
	SseHeaders  string // comma-separated header names to forward from SSE requests to proxy calls
	HttpHeaders string // comma-separated header names to forward from StreamableHTTP requests to proxy calls
	// APIs holds multiple named API profiles loaded from the swagger-mcp.yaml config file.
	// When populated, each API generates its own set of proxy tools.
	APIs []APIConfig
}

func load(args []string) (Config, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return Config{}, fmt.Errorf("get working directory: %w", err)
	}

	if err := LoadDotEnv(filepath.Join(workingDir, ".env")); err != nil {
		return Config{}, err
	}

	normalizedArgs := normalizeArgs(args)
	flagSet := flag.NewFlagSet("swagger-mcp", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	swaggerURL := flagSet.String("swagger-url", "", "URL of the Swagger/OpenAPI definition")
	transport := flagSet.String("transport", "stdio", "Transport mode: stdio, sse, streamable-http")
	port := flagSet.String("port", "8080", "Port for HTTP transports")
	enableUI := flagSet.Bool("ui", false, "Enable web UI for testing tools")
	proxyMode := flagSet.Bool("proxy-mode", false, "Enable dynamic proxy mode: each Swagger endpoint becomes an MCP tool")
	baseURL := flagSet.String("base-url", "", "Override the base URL from the Swagger spec")
	headers := flagSet.String("headers", "", "Custom headers for proxy requests (name1=value1,name2=value2)")
	includePaths := flagSet.String("include-paths", "", "Regex patterns for paths to include (comma-separated)")
	excludePaths := flagSet.String("exclude-paths", "", "Regex patterns for paths to exclude (comma-separated)")
	includeMethods := flagSet.String("include-methods", "", "HTTP methods to include (comma-separated, e.g. GET,POST)")
	excludeMethods := flagSet.String("exclude-methods", "", "HTTP methods to exclude (comma-separated)")
	sseHeaders := flagSet.String("sse-headers", "", "Header names to forward from SSE client requests to proxy API calls (comma-separated, e.g. Authorization,X-Tenant-ID)")
	httpHeaders := flagSet.String("http-headers", "", "Header names to forward from StreamableHTTP client requests to proxy API calls (comma-separated, e.g. Authorization,X-Tenant-ID)")
	if err := flagSet.Parse(normalizedArgs); err != nil {
		return Config{}, err
	}

	logLevel := strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	if logLevel == "" {
		logLevel = "info"
	}

	return Config{
		SwaggerURL: strings.TrimSpace(*swaggerURL),
		LogLevel:   logLevel,
		WorkingDir: workingDir,
		Auth:       LoadAuthConfig(),
		Filter: FilterConfig{
			IncludePaths:   strings.TrimSpace(*includePaths),
			ExcludePaths:   strings.TrimSpace(*excludePaths),
			IncludeMethods: strings.TrimSpace(*includeMethods),
			ExcludeMethods: strings.TrimSpace(*excludeMethods),
		},
		Transport:  strings.TrimSpace(*transport),
		Port:       strings.TrimSpace(*port),
		EnableUI:   *enableUI,
		ProxyMode:  *proxyMode,
		BaseURL:    strings.TrimSpace(*baseURL),
		Headers:    strings.TrimSpace(*headers),
		SseHeaders:  strings.TrimSpace(*sseHeaders),
		HttpHeaders: strings.TrimSpace(*httpHeaders),
	}, nil
}

func LoadAuthConfig() AuthConfig {
	apiKeyHeader := strings.TrimSpace(os.Getenv("API_KEY_HEADER"))
	if apiKeyHeader == "" {
		apiKeyHeader = "X-API-Key"
	}
	apiKeyIn := strings.TrimSpace(os.Getenv("API_KEY_IN"))
	if apiKeyIn == "" {
		apiKeyIn = "header"
	}
	return AuthConfig{
		APIKey:       strings.TrimSpace(os.Getenv("API_KEY")),
		APIKeyHeader: apiKeyHeader,
		APIKeyIn:     apiKeyIn,
		BearerToken:  strings.TrimSpace(os.Getenv("BEARER_TOKEN")),
		BasicUser:    strings.TrimSpace(os.Getenv("BASIC_AUTH_USER")),
		BasicPass:    strings.TrimSpace(os.Getenv("BASIC_AUTH_PASS")),
		OAuth2URL:    strings.TrimSpace(os.Getenv("OAUTH2_TOKEN_URL")),
		OAuth2ID:     strings.TrimSpace(os.Getenv("OAUTH2_CLIENT_ID")),
		OAuth2Secret: strings.TrimSpace(os.Getenv("OAUTH2_CLIENT_SECRET")),
		OAuth2Scopes: strings.TrimSpace(os.Getenv("OAUTH2_SCOPES")),
	}
}

func normalizeArgs(args []string) []string {
	normalized := make([]string, 0, len(args))
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--swaggerUrl="):
			normalized = append(normalized, "--swagger-url="+strings.TrimPrefix(arg, "--swaggerUrl="))
		case arg == "--swaggerUrl":
			normalized = append(normalized, "--swagger-url")
		default:
			normalized = append(normalized, arg)
		}
	}
	return normalized
}

func LoadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open .env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(strings.Trim(value, "\"'"))
		if key == "" {
			continue
		}
		if existingValue, exists := os.LookupEnv(key); exists && strings.TrimSpace(existingValue) != "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set env %s: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan .env file: %w", err)
	}
	return nil
}
