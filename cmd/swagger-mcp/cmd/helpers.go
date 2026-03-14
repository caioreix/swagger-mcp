package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/openapi"
)

// serveOptions groups all CLI flags shared by the root and serve commands.
type serveOptions struct {
	swaggerURL     string
	transport      string
	port           string
	logLevel       string
	baseURL        string
	headers        string
	includePaths   string
	excludePaths   string
	includeMethods string
	excludeMethods string
	sseHeaders     string
	httpHeaders    string
	enableUI       bool
	proxyMode      bool
}

// addFlags registers all serve-related flags on cmd.
// Each flag documents its environment variable equivalent.
func (o *serveOptions) addFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&o.swaggerURL, "swagger-url", "",
		"URL of the Swagger/OpenAPI definition to load and cache locally (env: SWAGGER_MCP_SWAGGER_URL)")
	f.StringVar(&o.transport, "transport", "",
		`Transport protocol: "stdio" (default), "sse", or "streamable-http" (env: SWAGGER_MCP_TRANSPORT)`)
	f.StringVar(&o.port, "port", "",
		"HTTP port used by the sse and streamable-http transports (default: 8080, env: SWAGGER_MCP_PORT)")
	f.StringVar(&o.logLevel, "log-level", "",
		`Log verbosity level: "debug", "info", "warn", or "error" (default: info, env: LOG_LEVEL)`)
	f.BoolVar(&o.enableUI, "ui", false,
		"Enable the built-in web UI at http://localhost:<port>/ (env: SWAGGER_MCP_UI)")
	f.BoolVar(&o.proxyMode, "proxy-mode", false,
		"Turn every Swagger endpoint into a live MCP tool — no code generation required (env: SWAGGER_MCP_PROXY_MODE)")
	f.StringVar(&o.baseURL, "base-url", "",
		"Override the base URL from the Swagger spec — useful for staging or local environments (env: SWAGGER_MCP_BASE_URL)")
	f.StringVar(&o.headers, "headers", "",
		"Static headers added to every proxy request — comma-separated key=value pairs, e.g. X-Tenant=acme (env: SWAGGER_MCP_HEADERS)")
	f.StringVar(&o.includePaths, "include-paths", "",
		"Regex patterns for API paths to expose as tools — comma-separated, e.g. ^/pets.*,^/users.* (env: SWAGGER_MCP_INCLUDE_PATHS)")
	f.StringVar(&o.excludePaths, "exclude-paths", "",
		"Regex patterns for API paths to hide from tools — comma-separated (env: SWAGGER_MCP_EXCLUDE_PATHS)")
	f.StringVar(&o.includeMethods, "include-methods", "",
		"HTTP methods to expose as tools — comma-separated, e.g. GET,POST (env: SWAGGER_MCP_INCLUDE_METHODS)")
	f.StringVar(&o.excludeMethods, "exclude-methods", "",
		"HTTP methods to hide from tools — comma-separated, e.g. DELETE,PUT (env: SWAGGER_MCP_EXCLUDE_METHODS)")
	f.StringVar(&o.sseHeaders, "sse-headers", "",
		"Request headers forwarded from SSE clients to proxy API calls — comma-separated (env: SWAGGER_MCP_SSE_HEADERS)")
	f.StringVar(&o.httpHeaders, "http-headers", "",
		"Request headers forwarded from StreamableHTTP clients to proxy API calls — comma-separated (env: SWAGGER_MCP_HTTP_HEADERS)")

	// Deprecated alias kept for backward compatibility.
	f.StringVar(&o.swaggerURL, "swaggerUrl", "", "Deprecated: use --swagger-url instead")
	_ = f.MarkHidden("swaggerUrl")
	_ = f.MarkDeprecated("swaggerUrl", "use --swagger-url instead")
}

// toConfig builds a config.Config from parsed flags and environment variables.
// Flags explicitly set by the user always take precedence over environment variables.
func (o *serveOptions) toConfig(cmd *cobra.Command) (config.Config, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return config.Config{}, fmt.Errorf("get working directory: %w", err)
	}
	if err := config.LoadDotEnv(filepath.Join(workingDir, ".env")); err != nil {
		return config.Config{}, err
	}

	transport := envOr(cmd, "transport", o.transport, "SWAGGER_MCP_TRANSPORT")
	if transport == "" {
		transport = "stdio"
	}
	port := envOr(cmd, "port", o.port, "SWAGGER_MCP_PORT")
	if port == "" {
		port = "8080"
	}
	logLevel := envOr(cmd, "log-level", o.logLevel, "LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	return config.Config{
		SwaggerURL: envOr(cmd, "swagger-url", o.swaggerURL, "SWAGGER_MCP_SWAGGER_URL"),
		LogLevel:   logLevel,
		WorkingDir: workingDir,
		Auth:       config.LoadAuthConfig(),
		Filter: config.FilterConfig{
			IncludePaths:   envOr(cmd, "include-paths", o.includePaths, "SWAGGER_MCP_INCLUDE_PATHS"),
			ExcludePaths:   envOr(cmd, "exclude-paths", o.excludePaths, "SWAGGER_MCP_EXCLUDE_PATHS"),
			IncludeMethods: envOr(cmd, "include-methods", o.includeMethods, "SWAGGER_MCP_INCLUDE_METHODS"),
			ExcludeMethods: envOr(cmd, "exclude-methods", o.excludeMethods, "SWAGGER_MCP_EXCLUDE_METHODS"),
		},
		Transport:   transport,
		Port:        port,
		EnableUI:    envOrBool(cmd, "ui", o.enableUI, "SWAGGER_MCP_UI"),
		ProxyMode:   envOrBool(cmd, "proxy-mode", o.proxyMode, "SWAGGER_MCP_PROXY_MODE"),
		BaseURL:     envOr(cmd, "base-url", o.baseURL, "SWAGGER_MCP_BASE_URL"),
		Headers:     envOr(cmd, "headers", o.headers, "SWAGGER_MCP_HEADERS"),
		SseHeaders:  envOr(cmd, "sse-headers", o.sseHeaders, "SWAGGER_MCP_SSE_HEADERS"),
		HttpHeaders: envOr(cmd, "http-headers", o.httpHeaders, "SWAGGER_MCP_HTTP_HEADERS"),
	}, nil
}

// envOr returns the flag value when the flag was explicitly set by the user,
// otherwise falls back to envKey, then to flagValue (the flag default).
func envOr(cmd *cobra.Command, flagName, flagValue, envKey string) string {
	if cmd.Flags().Changed(flagName) {
		return strings.TrimSpace(flagValue)
	}
	if v := os.Getenv(envKey); v != "" {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(flagValue)
}

// envOrBool returns the flag value when explicitly set by the user, otherwise
// checks the environment variable (treats "true", "1", or "yes" as true).
func envOrBool(cmd *cobra.Command, flagName string, flagValue bool, envKey string) bool {
	if cmd.Flags().Changed(flagName) {
		return flagValue
	}
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
	}
	return flagValue
}

// resolveDocument loads an OpenAPI document from a URL or local file path.
func resolveDocument(swaggerURL, swaggerFile string) (map[string]any, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	if err := config.LoadDotEnv(filepath.Join(workingDir, ".env")); err != nil {
		return nil, err
	}
	resolver := openapi.NewSourceResolver(workingDir, swaggerURL)
	if swaggerFile != "" {
		return resolver.Load(swaggerFile)
	}
	return resolver.Load("")
}

// printTable prints a simple aligned two-dimensional table to out.
func printTable(out io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	for i, h := range headers {
		if i < len(widths)-1 {
			fmt.Fprintf(out, "%-*s  ", widths[i], h)
		} else {
			fmt.Fprint(out, h)
		}
	}
	fmt.Fprintln(out)
	for i, w := range widths {
		sep := make([]byte, w)
		for j := range sep {
			sep[j] = '-'
		}
		if i < len(widths)-1 {
			fmt.Fprintf(out, "%s  ", sep)
		} else {
			fmt.Fprintf(out, "%s", sep)
		}
	}
	fmt.Fprintln(out)
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				if i < len(widths)-1 {
					fmt.Fprintf(out, "%-*s  ", widths[i], cell)
				} else {
					fmt.Fprint(out, cell)
				}
			}
		}
		fmt.Fprintln(out)
	}
}
