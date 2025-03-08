package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/openapi"
)

// resolveDocument loads an OpenAPI document from a URL or local file path.
// It loads the .env file from the working directory before resolving.
func resolveDocument(swaggerURL, swaggerFile string) (map[string]any, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	if err := config.LoadDotEnv(filepath.Join(workingDir, ".env")); err != nil {
		return nil, err
	}

	resolver := openapi.NewSourceResolver(workingDir, swaggerURL)

	if swaggerFile != "" {
		return resolver.Load(swaggerFile)
	}
	return resolver.Load("")
}

// printTable prints a simple aligned two-dimensional table to out.
func printTable(out io.Writer, headers []string, rows [][]string) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	for i, h := range headers {
		if i < len(widths)-1 {
			fmt.Fprintf(out, "%-*s  ", widths[i], h)
		} else {
			fmt.Fprint(out, h)
		}
	}
	fmt.Fprintln(out)

	for i, w := range widths {
		sep := make([]byte, w)
		for j := range sep {
			sep[j] = '-'
		}
		if i < len(widths)-1 {
			fmt.Fprintf(out, "%s  ", sep)
		} else {
			fmt.Fprintf(out, "%s", sep)
		}
	}
	fmt.Fprintln(out)

	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) {
				if i < len(widths)-1 {
					fmt.Fprintf(out, "%-*s  ", widths[i], cell)
				} else {
					fmt.Fprint(out, cell)
				}
			}
		}
		fmt.Fprintln(out)
	}
}
