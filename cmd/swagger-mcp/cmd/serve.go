package cmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/app"
)

func newServeCmd(stdin io.Reader, stdout io.Writer, stderr io.Writer, code *int) *cobra.Command {
	opts := &serveOptions{}

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server (default behavior when no command is given)",
		Long: `Start the swagger-mcp MCP server and wait for JSON-RPC 2.0 requests.

This is the default command — running swagger-mcp without a subcommand is
identical to running swagger-mcp serve.

The server exposes tools for endpoint discovery, model inspection, and live API
proxying over the configured transport protocol.

All flags can be set via environment variables using the SWAGGER_MCP_ prefix
(e.g. SWAGGER_MCP_TRANSPORT=streamable-http, SWAGGER_MCP_PORT=9090). Flags always take
precedence over environment variables.`,
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

	cmd.SetOut(stderr)
	cmd.SetErr(stderr)

	opts.addFlags(cmd)
	return cmd
}
