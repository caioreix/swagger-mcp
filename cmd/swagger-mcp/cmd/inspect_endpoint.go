package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/openapi"
)

func newInspectEndpointCmd(stdout io.Writer) *cobra.Command {
	var (
		swaggerURL  string
		swaggerFile string
		path        string
		method      string
	)

	cmd := &cobra.Command{
		Use:   "endpoint",
		Short: "Show details and models for a specific API endpoint",
		Long: `Show the full details of a specific endpoint including its summary,
description, operation ID, tags, and all referenced schema models.

Example:
  swagger-mcp inspect endpoint \
    --swagger-url=https://petstore.swagger.io/v2/swagger.json \
    --path=/pet/{petId} \
    --method=GET`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				return fmt.Errorf("--path is required")
			}
			if method == "" {
				return fmt.Errorf("--method is required")
			}

			document, err := resolveDocument(swaggerURL, swaggerFile)
			if err != nil {
				return err
			}

			endpoints, err := openapi.ListEndpoints(document)
			if err != nil {
				return fmt.Errorf("list endpoints: %w", err)
			}

			var found *openapi.Endpoint
			for i, ep := range endpoints {
				if ep.Path == path && strings.EqualFold(ep.Method, method) {
					found = &endpoints[i]
					break
				}
			}
			if found == nil {
				return fmt.Errorf("endpoint %s %s not found in the spec", strings.ToUpper(method), path)
			}

			models, err := openapi.ListEndpointModels(document, path, method)
			if err != nil {
				return fmt.Errorf("list endpoint models: %w", err)
			}

			result := map[string]any{
				"path":        found.Path,
				"method":      found.Method,
				"summary":     found.Summary,
				"description": found.Description,
				"operationId": found.OperationID,
				"tags":        found.Tags,
				"models":      models,
			}

			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}

	f := cmd.Flags()
	f.StringVar(&swaggerURL, "swagger-url", "",
		"URL of the Swagger/OpenAPI definition")
	f.StringVar(&swaggerFile, "swagger-file", "",
		"Path to a local Swagger/OpenAPI file (alternative to --swagger-url)")
	f.StringVar(&path, "path", "",
		"API endpoint path (e.g. /pet/{petId})")
	f.StringVar(&method, "method", "",
		"HTTP method (e.g. GET, POST, PUT, DELETE)")

	return cmd
}
