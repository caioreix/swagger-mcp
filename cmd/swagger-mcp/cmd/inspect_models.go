package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/openapi"
)

func newInspectModelsCmd(stdout io.Writer) *cobra.Command {
	var (
		swaggerURL  string
		swaggerFile string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "models",
		Short: "List all data models (schemas) defined in an OpenAPI definition",
		Long: `List all reusable data model schemas defined in a Swagger/OpenAPI
definition. These appear under "definitions" in Swagger 2.0 or
"components.schemas" in OpenAPI 3.x.

Use --format=json to include the full schema definition in the output.

Examples:
  swagger-mcp inspect models --swagger-url=https://petstore.swagger.io/v2/swagger.json
  swagger-mcp inspect models --swagger-file=./api.yaml --format=json`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			document, err := resolveDocument(swaggerURL, swaggerFile)
			if err != nil {
				return err
			}

			schemas := openapi.ListSchemas(document)

			switch strings.ToLower(format) {
			case formatJSON:
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(schemas)
			default:
				rows := make([][]string, 0, len(schemas))
				for _, s := range schemas {
					schemaType := s.Type
					if schemaType == "" {
						schemaType = "object"
					}
					rows = append(rows, []string{s.Name, schemaType, strconv.Itoa(s.Properties)})
				}
				printTable(stdout, []string{"NAME", "TYPE", "PROPERTIES"}, rows)
				fmt.Fprintf(stdout, "\n%d model(s)\n", len(schemas))
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&swaggerURL, "swagger-url", "",
		"URL of the Swagger/OpenAPI definition")
	f.StringVar(&swaggerFile, "swagger-file", "",
		"Path to a local Swagger/OpenAPI file (alternative to --swagger-url)")
	f.StringVar(&format, "format", "table",
		`Output format: "table" (default) or "json"`)

	return cmd
}
