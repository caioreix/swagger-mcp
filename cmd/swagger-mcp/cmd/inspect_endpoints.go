package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/openapi"
)

const formatJSON = "json"

func newInspectEndpointsCmd(stdout io.Writer) *cobra.Command {
	var (
		swaggerURL     string
		swaggerFile    string
		includePaths   string
		excludePaths   string
		includeMethods string
		excludeMethods string
		format         string
	)

	cmd := &cobra.Command{
		Use:   "endpoints",
		Short: "List all API endpoints from an OpenAPI definition",
		Long: `List all endpoints defined in a Swagger/OpenAPI definition, with their
HTTP methods and summaries.

Use --format=json to get machine-readable output suitable for piping.
Use filtering flags to narrow down the output.

Examples:
  swagger-mcp inspect endpoints --swagger-url=https://petstore.swagger.io/v2/swagger.json
  swagger-mcp inspect endpoints --swagger-url=... --include-paths='^/pet.*' --include-methods=GET
  swagger-mcp inspect endpoints --swagger-url=... --format=json`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			document, err := resolveDocument(swaggerURL, swaggerFile)
			if err != nil {
				return err
			}

			endpoints, err := openapi.ListEndpoints(document)
			if err != nil {
				return fmt.Errorf("list endpoints: %w", err)
			}

			if includePaths != "" || excludePaths != "" || includeMethods != "" || excludeMethods != "" {
				filter, filterErr := openapi.NewEndpointFilter(
					includePaths, excludePaths, includeMethods, excludeMethods,
				)
				if filterErr != nil {
					return fmt.Errorf("build filter: %w", filterErr)
				}
				endpoints = openapi.FilterEndpoints(endpoints, filter)
			}

			switch strings.ToLower(format) {
			case formatJSON:
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(endpoints)
			default:
				rows := make([][]string, 0, len(endpoints))
				for _, ep := range endpoints {
					summary := ep.Summary
					if summary == "" {
						summary = ep.Description
					}
					rows = append(rows, []string{ep.Method, ep.Path, summary})
				}
				printTable(stdout, []string{"METHOD", "PATH", "SUMMARY"}, rows)
				fmt.Fprintf(stdout, "\n%d endpoint(s)\n", len(endpoints))
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&swaggerURL, "swagger-url", "",
		"URL of the Swagger/OpenAPI definition")
	f.StringVar(&swaggerFile, "swagger-file", "",
		"Path to a local Swagger/OpenAPI file (alternative to --swagger-url)")
	f.StringVar(&includePaths, "include-paths", "",
		"Regex patterns for paths to include — comma-separated")
	f.StringVar(&excludePaths, "exclude-paths", "",
		"Regex patterns for paths to exclude — comma-separated")
	f.StringVar(&includeMethods, "include-methods", "",
		"HTTP methods to include — comma-separated (e.g. GET,POST)")
	f.StringVar(&excludeMethods, "exclude-methods", "",
		"HTTP methods to exclude — comma-separated (e.g. DELETE)")
	f.StringVar(&format, "format", "table",
		`Output format: "table" (default) or "json"`)

	return cmd
}
