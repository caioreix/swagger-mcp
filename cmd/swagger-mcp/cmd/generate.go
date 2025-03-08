package cmd

import (
	"github.com/spf13/cobra"
)

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate Go code from an OpenAPI definition",
		Long: `Generate Go code artifacts directly from a Swagger/OpenAPI definition.

Subcommands:
  server   Generate a complete, runnable MCP server project
  tool     Generate a single MCP tool scaffold for one endpoint
  model    Generate a Go struct for a schema model`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	return cmd
}
