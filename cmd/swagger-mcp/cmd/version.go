package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/config"
)

func newVersionCmd(out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the swagger-mcp version and exit",
		Long:  "Print the current version of the swagger-mcp server and exit.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(out, "swagger-mcp %s\n", config.Version)
		},
	}
}
