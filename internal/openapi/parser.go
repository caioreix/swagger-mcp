package openapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func parseDefinitionBytes(data []byte, pathHint string) (map[string]any, string, error) {
	if document, kind, err := parseJSONDefinition(data); err == nil {
		return document, kind, nil
	}

	parsedYAML, err := parseYAML(data)
	if err != nil {
		if strings.TrimSpace(pathHint) != "" {
			return nil, "", fmt.Errorf("content for %q is neither valid JSON nor supported YAML: %w", pathHint, err)
		}
		return nil, "", fmt.Errorf("content is neither valid JSON nor supported YAML: %w", err)
	}
	document, ok := parsedYAML.(map[string]any)
	if !ok {
		return nil, "", errors.New("parsed YAML root is not an object")
	}

	kind, err := validateDefinition(document)
	if err != nil {
		return nil, "", err
	}
	return document, kind, nil
}

func parseJSONDefinition(data []byte) (map[string]any, string, error) {
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, "", err
	}
	kind, err := validateDefinition(document)
	if err != nil {
		return nil, "", err
	}
	return document, kind, nil
}

func validateDefinition(document map[string]any) (string, error) {
	if _, ok := document["openapi"]; ok {
		return "openapi", nil
	}
	if _, ok := document["swagger"]; ok {
		return "swagger", nil
	}
	return "", errors.New("invalid Swagger definition: missing required \"openapi\" or \"swagger\" field")
}
