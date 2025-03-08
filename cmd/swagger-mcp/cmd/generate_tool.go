package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/codegen"
)

func newGenerateToolCmd(stdout io.Writer) *cobra.Command {
	var (
		swaggerURL       string
		swaggerFile      string
		endpointPath     string
		method           string
		includeAPI       bool
		includeVersion   bool
		singularize      bool
	)

	cmd := &cobra.Command{
		Use:   "tool",
		Short: "Generate a Go MCP tool scaffold for one endpoint",
		Long: `Generate a Go MCP tool scaffold for a specific Swagger/OpenAPI endpoint.

The output includes:
  - MCP tool metadata (name, description, input schema)
  - A handler stub ready for application-specific logic

The generated code is printed to stdout so it can be redirected or piped.

Example:
  swagger-mcp generate tool \
    --swagger-url=https://petstore.swagger.io/v2/swagger.json \
    --path=/pet/{petId} \
    --method=GET`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if endpointPath == "" {
				return fmt.Errorf("--path is required")
			}
			if method == "" {
				return fmt.Errorf("--method is required")
			}

			document, err := resolveDocument(swaggerURL, swaggerFile)
			if err != nil {
				return err
			}

			code, err := codegen.GenerateEndpointToolCode(document, codegen.GenerateEndpointToolCodeParams{
				Path:                    endpointPath,
				Method:                  method,
				IncludeAPIInName:        includeAPI,
				IncludeVersionInName:    includeVersion,
				SingularizeResourceName: singularize,
			})
			if err != nil {
				return fmt.Errorf("generate tool code: %w", err)
			}

			fmt.Fprint(stdout, code)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&swaggerURL, "swagger-url", "",
		"URL of the Swagger/OpenAPI definition")
	f.StringVar(&swaggerFile, "swagger-file", "",
		"Path to a local Swagger/OpenAPI file (alternative to --swagger-url)")
	f.StringVar(&endpointPath, "path", "",
		"API endpoint path (e.g. /pet/{petId})")
	f.StringVar(&method, "method", "",
		"HTTP method (e.g. GET, POST, PUT, DELETE)")
	f.BoolVar(&includeAPI, "include-api", false,
		"Include 'api' path segments in the generated tool name")
	f.BoolVar(&includeVersion, "include-version", false,
		"Include version segments (e.g. v1, v2) in the generated tool name")
	f.BoolVar(&singularize, "singularize", true,
		"Singularize resource names in the generated tool name (e.g. pets → pet)")

	return cmd
}
