package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/openapi"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpgoserver "github.com/mark3labs/mcp-go/server"
)

const (
	paramTypeString    = "string"
	paramTypeArray     = "array"
	paramLocationPath  = "path"
	paramLocationQuery = "query"

	httpClientTimeoutSeconds = 30
	httpErrorStatusThreshold = 400
)

// toolDefinition holds the MCP tool schema used for proxy tool registration and tests.
type toolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

// registerProxyTools registers dynamic proxy tools on the MCPServer.
func registerProxyTools(s *mcpgoserver.MCPServer, tools []proxyTool, cfg config.Config) {
	for _, pt := range tools {
		tool := buildMCPGoTool(pt)
		captured := pt
		s.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return executeProxyCall(ctx, captured, req.GetArguments(), cfg)
		})
	}
}

// buildMCPGoTool converts a proxyTool's definition into a mcp-go Tool.
func buildMCPGoTool(pt proxyTool) mcpgo.Tool {
	opts := []mcpgo.ToolOption{
		mcpgo.WithDescription(pt.Definition.Description),
		mcpgo.WithToolAnnotation(inferProxyAnnotations(pt.Method)),
	}

	if props, ok := pt.Definition.InputSchema["properties"].(map[string]any); ok {
		required := map[string]bool{}
		if reqList, reqOk := pt.Definition.InputSchema["required"].([]string); reqOk {
			for _, r := range reqList {
				required[r] = true
			}
		}
		for name, rawProp := range props {
			prop, _ := rawProp.(map[string]any)
			desc, _ := prop["description"].(string)
			propOpts := []mcpgo.PropertyOption{mcpgo.Description(desc)}
			if required[name] {
				propOpts = append(propOpts, mcpgo.Required())
			}
			opts = append(opts, mcpgo.WithString(name, propOpts...))
		}
	}

	return mcpgo.NewTool(pt.Definition.Name, opts...)
}

// inferProxyAnnotations returns tool annotations inferred from the HTTP method.
// GET/HEAD are read-only and idempotent; DELETE is destructive and idempotent;
// PUT is idempotent but not read-only; POST/PATCH are neither.
func inferProxyAnnotations(method string) mcpgo.ToolAnnotation {
	m := strings.ToUpper(method)
	readOnly := m == "GET" || m == "HEAD"
	destructive := m == "DELETE"
	idempotent := readOnly || m == "PUT" || m == "DELETE"
	return mcpgo.ToolAnnotation{
		ReadOnlyHint:    new(readOnly),
		DestructiveHint: new(destructive),
		IdempotentHint:  new(idempotent),
		OpenWorldHint:   new(true),
	}
}

type proxyTool struct {
	Definition toolDefinition
	Method     string
	Path       string
	Operation  map[string]any
	// Per-API context (populated when using multi-API config).
	APIName string
	BaseURL string
	Auth    config.AuthConfig
	Headers string
}

// buildProxyTools creates dynamic MCP tool definitions for each filtered endpoint.
// apiName prefixes tool names (used in multi-API mode); pass "" for single-API mode.
func buildProxyTools(
	document map[string]any,
	baseURL string,
	filter *openapi.EndpointFilter,
	apiName string,
	auth config.AuthConfig,
	headers string,
) ([]proxyTool, error) {
	endpoints, err := openapi.ListEndpoints(document)
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}

	endpoints = openapi.FilterEndpoints(endpoints, filter)
	tools := make([]proxyTool, 0, len(endpoints))

	for _, ep := range endpoints {
		operation, opErr := openapi.FindOperation(document, ep.Path, ep.Method)
		if opErr != nil {
			continue
		}

		toolName := proxyToolName(ep, apiName)
		description := proxyToolDescription(ep)
		inputSchema := proxyInputSchema(document, operation)

		tools = append(tools, proxyTool{
			Definition: toolDefinition{
				Name:        toolName,
				Description: description,
				InputSchema: inputSchema,
			},
			Method:    ep.Method,
			Path:      ep.Path,
			Operation: operation,
			APIName:   apiName,
			BaseURL:   baseURL,
			Auth:      auth,
			Headers:   headers,
		})
	}
	return tools, nil
}

func proxyToolName(ep openapi.Endpoint, apiName string) string {
	var base string
	if ep.OperationID != "" {
		name := strings.ReplaceAll(ep.OperationID, "_", "-")
		name = strings.ReplaceAll(name, " ", "-")
		base = strings.ToLower(name)
	} else {
		path := strings.ReplaceAll(ep.Path, "/", "-")
		path = strings.ReplaceAll(path, "{", "")
		path = strings.ReplaceAll(path, "}", "")
		path = strings.Trim(path, "-")
		base = strings.ToLower(ep.Method) + "-" + path
	}
	if apiName != "" {
		return apiName + "_" + base
	}
	return base
}

func proxyToolDescription(ep openapi.Endpoint) string {
	var b strings.Builder

	if ep.Summary != "" {
		b.WriteString(ep.Summary)
	}
	if ep.Description != "" {
		if b.Len() > 0 {
			b.WriteString(" — ")
		}
		b.WriteString(ep.Description)
	}
	if b.Len() == 0 {
		fmt.Fprintf(&b, "%s %s", ep.Method, ep.Path)
	}

	// Anti-hallucination instructions
	b.WriteString("\n\nIMPORTANT: Use this tool ONLY when the request exactly matches the description above. ")
	b.WriteString("If you don't have required parameters, always ask the user. ")
	b.WriteString("Do NOT fill any parameter on your own or keep it empty. ")
	b.WriteString("Do NOT maintain records in memory — always fetch fresh data from the API.")

	return b.String()
}

func proxyInputSchema(document map[string]any, operation map[string]any) map[string]any {
	properties := map[string]any{}
	required := []string{}

	if params, ok := operation["parameters"].([]any); ok {
		for _, rawParam := range params {
			param := openapi.DerefForCodegen(document, rawParam)
			name := openapi.StringValuePublic(param["name"])
			if name == "" {
				continue
			}
			in := openapi.StringValuePublic(param["in"])
			if in == "body" {
				processSwagger2BodyParam(document, param, name, properties, &required)
				continue
			}
			processRegularParam(param, name, in, properties, &required)
		}
	}

	if requestBody, ok := operation["requestBody"].(map[string]any); ok {
		processOAS3RequestBody(document, requestBody, properties, &required)
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func processSwagger2BodyParam(
	document, param map[string]any,
	name string,
	properties map[string]any,
	required *[]string,
) {
	schema, bodySchemaOk := param["schema"].(map[string]any)
	if !bodySchemaOk {
		return
	}
	resolved := openapi.DerefForCodegen(document, schema)
	if _, hasProps := resolved["properties"].(map[string]any); hasProps {
		addSchemaProperties(document, resolved, properties, required)
		return
	}
	paramType := openapi.StringValuePublic(param["type"])
	desc := openapi.StringValuePublic(param["description"])
	if paramType == "" {
		paramType = openapi.StringValuePublic(resolved["type"])
	}
	if paramType == "" {
		paramType = paramTypeString
	}
	if desc == "" {
		desc = "Request body"
	}
	propSchema := map[string]any{
		"type":        paramType,
		"description": desc,
	}
	if paramType == paramTypeArray {
		propSchema["items"] = extractResolvedArrayItems(resolved)
	}
	properties[name] = propSchema
	if isRequired, _ := param["required"].(bool); isRequired {
		*required = append(*required, name)
	}
}

func extractResolvedArrayItems(resolved map[string]any) map[string]any {
	items := map[string]any{"type": paramTypeString}
	if rawItems, itemsOk := resolved["items"].(map[string]any); itemsOk {
		if itemType := openapi.StringValuePublic(rawItems["type"]); itemType != "" {
			items = map[string]any{"type": itemType}
		}
	}
	return items
}

func processRegularParam(
	param map[string]any,
	name, in string,
	properties map[string]any,
	required *[]string,
) {
	paramType := openapi.StringValuePublic(param["type"])
	desc := openapi.StringValuePublic(param["description"])
	if schema, paramSchemaOk := param["schema"].(map[string]any); paramSchemaOk {
		if paramType == "" {
			paramType = openapi.StringValuePublic(schema["type"])
		}
		if desc == "" {
			desc = openapi.StringValuePublic(schema["description"])
		}
	}
	if paramType == "" {
		paramType = paramTypeString
	}
	if desc == "" {
		desc = fmt.Sprintf("%s parameter: %s", in, name)
	}
	propSchema := map[string]any{
		"type":        paramType,
		"description": desc,
	}
	if paramType == paramTypeArray {
		propSchema["items"] = extractParamArrayItems(param)
	}
	properties[name] = propSchema
	isRequired, _ := param["required"].(bool)
	if isRequired || in == paramLocationPath {
		*required = append(*required, name)
	}
}

func extractParamArrayItems(param map[string]any) map[string]any {
	items := map[string]any{"type": paramTypeString}
	if rawItems, itemsOk := param["items"].(map[string]any); itemsOk {
		if itemType := openapi.StringValuePublic(rawItems["type"]); itemType != "" {
			items = map[string]any{"type": itemType}
		}
	}
	if schema, schemaOk := param["schema"].(map[string]any); schemaOk {
		if rawItems, schemaItemsOk := schema["items"].(map[string]any); schemaItemsOk {
			if itemType := openapi.StringValuePublic(rawItems["type"]); itemType != "" {
				items = map[string]any{"type": itemType}
			}
		}
	}
	return items
}

func processOAS3RequestBody(
	document map[string]any,
	requestBody map[string]any,
	properties map[string]any,
	required *[]string,
) {
	content, contentOk := requestBody["content"].(map[string]any)
	if !contentOk {
		return
	}
	for _, mediaType := range []string{"application/json", "application/x-www-form-urlencoded"} {
		if media, mediaOk := content[mediaType].(map[string]any); mediaOk {
			if schema, schemaOk := media["schema"].(map[string]any); schemaOk {
				addSchemaProperties(document, schema, properties, required)
			}
			break
		}
	}
}

func addSchemaProperties(
	document map[string]any,
	schema map[string]any,
	properties map[string]any,
	required *[]string,
) {
	resolved := openapi.DerefForCodegen(document, schema)
	if props, ok := resolved["properties"].(map[string]any); ok {
		for name, rawProp := range props {
			properties[name] = buildBodyPropSchema(document, name, rawProp)
		}
	}
	if reqFields, ok := resolved["required"].([]any); ok {
		for _, r := range reqFields {
			if s, strOk := r.(string); strOk {
				if !containsString(*required, s) {
					*required = append(*required, s)
				}
			}
		}
	}
}

func buildBodyPropSchema(document map[string]any, name string, rawProp any) map[string]any {
	prop := openapi.DerefForCodegen(document, rawProp)
	propType := openapi.StringValuePublic(prop["type"])
	if propType == "" {
		propType = paramTypeString
	}
	desc := openapi.StringValuePublic(prop["description"])
	if desc == "" {
		desc = fmt.Sprintf("Body field: %s", name)
	}
	propSchema := map[string]any{
		"type":        propType,
		"description": desc,
	}
	if propType == paramTypeArray {
		items := map[string]any{"type": paramTypeString}
		if rawItems, rawItemsOk := prop["items"].(map[string]any); rawItemsOk {
			if itemType := openapi.StringValuePublic(rawItems["type"]); itemType != "" {
				items = map[string]any{"type": itemType}
			}
		}
		propSchema["items"] = items
	}
	return propSchema
}

func containsString(values []string, target string) bool {
	return slices.Contains(values, target)
}

// executeProxyCall executes an HTTP request to the target API for a proxy tool.
func executeProxyCall(
	ctx context.Context,
	tool proxyTool,
	arguments map[string]any,
	cfg config.Config,
) (*mcpgo.CallToolResult, error) {
	// Use per-tool base URL and auth when set (multi-API mode), otherwise fall back to global config.
	effectiveBaseURL := tool.BaseURL
	effectiveAuth := tool.Auth
	if effectiveAuth == (config.AuthConfig{}) {
		effectiveAuth = cfg.Auth
	}
	effectiveHeaders := tool.Headers
	if effectiveHeaders == "" {
		effectiveHeaders = cfg.Headers
	}

	// Extract extra headers injected by the transport layer.
	extraHeaders := ProxyHeadersFromContext(ctx)

	targetURL := buildProxyURL(tool.Path, effectiveBaseURL, tool.Operation, arguments)

	method := strings.ToUpper(tool.Method)
	body, err := buildBodyReader(tool.Operation, arguments, method)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil && body != http.NoBody {
		req.Header.Set("Content-Type", "application/json")
	}

	applyProxyAuth(req, effectiveAuth)
	applyCustomHeaders(req, effectiveHeaders)
	applyExtraHeaders(req, extraHeaders)
	applyHeaderParams(req, tool.Operation, arguments)

	client := &http.Client{Timeout: httpClientTimeoutSeconds * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= httpErrorStatusThreshold {
		return mcpgo.NewToolResultError(fmt.Sprintf("[Error] HTTP %d: %s", resp.StatusCode, string(respBody))), nil
	}

	return mcpgo.NewToolResultText(string(respBody)), nil
}

func buildBodyReader(operation, arguments map[string]any, method string) (io.Reader, error) {
	if !requiresBody(method) {
		return http.NoBody, nil
	}
	bodyData := buildRequestBody(operation, arguments)
	if bodyData == nil {
		return http.NoBody, nil
	}
	jsonBytes, err := json.Marshal(bodyData)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}
	return bytes.NewReader(jsonBytes), nil
}

func requiresBody(method string) bool {
	return method == "POST" || method == "PUT" || method == "PATCH"
}

func applyHeaderParams(req *http.Request, operation, arguments map[string]any) {
	params, ok := operation["parameters"].([]any)
	if !ok {
		return
	}
	for _, rawParam := range params {
		param, paramOk := rawParam.(map[string]any)
		if !paramOk {
			continue
		}
		if openapi.StringValuePublic(param["in"]) == "header" {
			name := openapi.StringValuePublic(param["name"])
			if v, valOk := arguments[name]; valOk {
				req.Header.Set(name, fmt.Sprint(v))
			}
		}
	}
}

func buildProxyURL(endpointPath, baseURL string, operation map[string]any, arguments map[string]any) string {
	path := substitutePathParams(endpointPath, operation, arguments)
	targetURL := strings.TrimRight(baseURL, "/") + path

	queryValues := buildQueryParams(operation, arguments)
	if encoded := queryValues.Encode(); encoded != "" {
		targetURL += "?" + encoded
	}

	return targetURL
}

func substitutePathParams(endpointPath string, operation, arguments map[string]any) string {
	path := endpointPath
	params, ok := operation["parameters"].([]any)
	if !ok {
		return path
	}
	for _, rawParam := range params {
		param, paramOk := rawParam.(map[string]any)
		if !paramOk {
			continue
		}
		if openapi.StringValuePublic(param["in"]) == paramLocationPath {
			name := openapi.StringValuePublic(param["name"])
			if v, valOk := arguments[name]; valOk {
				path = strings.ReplaceAll(path, "{"+name+"}", fmt.Sprint(v))
			}
		}
	}
	return path
}

func buildQueryParams(operation, arguments map[string]any) url.Values {
	queryValues := url.Values{}
	params, ok := operation["parameters"].([]any)
	if !ok {
		return queryValues
	}
	for _, rawParam := range params {
		param, paramOk := rawParam.(map[string]any)
		if !paramOk {
			continue
		}
		if openapi.StringValuePublic(param["in"]) == paramLocationQuery {
			name := openapi.StringValuePublic(param["name"])
			if v, valOk := arguments[name]; valOk {
				queryValues.Set(name, fmt.Sprint(v))
			}
		}
	}
	return queryValues
}

func buildRequestBody(operation map[string]any, arguments map[string]any) any {
	// OpenAPI 3.x requestBody; support explicit {"body": ...} and
	// {"requestBody": ...} arguments for compatibility with stricter clients.
	if _, hasRequestBody := operation["requestBody"].(map[string]any); hasRequestBody {
		if explicit, ok := extractExplicitBodyArg(arguments); ok {
			return explicit
		}
	}

	params, _ := operation["parameters"].([]any)
	return buildBodyFromParams(params, arguments)
}

func buildBodyFromParams(params []any, arguments map[string]any) any {
	locationParams := map[string]bool{}
	for _, rawParam := range params {
		param, paramOk := rawParam.(map[string]any)
		if !paramOk {
			continue
		}
		in := openapi.StringValuePublic(param["in"])
		name := openapi.StringValuePublic(param["name"])
		if isLocationParam(in) {
			locationParams[name] = true
			continue
		}
		if in == "body" {
			return handleSwagger2BodyParam(name, arguments, locationParams)
		}
	}
	return collectBodyParams(arguments, locationParams)
}

func isLocationParam(in string) bool {
	return in == paramLocationPath || in == paramLocationQuery || in == "header"
}

func handleSwagger2BodyParam(name string, arguments map[string]any, locationParams map[string]bool) any {
	if bodyArg, hasBodyArg := arguments[name]; hasBodyArg {
		return decodeBodyArg(bodyArg)
	}
	fallback := map[string]any{}
	for k, v := range arguments {
		if !locationParams[k] {
			fallback[k] = v
		}
	}
	if len(fallback) == 0 {
		return nil
	}
	return fallback
}

func collectBodyParams(arguments map[string]any, locationParams map[string]bool) any {
	bodyParams := map[string]any{}
	for k, v := range arguments {
		if !locationParams[k] {
			bodyParams[k] = v
		}
	}
	if len(bodyParams) == 0 {
		return nil
	}
	return bodyParams
}

func extractExplicitBodyArg(arguments map[string]any) (any, bool) {
	if bodyArg, ok := arguments["body"]; ok {
		return decodeBodyArg(bodyArg), true
	}
	if bodyArg, ok := arguments["requestBody"]; ok {
		return decodeBodyArg(bodyArg), true
	}
	return nil, false
}

func decodeBodyArg(bodyArg any) any {
	if raw, ok := bodyArg.(string); ok {
		var decoded any
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			return decoded
		}
	}
	return bodyArg
}

func applyProxyAuth(req *http.Request, auth config.AuthConfig) {
	if auth.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+auth.BearerToken)
		return
	}
	if auth.BasicUser != "" {
		req.SetBasicAuth(auth.BasicUser, auth.BasicPass)
		return
	}
	if auth.APIKey != "" {
		switch strings.ToLower(auth.APIKeyIn) {
		case "query":
			q := req.URL.Query()
			q.Set(auth.APIKeyHeader, auth.APIKey)
			req.URL.RawQuery = q.Encode()
		case "cookie":
			req.AddCookie(&http.Cookie{Name: auth.APIKeyHeader, Value: auth.APIKey})
		default:
			req.Header.Set(auth.APIKeyHeader, auth.APIKey)
		}
	}
}

func applyCustomHeaders(req *http.Request, headers string) {
	if headers == "" {
		return
	}
	for pair := range strings.SplitSeq(headers, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		req.Header.Set(strings.TrimSpace(key), strings.TrimSpace(value))
	}
}

// applyExtraHeaders sets headers forwarded from the transport layer (SSE or StreamableHTTP)
// to the proxy request. These override any previously set headers with the same name.
func applyExtraHeaders(req *http.Request, headers map[string]string) {
	for name, value := range headers {
		req.Header.Set(name, value)
	}
}
