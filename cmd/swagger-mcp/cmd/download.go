package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/openapi"
)

func newDownloadCmd(stdout io.Writer) *cobra.Command {
	var (
		url    string
		output string
	)

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download and cache a Swagger/OpenAPI definition locally",
		Long: `Download a Swagger/OpenAPI definition from a URL and save it to a local
directory. The file is cached with hash-based validation so subsequent runs
only re-download when the spec changes.

The saved file path is printed to stdout on success, making it easy to use
in scripts or to populate a .swagger-mcp mapping file.

Example:
  swagger-mcp download \
    --url=https://petstore.swagger.io/v2/swagger.json \
    --output=./swagger-cache

  # Populate .swagger-mcp automatically:
  echo "SWAGGER_FILEPATH=$(swagger-mcp download --url=... --output=.)" > .swagger-mcp`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if strings.TrimSpace(url) == "" {
				return errors.New("--url is required")
			}

			workingDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			if loadErr := config.LoadDotEnv(filepath.Join(workingDir, ".env")); loadErr != nil {
				return loadErr
			}

			if output == "" {
				output = "swagger-cache"
			}
			if !filepath.IsAbs(output) {
				output = filepath.Join(workingDir, output)
			}

			resolver := openapi.NewSourceResolver(workingDir, "")
			saved, err := resolver.DownloadDefinition(context.Background(), url, output)
			if err != nil {
				return fmt.Errorf("download definition: %w", err)
			}

			fmt.Fprintln(stdout, saved.FilePath)
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&url, "url", "",
		"URL of the Swagger/OpenAPI definition to download (required)")
	f.StringVar(&output, "output", "swagger-cache",
		"Directory to save the downloaded definition (default: swagger-cache)")

	return cmd
}
