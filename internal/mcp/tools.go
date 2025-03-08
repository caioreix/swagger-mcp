package mcp

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/caioreix/swagger-mcp/internal/codegen"
	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/openapi"
)

func (s *server) allToolDefinitions() []toolDefinition {
	defs := s.toolDefinitions()
	for _, pt := range s.proxyTools {
		defs = append(defs, pt.Definition)
	}
	return defs
}

func (s *server) toolDefinitions() []toolDefinition {
	swaggerPathDescription := "Optional path to the Swagger file. Used only if --swagger-url is not provided. You can find this path in the .swagger-mcp file in the solution root with the format SWAGGER_FILEPATH=path/to/file.json."
	return []toolDefinition{
		{
			Name:        "getSwaggerDefinition",
			Description: "Fetches a Swagger/OpenAPI definition from a URL and saves it locally. IMPORTANT: After calling this tool, you will receive a response containing a filePath property. You MUST then create a configuration file named .swagger-mcp in the root of the solution and write the file path to it in the format SWAGGER_FILEPATH=TheFullFilePath.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"url": map[string]any{"type": "string", "description": "The URL of the Swagger definition"}, "saveLocation": map[string]any{"type": "string", "description": "The location where to save the Swagger definition file. This should be the current solution's root folder."}}, "required": []string{"url", "saveLocation"}},
		},
		{
			Name:        "listEndpoints",
			Description: "Lists all endpoints from the Swagger definition including their HTTP methods and descriptions. Priority: CLI --swagger-url > swaggerFilePath parameter > .swagger-mcp file.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"swaggerFilePath": map[string]any{"type": "string", "description": swaggerPathDescription}}, "required": []string{}},
		},
		{
			Name:        "listEndpointModels",
			Description: "Lists all models used by a specific endpoint from the Swagger definition. Priority: CLI --swagger-url > swaggerFilePath parameter > .swagger-mcp file.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": "The path of the endpoint (e.g. /pets)"}, "method": map[string]any{"type": "string", "description": "The HTTP method of the endpoint (e.g. GET, POST, PUT, DELETE)"}, "swaggerFilePath": map[string]any{"type": "string", "description": swaggerPathDescription}}, "required": []string{"path", "method"}},
		},
		{
			Name:        "generateModelCode",
			Description: "Generates Go model code for a schema from the Swagger definition. Priority: CLI --swagger-url > swaggerFilePath parameter > .swagger-mcp file.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"modelName": map[string]any{"type": "string", "description": "The name of the model to generate code for"}, "swaggerFilePath": map[string]any{"type": "string", "description": swaggerPathDescription}}, "required": []string{"modelName"}},
		},
		{
			Name:        "generateEndpointToolCode",
			Description: "Generates Go code for an MCP tool definition scaffold based on a Swagger endpoint. Priority: CLI --swagger-url > swaggerFilePath parameter > .swagger-mcp file.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": "The path of the endpoint (e.g. /pets)"}, "method": map[string]any{"type": "string", "description": "The HTTP method of the endpoint (e.g. GET, POST, PUT, DELETE)"}, "swaggerFilePath": map[string]any{"type": "string", "description": swaggerPathDescription}, "includeApiInName": map[string]any{"type": "boolean", "description": "Whether to include api segments in the generated tool name (default: false)"}, "includeVersionInName": map[string]any{"type": "boolean", "description": "Whether to include version segments in the generated tool name (default: false)"}, "singularizeResourceNames": map[string]any{"type": "boolean", "description": "Whether to singularize resource names in the generated tool name (default: true)"}}, "required": []string{"path", "method"}},
		},
		{
			Name:        "generateServer",
			Description: "Generates a complete, runnable Go MCP server project from a Swagger/OpenAPI definition. Returns a file tree (main.go, server.go, tools.go, handlers.go, helpers.go, go.mod) ready to build and run. Priority: CLI --swagger-url > swaggerFilePath parameter > .swagger-mcp file.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"swaggerFilePath": map[string]any{"type": "string", "description": swaggerPathDescription}, "moduleName": map[string]any{"type": "string", "description": "Go module name for the generated server (e.g. github.com/user/my-mcp-server). Defaults to github.com/generated/mcp-server."}, "transportModes": map[string]any{"type": "string", "description": "Comma-separated transport modes: stdio, sse, streamable-http. Default: stdio."}, "proxyMode": map[string]any{"type": "boolean", "description": "If true, generates fully functional proxy handlers that forward requests to the original REST API. If false, generates stub handlers. Default: true."}, "endpoints": map[string]any{"type": "string", "description": "Comma-separated endpoint paths to include (e.g. /pets,/pets/{id}). If empty, includes all endpoints."}}, "required": []string{}},
		},
		{
			Name:        "version",
			Description: "Returns the current version number of the Swagger MCP Server.",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}},
		},
	}
}

func (s *server) callTool(name string, arguments map[string]any, extraHeaders map[string]string) (map[string]any, error) {
	switch name {
	case "getSwaggerDefinition":
		url, err := requiredString(arguments, "url")
		if err != nil {
			return nil, err
		}
		saveLocation, err := requiredString(arguments, "saveLocation")
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(saveLocation) {
			saveLocation = filepath.Join(s.cfg.WorkingDir, saveLocation)
		}
		savedDefinition, err := s.resolver.DownloadDefinition(url, saveLocation)
		if err != nil {
			return nil, fmt.Errorf("error retrieving swagger definition: %w", err)
		}
		return textResult(fmt.Sprintf("Successfully downloaded and saved Swagger definition.\n\nIMPORTANT: You must now create a file named '.swagger-mcp' in the solution root with the following content:\n\nSWAGGER_FILEPATH=%s\n\nThis file is required by all other Swagger-related tools.", savedDefinition.FilePath), false), nil
	case "listEndpoints":
		document, err := s.resolver.Load(optionalString(arguments, "swaggerFilePath"))
		if err != nil {
			return nil, fmt.Errorf("error retrieving endpoints: %w", err)
		}
		endpoints, err := openapi.ListEndpoints(document)
		if err != nil {
			return nil, fmt.Errorf("error retrieving endpoints: %w", err)
		}
		endpoints = openapi.FilterEndpoints(endpoints, s.filter)
		return jsonTextResult(endpoints, false)
	case "listEndpointModels":
		document, err := s.resolver.Load(optionalString(arguments, "swaggerFilePath"))
		if err != nil {
			return nil, fmt.Errorf("error retrieving endpoint models: %w", err)
		}
		endpointPath, err := requiredString(arguments, "path")
		if err != nil {
			return nil, err
		}
		method, err := requiredString(arguments, "method")
		if err != nil {
			return nil, err
		}
		models, err := openapi.ListEndpointModels(document, endpointPath, method)
		if err != nil {
			return nil, fmt.Errorf("error retrieving endpoint models: %w", err)
		}
		return jsonTextResult(models, false)
	case "generateModelCode":
		document, err := s.resolver.Load(optionalString(arguments, "swaggerFilePath"))
		if err != nil {
			return nil, fmt.Errorf("error generating model code: %w", err)
		}
		modelName, err := requiredString(arguments, "modelName")
		if err != nil {
			return nil, err
		}
		generatedCode, err := codegen.GenerateModelCode(document, modelName)
		if err != nil {
			return nil, fmt.Errorf("error generating model code: %w", err)
		}
		return textResult(generatedCode, false), nil
	case "generateEndpointToolCode":
		document, err := s.resolver.Load(optionalString(arguments, "swaggerFilePath"))
		if err != nil {
			return nil, fmt.Errorf("error generating endpoint tool code: %w", err)
		}
		endpointPath, err := requiredString(arguments, "path")
		if err != nil {
			return nil, err
		}
		method, err := requiredString(arguments, "method")
		if err != nil {
			return nil, err
		}
		generatedCode, err := codegen.GenerateEndpointToolCode(document, codegen.GenerateEndpointToolCodeParams{
			Path:                    endpointPath,
			Method:                  method,
			IncludeAPIInName:        optionalBool(arguments, "includeApiInName", false),
			IncludeVersionInName:    optionalBool(arguments, "includeVersionInName", false),
			SingularizeResourceName: optionalBool(arguments, "singularizeResourceNames", true),
		})
		if err != nil {
			return nil, fmt.Errorf("error generating endpoint tool code: %w", err)
		}
		return textResult(generatedCode, strings.HasPrefix(strings.TrimSpace(generatedCode), "MCP Schema Validation Failed")), nil
	case "generateServer":
		document, err := s.resolver.Load(optionalString(arguments, "swaggerFilePath"))
		if err != nil {
			return nil, fmt.Errorf("error generating server: %w", err)
		}
		transportStr := optionalString(arguments, "transportModes")
		transportModes := []string{"stdio"}
		if transportStr != "" {
			transportModes = strings.Split(transportStr, ",")
			for i := range transportModes {
				transportModes[i] = strings.TrimSpace(transportModes[i])
			}
		}
		endpointStr := optionalString(arguments, "endpoints")
		var endpoints []string
		if endpointStr != "" {
			endpoints = strings.Split(endpointStr, ",")
			for i := range endpoints {
				endpoints[i] = strings.TrimSpace(endpoints[i])
			}
		}
		files, err := codegen.GenerateCompleteServer(document, codegen.ServerGenParams{
			ModuleName:     optionalString(arguments, "moduleName"),
			TransportModes: transportModes,
			ProxyMode:      optionalBool(arguments, "proxyMode", true),
			Endpoints:      endpoints,
		})
		if err != nil {
			return nil, fmt.Errorf("error generating server: %w", err)
		}
		return jsonTextResult(files, false)
	case "version":
		return jsonTextResult(map[string]string{"version": config.Version}, false)
	default:
		// Check proxy tools
		for _, pt := range s.proxyTools {
			if pt.Definition.Name == name {
				baseURL := s.cfg.BaseURL
				document, err := s.resolver.Load("")
				if baseURL == "" {
					if err == nil {
						baseURL = openapi.ExtractBaseURL(document)
					}
				} else if err == nil {
					// User provided --base-url (host only); append basePath from spec if present.
					if basePath := openapi.ExtractBasePath(document); basePath != "" {
						trimmed := strings.TrimRight(baseURL, "/")
						if !strings.HasSuffix(trimmed, strings.TrimRight(basePath, "/")) {
							baseURL = trimmed + basePath
						}
					}
				}
				return executeProxyCall(pt, arguments, baseURL, s.cfg, extraHeaders)
			}
		}
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}
