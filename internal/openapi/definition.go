package openapi

import (
	"fmt"
	"os"
	"path/filepath"
)

func ReadDefinitionFromFile(swaggerFilePath string) (map[string]any, error) {
	content, err := os.ReadFile(swaggerFilePath)
	if err != nil {
		return nil, fmt.Errorf("read swagger file %s: %w", swaggerFilePath, err)
	}
	document, _, err := parseDefinitionBytes(content, filepath.Ext(swaggerFilePath))
	if err != nil {
		return nil, err
	}
	return document, nil
}
