package mcp

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/logging"
	"github.com/caioreix/swagger-mcp/internal/openapi"
	mcpgoserver "github.com/mark3labs/mcp-go/server"
)

// NewServer creates a configured mcp-go MCPServer with all tools and prompts registered.
func NewServer(cfg config.Config, logger *slog.Logger) (*mcpgoserver.MCPServer, error) {
	componentLogger := logging.WithComponent(logger, "mcp.server")
	resolver := openapi.NewSourceResolver(cfg.WorkingDir, cfg.SwaggerURL, componentLogger)
	if err := resolver.Preload(); err != nil {
		return nil, fmt.Errorf("initialize swagger source resolver: %w", err)
	}

	filter, err := openapi.NewEndpointFilter(
		cfg.Filter.IncludePaths,
		cfg.Filter.ExcludePaths,
		cfg.Filter.IncludeMethods,
		cfg.Filter.ExcludeMethods,
	)
	if err != nil {
		return nil, fmt.Errorf("compile endpoint filters: %w", err)
	}

	var proxyTools []proxyTool

	// Single-API proxy mode (CLI flags).
	if cfg.ProxyMode {
		singleTools, initErr := initProxyTools(resolver, cfg, filter, componentLogger)
		if initErr != nil {
			return nil, fmt.Errorf("initialize proxy tools: %w", initErr)
		}
		proxyTools = append(proxyTools, singleTools...)
	}

	// Multi-API proxy mode (config file).
	if len(cfg.APIs) > 0 {
		multiTools, initErr := initMultiAPIProxyTools(cfg, componentLogger)
		if initErr != nil {
			return nil, fmt.Errorf("initialize multi-api proxy tools: %w", initErr)
		}
		proxyTools = append(proxyTools, multiTools...)
	}

	instructions := "Use swagger_get_definition to download a Swagger/OpenAPI document, then call the Swagger tools or the add-endpoint prompt to inspect the API and generate scaffolding."
	if cfg.ProxyMode || len(cfg.APIs) > 0 {
		instructions = "This server is running in proxy mode. API endpoints are available as tools — call them directly to interact with the API. " + instructions
	}

	s := mcpgoserver.NewMCPServer("swagger-mcp-server", config.Version,
		mcpgoserver.WithToolCapabilities(false),
		mcpgoserver.WithRecovery(),
		mcpgoserver.WithInstructions(instructions),
	)

	registerStaticTools(s, resolver, filter, cfg, componentLogger)
	registerProxyTools(s, proxyTools, cfg)
	registerPrompts(s)

	return s, nil
}

func initProxyTools(
	resolver openapi.SourceResolver,
	cfg config.Config,
	filter *openapi.EndpointFilter,
	logger *slog.Logger,
) ([]proxyTool, error) {
	document, err := resolver.Load("")
	if err != nil {
		return nil, fmt.Errorf("load swagger definition for proxy mode: %w", err)
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = openapi.ExtractBaseURL(document)
	} else {
		if basePath := openapi.ExtractBasePath(document); basePath != "" {
			trimmed := strings.TrimRight(baseURL, "/")
			if !strings.HasSuffix(trimmed, strings.TrimRight(basePath, "/")) {
				baseURL = trimmed + basePath
			}
		}
	}
	if baseURL == "" {
		return nil, errors.New("no base URL available: provide --base-url or ensure the swagger spec defines one")
	}

	tools, err := buildProxyTools(document, baseURL, filter, "", cfg.Auth, cfg.Headers)
	if err != nil {
		return nil, err
	}

	logger.Info("proxy mode enabled", "tools_registered", len(tools), "base_url", baseURL)
	return tools, nil
}

func initMultiAPIProxyTools(cfg config.Config, logger *slog.Logger) ([]proxyTool, error) {
	var all []proxyTool
	for _, api := range cfg.APIs {
		tools, err := initAPIProxyTools(api, cfg.WorkingDir, logger)
		if err != nil {
			return nil, fmt.Errorf("initialize proxy tools for API %q: %w", api.Name, err)
		}
		all = append(all, tools...)
	}
	return all, nil
}

func initAPIProxyTools(api config.APIConfig, workingDir string, logger *slog.Logger) ([]proxyTool, error) {
	apiResolver := openapi.NewSourceResolver(workingDir, api.SwaggerURL, logger)
	if err := apiResolver.Preload(); err != nil {
		return nil, fmt.Errorf("load swagger spec: %w", err)
	}

	document, err := apiResolver.Load("")
	if err != nil {
		return nil, fmt.Errorf("parse swagger definition: %w", err)
	}

	baseURL := api.BaseURL
	if baseURL == "" {
		baseURL = openapi.ExtractBaseURL(document)
	} else {
		if basePath := openapi.ExtractBasePath(document); basePath != "" {
			trimmed := strings.TrimRight(baseURL, "/")
			if !strings.HasSuffix(trimmed, strings.TrimRight(basePath, "/")) {
				baseURL = trimmed + basePath
			}
		}
	}
	if baseURL == "" {
		return nil, errors.New("no base URL available: set \"base_url\" or ensure the swagger spec defines one")
	}

	filter, err := openapi.NewEndpointFilter(
		api.Filter.IncludePaths,
		api.Filter.ExcludePaths,
		api.Filter.IncludeMethods,
		api.Filter.ExcludeMethods,
	)
	if err != nil {
		return nil, fmt.Errorf("compile endpoint filters: %w", err)
	}

	tools, err := buildProxyTools(document, baseURL, filter, api.Name, api.Auth, api.Headers)
	if err != nil {
		return nil, err
	}

	logger.Info(
		"multi-api proxy tools registered",
		"api",
		api.Name,
		"tools_registered",
		len(tools),
		"base_url",
		baseURL,
	)
	return tools, nil
}
