package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/codegen"
)

func newGenerateModelCmd(stdout io.Writer) *cobra.Command {
	var (
		swaggerURL  string
		swaggerFile string
		modelName   string
	)

	cmd := &cobra.Command{
		Use:   "model",
		Short: "Generate a Go struct for a schema model from an OpenAPI spec",
		Long: `Generate a Go struct with JSON tags for a named schema model from a
Swagger/OpenAPI definition.

The generated code is printed to stdout so it can be redirected or piped.

Example:
  swagger-mcp generate model \
    --swagger-url=https://petstore.swagger.io/v2/swagger.json \
    --model=Pet`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if modelName == "" {
				return fmt.Errorf("--model is required")
			}

			document, err := resolveDocument(swaggerURL, swaggerFile)
			if err != nil {
				return err
			}

			code, err := codegen.GenerateModelCode(document, modelName)
			if err != nil {
				return fmt.Errorf("generate model code: %w", err)
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
	f.StringVar(&modelName, "model", "",
		"Name of the schema model to generate (e.g. Pet, Order, User)")

	return cmd
}
