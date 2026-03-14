package mcp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/logging"
	"github.com/caioreix/swagger-mcp/internal/openapi"
)

type server struct {
	cfg        config.Config
	logger     *slog.Logger
	resolver   openapi.SourceResolver
	proxyTools []proxyTool
	filter     *openapi.EndpointFilter
}

func NewServer(cfg config.Config, logger *slog.Logger) (*server, error) {
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

	s := &server{
		cfg:      cfg,
		logger:   componentLogger,
		resolver: resolver,
		filter:   filter,
	}

	// Single-API proxy mode (CLI flags).
	if cfg.ProxyMode {
		if err := s.initProxyTools(); err != nil {
			return nil, fmt.Errorf("initialize proxy tools: %w", err)
		}
	}

	// Multi-API proxy mode (config file).
	if len(cfg.APIs) > 0 {
		if err := s.initMultiAPIProxyTools(); err != nil {
			return nil, fmt.Errorf("initialize multi-api proxy tools: %w", err)
		}
	}

	return s, nil
}

func (s *server) initProxyTools() error {
	document, err := s.resolver.Load("")
	if err != nil {
		return fmt.Errorf("load swagger definition for proxy mode: %w", err)
	}

	baseURL := s.cfg.BaseURL
	if baseURL == "" {
		baseURL = openapi.ExtractBaseURL(document)
	} else {
		// User provided --base-url (host only); append basePath from spec if present.
		if basePath := openapi.ExtractBasePath(document); basePath != "" {
			trimmed := strings.TrimRight(baseURL, "/")
			if !strings.HasSuffix(trimmed, strings.TrimRight(basePath, "/")) {
				baseURL = trimmed + basePath
			}
		}
	}
	if baseURL == "" {
		return fmt.Errorf("no base URL available: provide --base-url or ensure the swagger spec defines one")
	}

	tools, err := buildProxyTools(document, baseURL, s.filter, "", s.cfg.Auth, s.cfg.Headers)
	if err != nil {
		return err
	}

	s.proxyTools = append(s.proxyTools, tools...)
	s.logger.Info("proxy mode enabled", "tools_registered", len(tools), "base_url", baseURL)
	return nil
}

func (s *server) initMultiAPIProxyTools() error {
	for _, api := range s.cfg.APIs {
		if err := s.initAPIProxyTools(api); err != nil {
			return fmt.Errorf("initialize proxy tools for API %q: %w", api.Name, err)
		}
	}
	return nil
}

func (s *server) initAPIProxyTools(api config.APIConfig) error {
	apiResolver := openapi.NewSourceResolver(s.cfg.WorkingDir, api.SwaggerURL, s.logger)
	if err := apiResolver.Preload(); err != nil {
		return fmt.Errorf("load swagger spec: %w", err)
	}

	document, err := apiResolver.Load("")
	if err != nil {
		return fmt.Errorf("parse swagger definition: %w", err)
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
		return fmt.Errorf("no base URL available: set \"base_url\" or ensure the swagger spec defines one")
	}

	filter, err := openapi.NewEndpointFilter(
		api.Filter.IncludePaths,
		api.Filter.ExcludePaths,
		api.Filter.IncludeMethods,
		api.Filter.ExcludeMethods,
	)
	if err != nil {
		return fmt.Errorf("compile endpoint filters: %w", err)
	}

	tools, err := buildProxyTools(document, baseURL, filter, api.Name, api.Auth, api.Headers)
	if err != nil {
		return err
	}

	s.proxyTools = append(s.proxyTools, tools...)
	s.logger.Info("multi-api proxy tools registered", "api", api.Name, "tools_registered", len(tools), "base_url", baseURL)
	return nil
}

func (s *server) HandleJSON(line []byte) ([]byte, error) {
	return s.handleJSONInternal(line, nil)
}

func (s *server) HandleJSONWithHeaders(line []byte, extraHeaders map[string]string) ([]byte, error) {
	return s.handleJSONInternal(line, extraHeaders)
}

func (s *server) handleJSONInternal(line []byte, extraHeaders map[string]string) ([]byte, error) {
	var request jsonRPCRequest
	if err := json.Unmarshal(line, &request); err != nil {
		s.logger.Warn("invalid JSON-RPC payload", "error", err)
		response := s.errorResponse(nil, -32700, fmt.Sprintf("invalid JSON-RPC payload: %v", err))
		return json.Marshal(response)
	}

	if request.JSONRPC != "2.0" {
		s.logger.Warn("invalid JSON-RPC version", "version", request.JSONRPC)
		response := s.errorResponse(request.ID, -32600, "invalid JSON-RPC version")
		return json.Marshal(response)
	}

	s.logger.Debug("handling MCP request", "method", request.Method, "has_id", request.ID != nil)

	switch request.Method {
	case "initialize":
		return s.handleInitialize(request.ID)
	case "notifications/initialized", "notifications/cancelled":
		s.logger.Debug("ignoring MCP notification", "method", request.Method)
		return nil, nil
	case "ping":
		return s.respond(request.ID, map[string]any{})
	case "tools/list":
		return s.respond(request.ID, map[string]any{"tools": s.allToolDefinitions()})
	case "tools/call":
		return s.handleToolCall(request.ID, request.Params, extraHeaders)
	case "prompts/list":
		return s.respond(request.ID, map[string]any{"prompts": s.promptDefinitions()})
	case "prompts/get":
		return s.handlePromptGet(request.ID, request.Params)
	default:
		if request.ID == nil {
			s.logger.Debug("ignoring unknown notification without id", "method", request.Method)
			return nil, nil
		}
		s.logger.Warn("unknown MCP method", "method", request.Method)
		response := s.errorResponse(request.ID, -32601, fmt.Sprintf("method not found: %s", request.Method))
		return json.Marshal(response)
	}
}

func (s *server) handleInitialize(id any) ([]byte, error) {
	s.logger.Info("initializing MCP session")

	instructions := "Use getSwaggerDefinition to download a Swagger/OpenAPI document, then call the Swagger tools or the add-endpoint prompt to inspect the API and generate scaffolding."
	if s.cfg.ProxyMode || len(s.cfg.APIs) > 0 {
		instructions = "This server is running in proxy mode. API endpoints are available as tools — call them directly to interact with the API. " + instructions
	}

	result := map[string]any{
		"protocolVersion": latestProtocolVersion,
		"capabilities": map[string]any{
			"tools":   map[string]any{},
			"prompts": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "Swagger MCP Server",
			"version": config.Version,
		},
		"instructions": instructions,
	}
	return s.respond(id, result)
}

func (s *server) handleToolCall(id any, rawParams json.RawMessage, extraHeaders map[string]string) ([]byte, error) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		s.logger.Warn("invalid tool call parameters", "error", err)
		response := s.errorResponse(id, -32602, fmt.Sprintf("invalid tool parameters: %v", err))
		return json.Marshal(response)
	}

	arguments := params.Arguments
	if arguments == nil {
		arguments = map[string]any{}
	}

	s.logger.Debug("handling tool call", "tool", params.Name)
	result, err := s.callTool(params.Name, arguments, extraHeaders)
	if err != nil {
		s.logger.Warn("tool call failed", "tool", params.Name, "error", err)
		return s.respond(id, map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}, "isError": true})
	}
	s.logger.Debug("tool call completed", "tool", params.Name)
	return s.respond(id, result)
}

func (s *server) handlePromptGet(id any, rawParams json.RawMessage) ([]byte, error) {
	var params struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := json.Unmarshal(rawParams, &params); err != nil {
		s.logger.Warn("invalid prompt parameters", "error", err)
		response := s.errorResponse(id, -32602, fmt.Sprintf("invalid prompt parameters: %v", err))
		return json.Marshal(response)
	}

	s.logger.Debug("handling prompt request", "prompt", params.Name)
	result, err := s.getPrompt(params.Name, params.Arguments)
	if err != nil {
		s.logger.Warn("prompt request failed", "prompt", params.Name, "error", err)
		response := s.errorResponse(id, -32601, err.Error())
		return json.Marshal(response)
	}
	return s.respond(id, result)
}
