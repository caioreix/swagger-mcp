package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/app"
	"github.com/caioreix/swagger-mcp/internal/config"
)

// Execute builds the root Cobra command tree and runs it, returning an OS exit code.
func Execute(stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	code := 0
	root := newRootCmd(stdin, stdout, stderr, &code)

	// Explicit serve subcommand (same behaviour as root default).
	root.AddCommand(newServeCmd(stdin, stdout, stderr, &code))

	// generate subcommand group.
	generate := newGenerateCmd()
	generate.AddCommand(newGenerateServerCmd(stderr))
	generate.AddCommand(newGenerateToolCmd(stdout))
	generate.AddCommand(newGenerateModelCmd(stdout))
	root.AddCommand(generate)

	// inspect subcommand group.
	inspect := newInspectCmd()
	inspect.AddCommand(newInspectEndpointsCmd(stdout))
	inspect.AddCommand(newInspectEndpointCmd(stdout))
	root.AddCommand(inspect)

	// download subcommand.
	root.AddCommand(newDownloadCmd(stdout))

	// version subcommand.
	root.AddCommand(newVersionCmd(stderr))

	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return code
}

func newRootCmd(stdin io.Reader, stdout io.Writer, stderr io.Writer, code *int) *cobra.Command {
	var (
		swaggerURL     string
		transport      string
		port           string
		enableUI       bool
		proxyMode      bool
		baseURL        string
		headers        string
		includePaths   string
		excludePaths   string
		includeMethods string
		excludeMethods string
		sseHeaders     string
		httpHeaders    string
	)

	cmd := &cobra.Command{
		Use:   "swagger-mcp",
		Short: "MCP server that bridges AI assistants with Swagger/OpenAPI-described REST APIs",
		Long: `swagger-mcp is a Model Context Protocol (MCP) server that reads any
Swagger/OpenAPI definition and exposes it to AI clients as structured tools.

AI assistants connected via MCP can:
  • Discover and inspect API endpoints and their parameters
  • Explore data models and JSON schemas
  • Generate production-ready Go scaffolding (structs, tool handlers, full servers)
  • Proxy live API requests in real time — no code generation required

Transport modes:
  stdio           (default) communicates over stdin/stdout — ideal for Cursor, Claude Desktop
  sse             HTTP server with Server-Sent Events — good for web-based clients
  streamable-http HTTP server following the MCP StreamableHTTP spec

Authentication is configured via environment variables:
  API_KEY, API_KEY_HEADER, API_KEY_IN
  BEARER_TOKEN
  BASIC_AUTH_USER, BASIC_AUTH_PASS
  OAUTH2_TOKEN_URL, OAUTH2_CLIENT_ID, OAUTH2_CLIENT_SECRET, OAUTH2_SCOPES

Log level is controlled with LOG_LEVEL (debug | info | warn | error, default: info).
Settings can also be placed in a .env file in the working directory.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := buildConfig(
				swaggerURL, transport, port, baseURL, headers,
				includePaths, excludePaths, includeMethods, excludeMethods,
				sseHeaders, httpHeaders,
				enableUI, proxyMode,
			)
			if err != nil {
				return err
			}
			*code = app.Run(cfg, stdin, stdout, stderr)
			return nil
		},
	}

	// Route help/error output to stderr to keep stdout clean for JSON-RPC.
	cmd.SetOut(stderr)
	cmd.SetErr(stderr)

	f := cmd.Flags()

	f.StringVar(&swaggerURL, "swagger-url", "",
		"URL of the Swagger/OpenAPI definition to load and cache locally")
	f.StringVar(&transport, "transport", "stdio",
		`Transport protocol: "stdio" (default), "sse", or "streamable-http"`)
	f.StringVar(&port, "port", "8080",
		"HTTP port used by the sse and streamable-http transports")
	f.BoolVar(&enableUI, "ui", false,
		"Enable the built-in web UI for testing tools (available at http://localhost:<port>/)")
	f.BoolVar(&proxyMode, "proxy-mode", false,
		"Turn every Swagger endpoint into a live MCP tool — no code generation required")
	f.StringVar(&baseURL, "base-url", "",
		"Override the base URL extracted from the Swagger spec (useful for staging or local environments)")
	f.StringVar(&headers, "headers", "",
		"Static headers added to every proxy request — comma-separated key=value pairs (e.g. X-Tenant=acme,X-Source=mcp)")
	f.StringVar(&includePaths, "include-paths", "",
		"Regex patterns for API paths to expose as tools — comma-separated (e.g. ^/pets.*,^/users.*)")
	f.StringVar(&excludePaths, "exclude-paths", "",
		"Regex patterns for API paths to hide from tools — comma-separated")
	f.StringVar(&includeMethods, "include-methods", "",
		"HTTP methods to expose as tools — comma-separated (e.g. GET,POST)")
	f.StringVar(&excludeMethods, "exclude-methods", "",
		"HTTP methods to hide from tools — comma-separated (e.g. DELETE,PUT)")
	f.StringVar(&sseHeaders, "sse-headers", "",
		"Request headers forwarded from SSE clients to proxy API calls — comma-separated (e.g. Authorization,X-Tenant-ID)")
	f.StringVar(&httpHeaders, "http-headers", "",
		"Request headers forwarded from StreamableHTTP clients to proxy API calls — comma-separated")

	// Deprecated alias kept for backward compatibility.
	f.StringVar(&swaggerURL, "swaggerUrl", "", "Deprecated: use --swagger-url instead")
	_ = f.MarkHidden("swaggerUrl")
	_ = f.MarkDeprecated("swaggerUrl", "use --swagger-url instead")

	return cmd
}

func buildConfig(
	swaggerURL, transport, port, baseURL, headers,
	includePaths, excludePaths, includeMethods, excludeMethods,
	sseHeaders, httpHeaders string,
	enableUI, proxyMode bool,
) (config.Config, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return config.Config{}, fmt.Errorf("get working directory: %w", err)
	}

	if err := config.LoadDotEnv(filepath.Join(workingDir, ".env")); err != nil {
		return config.Config{}, err
	}

	logLevel := strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	if logLevel == "" {
		logLevel = "info"
	}

	return config.Config{
		SwaggerURL: strings.TrimSpace(swaggerURL),
		LogLevel:   logLevel,
		WorkingDir: workingDir,
		Auth:       config.LoadAuthConfig(),
		Filter: config.FilterConfig{
			IncludePaths:   strings.TrimSpace(includePaths),
			ExcludePaths:   strings.TrimSpace(excludePaths),
			IncludeMethods: strings.TrimSpace(includeMethods),
			ExcludeMethods: strings.TrimSpace(excludeMethods),
		},
		Transport:   strings.TrimSpace(transport),
		Port:        strings.TrimSpace(port),
		EnableUI:    enableUI,
		ProxyMode:   proxyMode,
		BaseURL:     strings.TrimSpace(baseURL),
		Headers:     strings.TrimSpace(headers),
		SseHeaders:  strings.TrimSpace(sseHeaders),
		HttpHeaders: strings.TrimSpace(httpHeaders),
	}, nil
}
