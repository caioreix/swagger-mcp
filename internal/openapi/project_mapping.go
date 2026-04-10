package openapi

import (
	"errors"
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
	for line := range strings.SplitSeq(string(content), "\n") {
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
	return "", errors.New("SWAGGER_FILEPATH not found in .swagger-mcp")
}
