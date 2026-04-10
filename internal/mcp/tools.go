package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/openapi"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpgoserver "github.com/mark3labs/mcp-go/server"
)

func registerStaticTools( //nolint:gocognit,funlen
	s *mcpgoserver.MCPServer,
	resolver openapi.SourceResolver,
	filter *openapi.EndpointFilter,
	cfg config.Config,
	logger *slog.Logger,
) {
	swaggerPathDescription := "Optional path to the Swagger file. Used only if --swagger-url is not provided. You can find this path in the .swagger-mcp file in the solution root with the format SWAGGER_FILEPATH=path/to/file.json."

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
		saveLocation := req.GetString("saveLocation", "")
		if strings.TrimSpace(saveLocation) == "" {
			return mcpgo.NewToolResultError("saveLocation is required"), nil
		}
		if !filepath.IsAbs(saveLocation) {
			saveLocation = filepath.Join(cfg.WorkingDir, saveLocation)
		}
		savedDefinition, err := resolver.DownloadDefinition(urlVal, saveLocation)
		if err != nil {
			return mcpgo.NewToolResultError(fmt.Sprintf("error retrieving swagger definition: %v", err)), nil
		}
		return mcpgo.NewToolResultText(
			fmt.Sprintf(
				"Successfully downloaded and saved Swagger definition.\n\nIMPORTANT: You must now create a file named '.swagger-mcp' in the solution root with the following content:\n\nSWAGGER_FILEPATH=%s\n\nThis file is required by all other Swagger-related tools.",
				savedDefinition.FilePath,
			),
		), nil
	})

	// swagger_list_endpoints
	s.AddTool(mcpgo.NewTool(
		"swagger_list_endpoints",
		mcpgo.WithDescription(`Lists all endpoints from a Swagger/OpenAPI definition including HTTP methods and descriptions.

Source priority: CLI --swagger-url flag > swaggerFilePath parameter > .swagger-mcp file.

Args:
  - swaggerFilePath (string, optional): Path to the Swagger file. If omitted, uses
    --swagger-url flag or the path stored in .swagger-mcp (SWAGGER_FILEPATH=...).

Returns: JSON array of endpoints with path, method, summary, and description fields

Error Handling:
  - Returns error if no Swagger source is configured and swaggerFilePath is empty
  - Returns error if the file cannot be read or parsed`),
		mcpgo.WithString("swaggerFilePath", mcpgo.Description(swaggerPathDescription)),
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
			return mcpgo.NewToolResultError(fmt.Sprintf("error retrieving endpoints: %v", err)), nil
		}
		endpoints, err := openapi.ListEndpoints(document)
		if err != nil {
			return mcpgo.NewToolResultError(fmt.Sprintf("error retrieving endpoints: %v", err)), nil
		}
		endpoints = openapi.FilterEndpoints(endpoints, filter)
		data, err := json.MarshalIndent(endpoints, "", "  ")
		if err != nil {
			return mcpgo.NewToolResultError(fmt.Sprintf("error marshaling endpoints: %v", err)), nil
		}
		return mcpgo.NewToolResultText(string(data)), nil
	})

	// swagger_list_endpoint_models
	s.AddTool(mcpgo.NewTool(
		"swagger_list_endpoint_models",
		mcpgo.WithDescription(`Lists all request and response models used by a specific endpoint in a Swagger/OpenAPI definition.

Source priority: CLI --swagger-url flag > swaggerFilePath parameter > .swagger-mcp file.

Args:
  - path (string, required): The endpoint path (e.g. /pets or /pets/{id})
  - method (string, required): The HTTP method (e.g. GET, POST, PUT, DELETE)
  - swaggerFilePath (string, optional): Path to the Swagger file. If omitted, uses
    --swagger-url flag or the path stored in .swagger-mcp (SWAGGER_FILEPATH=...).

Returns: JSON object with the request/response schema models for the endpoint

Error Handling:
  - Returns error if the endpoint path or method is not found in the definition
  - Returns error if no Swagger source is configured and swaggerFilePath is empty`),
		mcpgo.WithString("path", mcpgo.Required(), mcpgo.Description("The endpoint path (e.g. /pets or /pets/{id})")),
		mcpgo.WithString(
			"method",
			mcpgo.Required(),
			mcpgo.Description("The HTTP method of the endpoint (e.g. GET, POST, PUT, DELETE)"),
		),
		mcpgo.WithString("swaggerFilePath", mcpgo.Description(swaggerPathDescription)),
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
			return mcpgo.NewToolResultError(fmt.Sprintf("error retrieving endpoint models: %v", err)), nil
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
			return mcpgo.NewToolResultError(fmt.Sprintf("error retrieving endpoint models: %v", err)), nil
		}
		data, marshalErr := json.MarshalIndent(models, "", "  ")
		if marshalErr != nil {
			return mcpgo.NewToolResultError(fmt.Sprintf("error marshaling models: %v", marshalErr)), nil
		}
		return mcpgo.NewToolResultText(string(data)), nil
	})

	// swagger_get_version
	s.AddTool(mcpgo.NewTool(
		"swagger_get_version",
		mcpgo.WithDescription(`Returns the current version number of the Swagger MCP Server.

Returns: JSON object with a "version" field containing the semver string

Error Handling:
  - Returns error if the version cannot be serialized (unexpected)`),
		mcpgo.WithToolAnnotation(mcpgo.ToolAnnotation{
			Title:           "Get Server Version",
			ReadOnlyHint:    new(true),
			DestructiveHint: new(false),
			IdempotentHint:  new(true),
			OpenWorldHint:   new(false),
		}),
	), func(_ context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		data, err := json.MarshalIndent(map[string]string{"version": config.Version}, "", "  ")
		if err != nil {
			return mcpgo.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
		}
		return mcpgo.NewToolResultText(string(data)), nil
	})

	_ = logger
}
