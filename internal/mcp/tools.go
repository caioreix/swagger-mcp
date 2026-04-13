package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/openapi"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpgoserver "github.com/mark3labs/mcp-go/server"
)

const (
	// characterLimit is the maximum response size in bytes before truncation.
	characterLimit = 25000

	defaultEndpointLimit = 50
	maxEndpointLimit     = 200
)

// endpointListResult is the paginated response returned by swagger_list_endpoints.
type endpointListResult struct {
	Total      int                `json:"total"`
	Count      int                `json:"count"`
	Offset     int                `json:"offset"`
	Endpoints  []openapi.Endpoint `json:"endpoints"`
	HasMore    bool               `json:"has_more"`
	NextOffset *int               `json:"next_offset,omitempty"`
}

type definitionDownloadResult struct {
	FilePath      string `json:"file_path"`
	ConfigFile    string `json:"config_file"`
	ConfigSnippet string `json:"config_snippet"`
}

type modelListResult struct {
	Path   string          `json:"path"`
	Method string          `json:"method"`
	Count  int             `json:"count"`
	Models []openapi.Model `json:"models"`
}

type versionToolResult struct {
	Version string `json:"version"`
}

func registerStaticTools( //nolint:gocognit,funlen
	s *mcpgoserver.MCPServer,
	resolver openapi.SourceResolver,
	filter *openapi.EndpointFilter,
	cfg config.Config,
) {
	swaggerPathDescription := "Optional path to the Swagger file. Used only if --swagger-url is not provided. You can find this path in the .swagger-mcp file in the solution root with the format SWAGGER_FILEPATH=path/to/file.json."
	responseFormatDescription := "Output format: 'markdown' for human-readable or 'json' for machine-readable (default: markdown)"

	// swagger_get_definition
	s.AddTool(mcpgo.NewTool(
		"swagger_get_definition",
		mcpgo.WithDescription(`Downloads a Swagger/OpenAPI definition from a URL and saves it locally.

IMPORTANT: After calling this tool, you MUST create a file named '.swagger-mcp' in the
solution root with the content: SWAGGER_FILEPATH=<returned filePath>. This file is
required by all other swagger tools.

Args:
  - url (string, required): The URL of the Swagger/OpenAPI definition
  - saveLocation (string, required): Directory where the definition will be saved (solution root)

Returns: Success message with the saved file path to write into .swagger-mcp

Error Handling:
  - Returns error if the URL is unreachable or the definition cannot be parsed
  - Returns error if saveLocation is empty or cannot be written to`),
		mcpgo.WithString("url", mcpgo.Required(), mcpgo.Description("The URL of the Swagger/OpenAPI definition")),
		mcpgo.WithString(
			"saveLocation",
			mcpgo.Required(),
			mcpgo.Description("Directory where the definition will be saved (should be the solution root folder)"),
		),
		mcpgo.WithOutputSchema[definitionDownloadResult](),
		mcpgo.WithToolAnnotation(mcpgo.ToolAnnotation{
			Title:           "Download Swagger Definition",
			ReadOnlyHint:    new(false),
			DestructiveHint: new(false),
			IdempotentHint:  new(true),
			OpenWorldHint:   new(true),
		}),
	), func(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		urlVal, err := req.RequireString("url")
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		saveLocation, err := req.RequireString("saveLocation")
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		saveLocation = strings.TrimSpace(saveLocation)
		if saveLocation == "" {
			return mcpgo.NewToolResultError("saveLocation cannot be empty or contain only whitespace"), nil
		}
		if !filepath.IsAbs(saveLocation) {
			saveLocation = filepath.Join(cfg.WorkingDir, saveLocation)
		}
		savedDefinition, err := resolver.DownloadDefinition(urlVal, saveLocation)
		if err != nil {
			return mcpgo.NewToolResultError(
				fmt.Sprintf(
					"failed to download the Swagger/OpenAPI definition: %v. Verify the URL is reachable and points to a valid specification document.",
					err,
				),
			), nil
		}
		result := definitionDownloadResult{
			FilePath:      savedDefinition.FilePath,
			ConfigFile:    ".swagger-mcp",
			ConfigSnippet: fmt.Sprintf("SWAGGER_FILEPATH=%s", savedDefinition.FilePath),
		}
		return mcpgo.NewToolResultStructured(
			result,
			fmt.Sprintf(
				"Successfully downloaded and saved Swagger definition.\n\nIMPORTANT: You must now create a file named '.swagger-mcp' in the solution root with the following content:\n\nSWAGGER_FILEPATH=%s\n\nThis file is required by all other Swagger-related tools.",
				savedDefinition.FilePath,
			),
		), nil
	})

	// swagger_list_endpoints
	limitDesc := fmt.Sprintf("Maximum endpoints to return (1-%d, default: %d)", maxEndpointLimit, defaultEndpointLimit)
	s.AddTool(mcpgo.NewTool(
		"swagger_list_endpoints",
		mcpgo.WithDescription(
			`Lists endpoints from a Swagger/OpenAPI definition including HTTP methods and descriptions.

Source priority: CLI --swagger-url flag > swaggerFilePath parameter > .swagger-mcp file.

Args:
  - swaggerFilePath (string, optional): Path to the Swagger file. If omitted, uses
    --swagger-url flag or the path stored in .swagger-mcp (SWAGGER_FILEPATH=...).
  - limit (number, optional): Maximum number of endpoints to return (1-200, default: 50)
  - offset (number, optional): Number of endpoints to skip for pagination (default: 0)
  - response_format (string, optional): Output format — 'markdown' or 'json' (default: markdown)

Returns: Paginated list of endpoints with path, method, summary, description,
and pagination metadata (total, has_more, next_offset)

Error Handling:
  - Returns error if no Swagger source is configured and swaggerFilePath is empty
  - Returns error if the file cannot be read or parsed`,
		),
		mcpgo.WithString("swaggerFilePath", mcpgo.Description(swaggerPathDescription)),
		mcpgo.WithNumber("limit", mcpgo.Description(limitDesc)),
		mcpgo.WithNumber("offset", mcpgo.Description("Number of endpoints to skip for pagination (default: 0)")),
		mcpgo.WithString(
			"response_format",
			mcpgo.Description(responseFormatDescription),
			mcpgo.Enum("markdown", "json"),
		),
		mcpgo.WithOutputSchema[endpointListResult](),
		mcpgo.WithToolAnnotation(mcpgo.ToolAnnotation{
			Title:           "List API Endpoints",
			ReadOnlyHint:    new(true),
			DestructiveHint: new(false),
			IdempotentHint:  new(true),
			OpenWorldHint:   new(false),
		}),
	), func(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		document, err := resolver.Load(req.GetString("swaggerFilePath", ""))
		if err != nil {
			return mcpgo.NewToolResultError(swaggerSourceError("list endpoints", err)), nil
		}
		endpoints, err := openapi.ListEndpoints(document)
		if err != nil {
			return mcpgo.NewToolResultError(fmt.Sprintf("failed to list endpoints: %v", err)), nil
		}
		endpoints = openapi.FilterEndpoints(endpoints, filter)

		limit := max(1, min(int(req.GetFloat("limit", defaultEndpointLimit)), maxEndpointLimit))
		offset := max(0, int(req.GetFloat("offset", 0)))
		format := req.GetString("response_format", "markdown")

		total := len(endpoints)
		if offset >= total {
			offset = total
		}
		page := endpoints[offset:]
		if len(page) > limit {
			page = page[:limit]
		}
		hasMore := offset+len(page) < total
		result := endpointListResult{
			Total:     total,
			Count:     len(page),
			Offset:    offset,
			Endpoints: page,
			HasMore:   hasMore,
		}
		if hasMore {
			next := offset + len(page)
			result.NextOffset = &next
		}

		text, err := formatEndpointList(result, format)
		if err != nil {
			return mcpgo.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
		}
		return mcpgo.NewToolResultStructured(result, text), nil
	})

	// swagger_list_endpoint_models
	s.AddTool(mcpgo.NewTool(
		"swagger_list_endpoint_models",
		mcpgo.WithDescription(
			`Lists all request and response models used by a specific endpoint in a Swagger/OpenAPI definition.

Source priority: CLI --swagger-url flag > swaggerFilePath parameter > .swagger-mcp file.

Args:
  - path (string, required): The endpoint path (e.g. /pets or /pets/{id})
  - method (string, required): The HTTP method (e.g. GET, POST, PUT, DELETE)
  - swaggerFilePath (string, optional): Path to the Swagger file. If omitted, uses
    --swagger-url flag or the path stored in .swagger-mcp (SWAGGER_FILEPATH=...).
  - response_format (string, optional): Output format — 'markdown' or 'json' (default: markdown)

Returns: Request/response schema models for the endpoint

Error Handling:
  - Returns error if the endpoint path or method is not found in the definition
  - Returns error if no Swagger source is configured and swaggerFilePath is empty`,
		),
		mcpgo.WithString("path", mcpgo.Required(), mcpgo.Description("The endpoint path (e.g. /pets or /pets/{id})")),
		mcpgo.WithString(
			"method",
			mcpgo.Required(),
			mcpgo.Description("The HTTP method of the endpoint (e.g. GET, POST, PUT, DELETE)"),
		),
		mcpgo.WithString("swaggerFilePath", mcpgo.Description(swaggerPathDescription)),
		mcpgo.WithString(
			"response_format",
			mcpgo.Description(responseFormatDescription),
			mcpgo.Enum("markdown", "json"),
		),
		mcpgo.WithOutputSchema[modelListResult](),
		mcpgo.WithToolAnnotation(mcpgo.ToolAnnotation{
			Title:           "List Endpoint Models",
			ReadOnlyHint:    new(true),
			DestructiveHint: new(false),
			IdempotentHint:  new(true),
			OpenWorldHint:   new(false),
		}),
	), func(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		document, err := resolver.Load(req.GetString("swaggerFilePath", ""))
		if err != nil {
			return mcpgo.NewToolResultError(swaggerSourceError("list endpoint models", err)), nil
		}
		endpointPath, err := req.RequireString("path")
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		method, err := req.RequireString("method")
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		models, err := openapi.ListEndpointModels(document, endpointPath, method)
		if err != nil {
			return mcpgo.NewToolResultError(fmt.Sprintf("failed to list endpoint models: %v", err)), nil
		}
		format := req.GetString("response_format", "markdown")
		result := modelListResult{
			Path:   endpointPath,
			Method: strings.ToUpper(method),
			Count:  len(models),
			Models: models,
		}
		text, err := formatModelList(result, format)
		if err != nil {
			return mcpgo.NewToolResultError(fmt.Sprintf("failed to format response: %v", err)), nil
		}
		return mcpgo.NewToolResultStructured(result, text), nil
	})

	// swagger_get_version
	s.AddTool(mcpgo.NewTool(
		"swagger_get_version",
		mcpgo.WithDescription(`Returns the current version number of the Swagger MCP Server.

Args:
  - response_format (string, optional): Output format — 'markdown' or 'json' (default: json)

Returns: Version metadata with a "version" field containing the semver string

Error Handling:
  - Returns error if the version cannot be serialized (unexpected)`),
		mcpgo.WithString(
			"response_format",
			mcpgo.Description("Output format: 'markdown' for human-readable or 'json' for machine-readable (default: json)"),
			mcpgo.Enum("markdown", "json"),
		),
		mcpgo.WithOutputSchema[versionToolResult](),
		mcpgo.WithToolAnnotation(mcpgo.ToolAnnotation{
			Title:           "Get Server Version",
			ReadOnlyHint:    new(true),
			DestructiveHint: new(false),
			IdempotentHint:  new(true),
			OpenWorldHint:   new(false),
		}),
	), func(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		result := versionToolResult{Version: config.Version}
		format := req.GetString("response_format", "json")
		text, err := formatStructuredText(result, format, fmt.Sprintf("Version: %s", config.Version))
		if err != nil {
			return mcpgo.NewToolResultError(fmt.Sprintf("failed to format version response: %v", err)), nil
		}
		return mcpgo.NewToolResultStructured(result, text), nil
	})

}

// formatEndpointList renders an endpointListResult as either Markdown or JSON,
// truncating output that exceeds characterLimit.
func formatEndpointList(result endpointListResult, format string) (string, error) {
	var text string
	if format == "json" {
		b, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", err
		}
		text = string(b)
	} else {
		text = formatEndpointListMarkdown(result)
	}

	if len(text) > characterLimit {
		text = text[:characterLimit] + "\n\n[Response truncated. Use 'offset' or 'limit' parameters to narrow results.]"
	}
	return text, nil
}

func formatEndpointListMarkdown(result endpointListResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# API Endpoints (%d total", result.Total)
	if result.HasMore {
		fmt.Fprintf(&sb, ", showing %d–%d", result.Offset+1, result.Offset+result.Count)
	}
	sb.WriteString(")\n\n")
	for _, ep := range result.Endpoints {
		fmt.Fprintf(&sb, "## %s %s\n", ep.Method, ep.Path)
		if ep.Summary != "" {
			fmt.Fprintf(&sb, "- **Summary**: %s\n", ep.Summary)
		}
		if ep.Description != "" {
			fmt.Fprintf(&sb, "- **Description**: %s\n", ep.Description)
		}
		if ep.OperationID != "" {
			fmt.Fprintf(&sb, "- **OperationID**: %s\n", ep.OperationID)
		}
		if len(ep.Tags) > 0 {
			fmt.Fprintf(&sb, "- **Tags**: %s\n", strings.Join(ep.Tags, ", "))
		}
		sb.WriteString("\n")
	}
	if result.HasMore {
		fmt.Fprintf(&sb, "*More endpoints available. Use offset=%d to continue.*\n", *result.NextOffset)
	}
	return sb.String()
}

// formatModelList renders a slice of openapi.Model as either Markdown or JSON,
// truncating output that exceeds characterLimit.
func formatModelList(result modelListResult, format string) (string, error) {
	var text string
	if format == "json" {
		b, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", err
		}
		text = string(b)
	} else {
		text = formatModelListMarkdown(result)
	}

	if len(text) > characterLimit {
		text = text[:characterLimit] + "\n\n[Response truncated. Use a more specific endpoint path or filter.]"
	}
	return text, nil
}

func formatModelListMarkdown(result modelListResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Models for %s %s\n\n", result.Method, result.Path)
	if len(result.Models) == 0 {
		sb.WriteString("No models found for this endpoint.\n")
		return sb.String()
	}
	for _, m := range result.Models {
		fmt.Fprintf(&sb, "## %s\n", m.Name)
		if m.Schema != nil {
			schemaBytes, err := json.MarshalIndent(m.Schema, "", "  ")
			if err == nil {
				sb.WriteString("```json\n")
				sb.Write(schemaBytes)
				sb.WriteString("\n```\n")
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func swaggerSourceError(action string, err error) string {
	return fmt.Sprintf(
		"failed to %s: %v. Provide --swagger-url, pass swaggerFilePath, or create a .swagger-mcp file with SWAGGER_FILEPATH=<path>.",
		action,
		err,
	)
}

func formatStructuredText(data any, format, markdown string) (string, error) {
	if format != "json" {
		return markdown, nil
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
