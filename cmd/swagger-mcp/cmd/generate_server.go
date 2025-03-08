package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/caioreix/swagger-mcp/internal/codegen"
)

func newGenerateServerCmd(stderr io.Writer) *cobra.Command {
	var (
		swaggerURL string
		swaggerFile string
		moduleName  string
		transports  string
		proxyMode   bool
		endpoints   string
		outputDir   string
	)

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Generate a complete, runnable MCP server project from an OpenAPI spec",
		Long: `Generate a fully functional Go MCP server project from a Swagger/OpenAPI definition.

The generated project includes:
  go.mod        Go module file
  main.go       Entry point with transport setup
  server.go     MCP server and request routing
  tools.go      MCP tool definitions
  handlers.go   Request handlers (proxy or stub)
  helpers.go    Auth and HTTP utilities

Example:
  swagger-mcp generate server \
    --swagger-url=https://petstore.swagger.io/v2/swagger.json \
    --module=github.com/acme/petstore-mcp \
    --output=./petstore-server`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			document, err := resolveDocument(swaggerURL, swaggerFile)
			if err != nil {
				return err
			}

			var transportModes []string
			if transports != "" {
				for _, t := range strings.Split(transports, ",") {
					t = strings.TrimSpace(t)
					if t != "" {
						transportModes = append(transportModes, t)
					}
				}
			}

			var endpointList []string
			if endpoints != "" {
				for _, e := range strings.Split(endpoints, ",") {
					e = strings.TrimSpace(e)
					if e != "" {
						endpointList = append(endpointList, e)
					}
				}
			}

			files, err := codegen.GenerateCompleteServer(document, codegen.ServerGenParams{
				ModuleName:     strings.TrimSpace(moduleName),
				TransportModes: transportModes,
				ProxyMode:      proxyMode,
				Endpoints:      endpointList,
			})
			if err != nil {
				return fmt.Errorf("generate server: %w", err)
			}

			if outputDir == "" {
				outputDir = "generated-server"
			}
			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("create output directory %s: %w", outputDir, err)
			}

			for name, content := range files {
				path := filepath.Join(outputDir, name)
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					return fmt.Errorf("write %s: %w", name, err)
				}
			}

			fmt.Fprintf(stderr, "Generated %d files in %s:\n", len(files), outputDir)
			for name := range files {
				fmt.Fprintf(stderr, "  %s\n", filepath.Join(outputDir, name))
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&swaggerURL, "swagger-url", "",
		"URL of the Swagger/OpenAPI definition")
	f.StringVar(&swaggerFile, "swagger-file", "",
		"Path to a local Swagger/OpenAPI file (alternative to --swagger-url)")
	f.StringVar(&moduleName, "module", "github.com/generated/mcp-server",
		"Go module name for the generated project (e.g. github.com/acme/my-mcp-server)")
	f.StringVar(&transports, "transport", "stdio",
		"Comma-separated transport modes to include: stdio, sse, streamable-http")
	f.BoolVar(&proxyMode, "proxy", true,
		"Generate proxy handlers that forward requests to the real API (false = generate stub handlers)")
	f.StringVar(&endpoints, "endpoints", "",
		"Comma-separated endpoint paths to include (empty = all, e.g. /pets,/pets/{id})")
	f.StringVar(&outputDir, "output", "generated-server",
		"Directory to write the generated project files")

	return cmd
}
