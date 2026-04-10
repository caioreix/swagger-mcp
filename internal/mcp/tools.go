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

	// getSwaggerDefinition
	getSwaggerDef := mcpgo.NewTool(
		"getSwaggerDefinition",
		mcpgo.WithDescription(
			"Fetches a Swagger/OpenAPI definition from a URL and saves it locally. IMPORTANT: After calling this tool, you will receive a response containing a filePath property. You MUST then create a configuration file named .swagger-mcp in the root of the solution and write the file path to it in the format SWAGGER_FILEPATH=TheFullFilePath.",
		),
		mcpgo.WithString("url", mcpgo.Required(), mcpgo.Description("The URL of the Swagger definition")),
		mcpgo.WithString(
			"saveLocation",
			mcpgo.Required(),
			mcpgo.Description(
				"The location where to save the Swagger definition file. This should be the current solution's root folder.",
			),
		),
	)
	getSwaggerDef.Annotations = mcpgo.ToolAnnotation{
		ReadOnlyHint:    new(false),
		DestructiveHint: new(false),
	}
	s.AddTool(getSwaggerDef, func(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
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

	// listEndpoints
	listEp := mcpgo.NewTool(
		"listEndpoints",
		mcpgo.WithDescription(
			"Lists all endpoints from the Swagger definition including their HTTP methods and descriptions. Priority: CLI --swagger-url > swaggerFilePath parameter > .swagger-mcp file.",
		),
		mcpgo.WithString("swaggerFilePath", mcpgo.Description(swaggerPathDescription)),
	)
	listEp.Annotations = mcpgo.ToolAnnotation{
		ReadOnlyHint:    new(true),
		DestructiveHint: new(false),
	}
	s.AddTool(listEp, func(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
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

	// listEndpointModels
	listModels := mcpgo.NewTool(
		"listEndpointModels",
		mcpgo.WithDescription(
			"Lists all models used by a specific endpoint from the Swagger definition. Priority: CLI --swagger-url > swaggerFilePath parameter > .swagger-mcp file.",
		),
		mcpgo.WithString("path", mcpgo.Required(), mcpgo.Description("The path of the endpoint (e.g. /pets)")),
		mcpgo.WithString(
			"method",
			mcpgo.Required(),
			mcpgo.Description("The HTTP method of the endpoint (e.g. GET, POST, PUT, DELETE)"),
		),
		mcpgo.WithString("swaggerFilePath", mcpgo.Description(swaggerPathDescription)),
	)
	listModels.Annotations = mcpgo.ToolAnnotation{
		ReadOnlyHint:    new(true),
		DestructiveHint: new(false),
	}
	s.AddTool(listModels, func(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
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

	// version
	versionTool := mcpgo.NewTool("version",
		mcpgo.WithDescription("Returns the current version number of the Swagger MCP Server."),
	)
	versionTool.Annotations = mcpgo.ToolAnnotation{
		ReadOnlyHint:    new(true),
		DestructiveHint: new(false),
	}
	s.AddTool(versionTool, func(_ context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		data, err := json.MarshalIndent(map[string]string{"version": config.Version}, "", "  ")
		if err != nil {
			return mcpgo.NewToolResultError(fmt.Sprintf("error: %v", err)), nil
		}
		return mcpgo.NewToolResultText(string(data)), nil
	})

	_ = logger
}
