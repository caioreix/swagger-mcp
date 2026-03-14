package openapi

import "github.com/caioreix/swagger-mcp/internal/format"

func parseYAML(data []byte) (any, error) {
	return format.ParseYAML(data)
}
