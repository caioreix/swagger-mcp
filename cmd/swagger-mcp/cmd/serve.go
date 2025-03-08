package cmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/app"
)

func newServeCmd(stdin io.Reader, stdout io.Writer, stderr io.Writer, code *int) *cobra.Command {
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
		Use:   "serve",
		Short: "Start the MCP server (default behavior when no command is given)",
		Long: `Start the swagger-mcp MCP server and wait for JSON-RPC 2.0 requests.

This is the default command — running swagger-mcp without a subcommand is
identical to running swagger-mcp serve.

The server exposes tools for endpoint discovery, model inspection, Go code
generation, and live API proxying over the configured transport protocol.`,
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

	f.StringVar(&swaggerURL, "swaggerUrl", "", "Deprecated: use --swagger-url instead")
	_ = f.MarkHidden("swaggerUrl")
	_ = f.MarkDeprecated("swaggerUrl", "use --swagger-url instead")

	return cmd
}
