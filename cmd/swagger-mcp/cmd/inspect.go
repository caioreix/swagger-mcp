package cmd

import (
	"github.com/spf13/cobra"
)

func newInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect and explore an OpenAPI definition",
		Long: `Inspect a Swagger/OpenAPI definition and display its structure.

Subcommands:
  endpoints   List all API endpoints with methods and summaries
  endpoint    Show details and models for a specific endpoint`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	return cmd
}
