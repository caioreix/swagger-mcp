package openapi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readProjectMapping(workingDir string) (string, error) {
	mappingPath := filepath.Join(workingDir, ".swagger-mcp")
	content, err := os.ReadFile(mappingPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "SWAGGER_FILEPATH") {
			_, value, ok := strings.Cut(trimmed, "=")
			if !ok {
				continue
			}
			value = strings.TrimSpace(strings.Trim(value, "\"'"))
			if value == "" {
				continue
			}
			if filepath.IsAbs(value) {
				return value, nil
			}
			return filepath.Join(workingDir, value), nil
		}
	}
	return "", fmt.Errorf("SWAGGER_FILEPATH not found in .swagger-mcp")
}
