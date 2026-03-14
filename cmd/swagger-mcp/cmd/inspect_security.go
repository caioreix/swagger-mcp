package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/openapi"
)

func newInspectSecurityCmd(stdout io.Writer) *cobra.Command {
	var (
		swaggerURL  string
		swaggerFile string
		format      string
	)

	cmd := &cobra.Command{
		Use:   "security",
		Short: "List all security schemes defined in an OpenAPI definition",
		Long: `List all authentication and authorization schemes defined in a
Swagger/OpenAPI definition, including API key, HTTP (Bearer/Basic), and OAuth2 schemes.

Use --format=json to get machine-readable output including OAuth2 scopes and token URLs.

Examples:
  swagger-mcp inspect security --swagger-url=https://petstore.swagger.io/v2/swagger.json
  swagger-mcp inspect security --swagger-file=./api.yaml --format=json`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			document, err := resolveDocument(swaggerURL, swaggerFile)
			if err != nil {
				return err
			}

			schemes := openapi.ExtractSecuritySchemes(document)

			switch strings.ToLower(format) {
			case "json":
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(schemes)
			default:
				rows := make([][]string, 0, len(schemes))
				for _, s := range schemes {
					detail := s.Scheme
					if detail == "" {
						detail = s.In
					}
					if detail == "" && s.FlowType != "" {
						detail = s.FlowType
					}
					param := s.ParamName
					if param == "" {
						param = s.BearerFmt
					}
					rows = append(rows, []string{s.Name, s.Type, detail, param})
				}
				printTable(stdout, []string{"NAME", "TYPE", "SCHEME/IN", "PARAM/FORMAT"}, rows)
				fmt.Fprintf(stdout, "\n%d scheme(s)\n", len(schemes))
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
