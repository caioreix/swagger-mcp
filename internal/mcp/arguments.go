package mcp

import (
	"fmt"
	"strings"
)

func requiredString(arguments map[string]any, key string) (string, error) {
	value := optionalString(arguments, key)
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func optionalString(arguments map[string]any, key string) string {
	if rawValue, ok := arguments[key]; ok {
		switch value := rawValue.(type) {
		case string:
			return value
		default:
			return fmt.Sprint(value)
		}
	}
	return ""
}

func optionalBool(arguments map[string]any, key string, fallback bool) bool {
	rawValue, ok := arguments[key]
	if !ok {
		return fallback
	}
	switch value := rawValue.(type) {
	case bool:
		return value
	case string:
		lower := strings.ToLower(strings.TrimSpace(value))
		if lower == "true" {
			return true
		}
		if lower == "false" {
			return false
		}
	}
	return fallback
}
