package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/openapi"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpgoserver "github.com/mark3labs/mcp-go/server"
)

const (
	paramTypeString    = "string"
	paramTypeBoolean   = "boolean"
	paramTypeArray     = "array"
	paramTypeInteger   = "integer"
	paramTypeNumber    = "number"
	paramTypeObject    = "object"
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

type proxyResponseResult struct {
	StatusCode  int    `json:"status_code"`
	ContentType string `json:"content_type,omitempty"`
	Body        any    `json:"body,omitempty"`
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
	schemaBytes, err := json.Marshal(pt.Definition.InputSchema)
	if err != nil {
		tool := mcpgo.NewTool(pt.Definition.Name, mcpgo.WithDescription(pt.Definition.Description))
		tool.Annotations = inferProxyAnnotations(pt.Method)
		return tool
	}
	tool := mcpgo.NewToolWithRawSchema(pt.Definition.Name, pt.Definition.Description, schemaBytes)
	mcpgo.WithOutputSchema[proxyResponseResult]()(&tool)
	tool.Annotations = inferProxyAnnotations(pt.Method)
	return tool
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
			slog.Warn("skipping endpoint: operation not found", "path", ep.Path, "method", ep.Method)
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
	prefix := normalizedToolPrefix(apiName)
	base := toolBaseName(ep)
	if prefix == "" {
		prefix = "swagger"
	}
	return prefix + "_" + base
}

func normalizedToolPrefix(apiName string) string {
	return toSnakeCase(apiName)
}

func toolBaseName(ep openapi.Endpoint) string {
	if ep.OperationID != "" {
		return toSnakeCase(ep.OperationID)
	}
	return pathToolBaseName(ep.Method, ep.Path)
}

func pathToolBaseName(method, endpointPath string) string {
	path := strings.Trim(strings.ReplaceAll(endpointPath, "/", "_"), "_")
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	base := method
	if path != "" {
		base += "_" + path
	}
	return toSnakeCase(base)
}

func toSnakeCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	runes := []rune(value)
	var b strings.Builder
	lastUnderscore := false

	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			if b.Len() > 0 && !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
			continue
		}

		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if (unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower)) &&
					b.Len() > 0 && !lastUnderscore {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			lastUnderscore = false
			continue
		}

		b.WriteRune(unicode.ToLower(r))
		lastUnderscore = false
	}

	return strings.Trim(b.String(), "_")
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
			processRegularParam(document, param, name, in, properties, &required)
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
	desc := openapi.StringValuePublic(param["description"])
	if desc == "" {
		desc = "Request body"
	}
	properties[name] = normalizeSchema(document, resolved, desc)
	if isRequired, _ := param["required"].(bool); isRequired {
		appendRequiredUnique(required, name)
	}
}

func processRegularParam(
	document map[string]any,
	param map[string]any,
	name, in string,
	properties map[string]any,
	required *[]string,
) {
	desc := openapi.StringValuePublic(param["description"])
	if desc == "" {
		desc = fmt.Sprintf("%s parameter: %s", in, name)
	}
	schemaObj, hasSchema := param["schema"].(map[string]any)
	if hasSchema {
		properties[name] = normalizeSchema(document, schemaObj, desc)
	} else {
		properties[name] = normalizeSchema(document, param, desc)
	}
	isRequired, _ := param["required"].(bool)
	if isRequired || in == paramLocationPath {
		appendRequiredUnique(required, name)
	}
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
			rawSchema, _ := media["schema"].(map[string]any)
			if rawSchema != nil {
				resolved := openapi.DerefForCodegen(document, rawSchema)
				if resolved != nil {
					rawSchema = resolved
				}
				normalized := normalizeSchema(document, rawSchema, "Request body")
				if normalized["type"] == paramTypeObject {
					if propMap, ok := normalized["properties"].(map[string]any); ok {
						for name, rawProp := range propMap {
							properties[name] = rawProp
						}
					}
					if reqFields, ok := normalized["required"].([]string); ok {
						for _, field := range reqFields {
							appendRequiredUnique(required, field)
						}
					}
					break
				}
				properties["requestBody"] = normalized
				if bodyRequired, _ := requestBody["required"].(bool); bodyRequired {
					appendRequiredUnique(required, "requestBody")
				}
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
				appendRequiredUnique(required, s)
			}
		}
	}
}

func buildBodyPropSchema(document map[string]any, name string, rawProp any) map[string]any {
	return normalizeSchema(document, rawProp, fmt.Sprintf("Body field: %s", name))
}

func containsString(values []string, target string) bool {
	return slices.Contains(values, target)
}

func appendRequiredUnique(required *[]string, name string) {
	if name == "" || containsString(*required, name) {
		return
	}
	*required = append(*required, name)
}

func normalizeSchema(document map[string]any, rawSchema any, fallbackDescription string) map[string]any {
	resolved := openapi.DerefForCodegen(document, rawSchema)
	schema := map[string]any{}

	desc := openapi.StringValuePublic(resolved["description"])
	if desc == "" {
		desc = fallbackDescription
	}
	if desc != "" {
		schema["description"] = desc
	}

	schemaType := openapi.StringValuePublic(resolved["type"])
	if schemaType == "" {
		if _, ok := resolved["properties"].(map[string]any); ok {
			schemaType = paramTypeObject
		} else if _, ok := resolved["items"].(map[string]any); ok {
			schemaType = paramTypeArray
		} else {
			schemaType = paramTypeString
		}
	}
	schema["type"] = schemaType

	for _, key := range []string{
		"format", "pattern", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum",
		"minLength", "maxLength", "default", "example", "collectionFormat",
	} {
		copySchemaValue(schema, resolved, key)
	}
	if enumValues, ok := resolved["enum"].([]any); ok && len(enumValues) > 0 {
		schema["enum"] = enumValues
	}

	switch schemaType {
	case paramTypeArray:
		if rawItems, ok := resolved["items"]; ok {
			schema["items"] = normalizeSchema(document, rawItems, "")
		} else {
			schema["items"] = map[string]any{"type": paramTypeString}
		}
	case paramTypeObject:
		if props, ok := resolved["properties"].(map[string]any); ok {
			nested := make(map[string]any, len(props))
			for name, rawProp := range props {
				nested[name] = normalizeSchema(document, rawProp, fmt.Sprintf("Body field: %s", name))
			}
			schema["properties"] = nested
		}
		if reqFields := requiredFieldsFromSchema(resolved); len(reqFields) > 0 {
			schema["required"] = reqFields
		}
	case paramTypeBoolean, paramTypeInteger, paramTypeNumber, paramTypeString:
	default:
		schema["type"] = paramTypeString
	}

	return schema
}

func copySchemaValue(dst, src map[string]any, key string) {
	if value, ok := src[key]; ok {
		dst[key] = value
	}
}

func requiredFieldsFromSchema(schema map[string]any) []string {
	switch reqFields := schema["required"].(type) {
	case []string:
		if len(reqFields) == 0 {
			return nil
		}
		return append([]string(nil), reqFields...)
	case []any:
		required := make([]string, 0, len(reqFields))
		for _, field := range reqFields {
			name, ok := field.(string)
			if ok && name != "" {
				required = append(required, name)
			}
		}
		return required
	default:
		return nil
	}
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
		slog.Error("HTTP proxy request failed", "path", tool.Path, "method", tool.Method, "error", err)
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

	return newProxyResult(resp.StatusCode, resp.Header.Get("Content-Type"), respBody), nil
}

func newProxyResult(statusCode int, contentType string, body []byte) *mcpgo.CallToolResult {
	parsed, formatted, ok := parseJSONToolBody(body)
	if !ok {
		return mcpgo.NewToolResultText(string(body))
	}

	return mcpgo.NewToolResultStructured(proxyResponseResult{
		StatusCode:  statusCode,
		ContentType: contentType,
		Body:        parsed,
	}, formatted)
}

func parseJSONToolBody(body []byte) (any, string, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, "", false
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return nil, "", false
	}

	var parsed any
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return nil, "", false
	}

	formatted := string(trimmed)
	if pretty, err := json.MarshalIndent(parsed, "", "  "); err == nil {
		formatted = string(pretty)
	}
	return parsed, formatted, true
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
