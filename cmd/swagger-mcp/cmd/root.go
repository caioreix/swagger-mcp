package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/app"
)

// Execute builds the root Cobra command tree and runs it, returning an OS exit code.
// args should be os.Args[1:] in production; tests can pass custom args.
func Execute(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	code := 0
	root := newRootCmd(stdin, stdout, stderr, &code)

	// Explicit serve subcommand (same behaviour as root default).
	root.AddCommand(newServeCmd(stdin, stdout, stderr, &code))

	// inspect subcommand group.
	inspect := newInspectCmd()
	inspect.AddCommand(newInspectEndpointsCmd(stdout))
	inspect.AddCommand(newInspectEndpointCmd(stdout))
	inspect.AddCommand(newInspectSecurityCmd(stdout))
	inspect.AddCommand(newInspectModelsCmd(stdout))
	root.AddCommand(inspect)

	// download subcommand.
	root.AddCommand(newDownloadCmd(stdout))

	// version subcommand.
	root.AddCommand(newVersionCmd(stderr))

	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return code
}

func newRootCmd(stdin io.Reader, stdout io.Writer, stderr io.Writer, code *int) *cobra.Command {
	opts := &serveOptions{}

	cmd := &cobra.Command{
		Use:   "swagger-mcp",
		Short: "MCP server that bridges AI assistants with Swagger/OpenAPI-described REST APIs",
		Long: `swagger-mcp is a Model Context Protocol (MCP) server that reads any
Swagger/OpenAPI definition and exposes it to AI clients as structured tools.

AI assistants connected via MCP can:
  • Discover and inspect API endpoints and their parameters
  • Explore data models and JSON schemas
  • Proxy live API requests in real time

Transport modes:
  stdio           (default) communicates over stdin/stdout — ideal for Cursor, Claude Desktop
  sse             HTTP server with Server-Sent Events — good for web-based clients
  streamable-http HTTP server following the MCP StreamableHTTP spec

All flags can be set via environment variables using the SWAGGER_MCP_ prefix
(e.g. SWAGGER_MCP_TRANSPORT=sse, SWAGGER_MCP_PORT=9090). Flags always take
precedence over environment variables.

Authentication is configured via environment variables:
  API_KEY, API_KEY_HEADER, API_KEY_IN
  BEARER_TOKEN
  BASIC_AUTH_USER, BASIC_AUTH_PASS
  OAUTH2_TOKEN_URL, OAUTH2_CLIENT_ID, OAUTH2_CLIENT_SECRET, OAUTH2_SCOPES

Settings can also be placed in a .env file in the working directory.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := opts.toConfig(cmd)
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

	opts.addFlags(cmd)
	return cmd
}
