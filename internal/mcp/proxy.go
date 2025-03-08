package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/openapi"
)

type proxyTool struct {
	Definition toolDefinition
	Method     string
	Path       string
	Operation  map[string]any
}

// buildProxyTools creates dynamic MCP tool definitions for each filtered endpoint.
func buildProxyTools(document map[string]any, baseURL string, filter *openapi.EndpointFilter) ([]proxyTool, error) {
	endpoints, err := openapi.ListEndpoints(document)
	if err != nil {
		return nil, fmt.Errorf("list endpoints: %w", err)
	}

	endpoints = openapi.FilterEndpoints(endpoints, filter)
	tools := make([]proxyTool, 0, len(endpoints))

	for _, ep := range endpoints {
		operation, err := openapi.FindOperation(document, ep.Path, ep.Method)
		if err != nil {
			continue
		}

		toolName := proxyToolName(ep)
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
		})
	}
	return tools, nil
}

func proxyToolName(ep openapi.Endpoint) string {
	if ep.OperationID != "" {
		name := strings.ReplaceAll(ep.OperationID, "_", "-")
		name = strings.ReplaceAll(name, " ", "-")
		return strings.ToLower(name)
	}
	path := strings.ReplaceAll(ep.Path, "/", "-")
	path = strings.ReplaceAll(path, "{", "")
	path = strings.ReplaceAll(path, "}", "")
	path = strings.Trim(path, "-")
	return strings.ToLower(ep.Method) + "-" + path
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
		b.WriteString(fmt.Sprintf("%s %s", ep.Method, ep.Path))
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

	// Collect parameters (path, query, header)
	if params, ok := operation["parameters"].([]any); ok {
		for _, rawParam := range params {
			param := openapi.DerefForCodegen(document, rawParam)
			name := openapi.StringValuePublic(param["name"])
			if name == "" {
				continue
			}
			in := openapi.StringValuePublic(param["in"])
			paramType := openapi.StringValuePublic(param["type"])
			desc := openapi.StringValuePublic(param["description"])

			if in == "body" {
				if schema, ok := param["schema"].(map[string]any); ok {
					resolved := openapi.DerefForCodegen(document, schema)
					if _, hasProps := resolved["properties"].(map[string]any); hasProps {
						addSchemaProperties(document, resolved, properties, &required)
					} else {
						if paramType == "" {
							paramType = openapi.StringValuePublic(resolved["type"])
						}
						if paramType == "" {
							paramType = "string"
						}
						if desc == "" {
							desc = "Request body"
						}
						propSchema := map[string]any{
							"type":        paramType,
							"description": desc,
						}
						if paramType == "array" {
							items := map[string]any{"type": "string"}
							if rawItems, ok := resolved["items"].(map[string]any); ok {
								if itemType := openapi.StringValuePublic(rawItems["type"]); itemType != "" {
									items = map[string]any{"type": itemType}
								}
							}
							propSchema["items"] = items
						}
						properties[name] = propSchema
						if isRequired, _ := param["required"].(bool); isRequired {
							required = append(required, name)
						}
					}
				}
				continue
			}

			if schema, ok := param["schema"].(map[string]any); ok {
				if paramType == "" {
					paramType = openapi.StringValuePublic(schema["type"])
				}
				if desc == "" {
					desc = openapi.StringValuePublic(schema["description"])
				}
			}

			if paramType == "" {
				paramType = "string"
			}
			if desc == "" {
				desc = fmt.Sprintf("%s parameter: %s", in, name)
			}

			propSchema := map[string]any{
				"type":        paramType,
				"description": desc,
			}
			if paramType == "array" {
				items := map[string]any{"type": "string"}
				if rawItems, ok := param["items"].(map[string]any); ok {
					if itemType := openapi.StringValuePublic(rawItems["type"]); itemType != "" {
						items = map[string]any{"type": itemType}
					}
				}
				if schema, ok := param["schema"].(map[string]any); ok {
					if rawItems, ok := schema["items"].(map[string]any); ok {
						if itemType := openapi.StringValuePublic(rawItems["type"]); itemType != "" {
							items = map[string]any{"type": itemType}
						}
					}
				}
				propSchema["items"] = items
			}
			properties[name] = propSchema

			isRequired, _ := param["required"].(bool)
			if isRequired || in == "path" {
				required = append(required, name)
			}
		}
	}

	// OpenAPI 3.x requestBody
	if requestBody, ok := operation["requestBody"].(map[string]any); ok {
		if content, ok := requestBody["content"].(map[string]any); ok {
			for _, mediaType := range []string{"application/json", "application/x-www-form-urlencoded"} {
				if media, ok := content[mediaType].(map[string]any); ok {
					if schema, ok := media["schema"].(map[string]any); ok {
						addSchemaProperties(document, schema, properties, &required)
					}
					break
				}
			}
		}
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

func addSchemaProperties(document map[string]any, schema map[string]any, properties map[string]any, required *[]string) {
	resolved := openapi.DerefForCodegen(document, schema)
	if props, ok := resolved["properties"].(map[string]any); ok {
		for name, rawProp := range props {
			prop := openapi.DerefForCodegen(document, rawProp)
			propType := openapi.StringValuePublic(prop["type"])
			if propType == "" {
				propType = "string"
			}
			desc := openapi.StringValuePublic(prop["description"])
			if desc == "" {
				desc = fmt.Sprintf("Body field: %s", name)
			}
			propSchema := map[string]any{
				"type":        propType,
				"description": desc,
			}
			if propType == "array" {
				items := map[string]any{"type": "string"}
				if rawItems, ok := prop["items"].(map[string]any); ok {
					if itemType := openapi.StringValuePublic(rawItems["type"]); itemType != "" {
						items = map[string]any{"type": itemType}
					}
				}
				propSchema["items"] = items
			}
			properties[name] = propSchema
		}
	}
	if reqFields, ok := resolved["required"].([]any); ok {
		for _, r := range reqFields {
			if s, ok := r.(string); ok {
				if !containsString(*required, s) {
					*required = append(*required, s)
				}
			}
		}
	}
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

// executeProxyCall executes an HTTP request to the target API for a proxy tool.
func executeProxyCall(tool proxyTool, arguments map[string]any, baseURL string, cfg config.Config, extraHeaders map[string]string) (map[string]any, error) {
	targetURL, err := buildProxyURL(tool.Path, baseURL, tool.Operation, arguments)
	if err != nil {
		return nil, fmt.Errorf("build URL: %w", err)
	}

	method := strings.ToUpper(tool.Method)
	var body io.Reader
	if method == "POST" || method == "PUT" || method == "PATCH" {
		bodyData := buildRequestBody(tool.Operation, arguments)
		if bodyData != nil {
			jsonBytes, err := json.Marshal(bodyData)
			if err != nil {
				return nil, fmt.Errorf("marshal request body: %w", err)
			}
			body = bytes.NewReader(jsonBytes)
		}
	}

	req, err := http.NewRequest(method, targetURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	applyProxyAuth(req, cfg.Auth)
	applyCustomHeaders(req, cfg.Headers)
	applyExtraHeaders(req, extraHeaders)

	// Add header parameters from arguments
	if params, ok := tool.Operation["parameters"].([]any); ok {
		for _, rawParam := range params {
			param, ok := rawParam.(map[string]any)
			if !ok {
				continue
			}
			if openapi.StringValuePublic(param["in"]) == "header" {
				name := openapi.StringValuePublic(param["name"])
				if v, ok := arguments[name]; ok {
					req.Header.Set(name, fmt.Sprint(v))
				}
			}
		}
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return textResult(fmt.Sprintf("[Error] HTTP %d: %s", resp.StatusCode, string(respBody)), true), nil
	}

	return textResult(string(respBody), false), nil
}

func buildProxyURL(endpointPath, baseURL string, operation map[string]any, arguments map[string]any) (string, error) {
	path := endpointPath

	// Substitute path parameters
	if params, ok := operation["parameters"].([]any); ok {
		for _, rawParam := range params {
			param, ok := rawParam.(map[string]any)
			if !ok {
				continue
			}
			if openapi.StringValuePublic(param["in"]) == "path" {
				name := openapi.StringValuePublic(param["name"])
				if v, ok := arguments[name]; ok {
					path = strings.ReplaceAll(path, "{"+name+"}", fmt.Sprint(v))
				}
			}
		}
	}

	targetURL := strings.TrimRight(baseURL, "/") + path

	// Add query parameters
	queryValues := url.Values{}
	if params, ok := operation["parameters"].([]any); ok {
		for _, rawParam := range params {
			param, ok := rawParam.(map[string]any)
			if !ok {
				continue
			}
			if openapi.StringValuePublic(param["in"]) == "query" {
				name := openapi.StringValuePublic(param["name"])
				if v, ok := arguments[name]; ok {
					queryValues.Set(name, fmt.Sprint(v))
				}
			}
		}
	}
	if encoded := queryValues.Encode(); encoded != "" {
		targetURL += "?" + encoded
	}

	return targetURL, nil
}

func buildRequestBody(operation map[string]any, arguments map[string]any) any {
	bodyParams := map[string]any{}

	// OpenAPI 3.x requestBody; support explicit {"body": ...} and
	// {"requestBody": ...} arguments for compatibility with stricter clients.
	if _, hasRequestBody := operation["requestBody"].(map[string]any); hasRequestBody {
		if explicit, ok := extractExplicitBodyArg(arguments); ok {
			return explicit
		}
	}

	// Collect non-path, non-query, non-header parameters as body
	pathAndQueryParams := map[string]bool{}
	if params, ok := operation["parameters"].([]any); ok {
		for _, rawParam := range params {
			param, ok := rawParam.(map[string]any)
			if !ok {
				continue
			}
			in := openapi.StringValuePublic(param["in"])
			name := openapi.StringValuePublic(param["name"])
			if in == "path" || in == "query" || in == "header" {
				pathAndQueryParams[name] = true
			}
			if in == "body" {
				// Swagger 2.0 body parameter; support explicit {"body": ...} and flattened fields.
				if bodyArg, hasBodyArg := arguments[name]; hasBodyArg {
					return decodeBodyArg(bodyArg)
				}

				fallback := map[string]any{}
				for k, v := range arguments {
					if !pathAndQueryParams[k] {
						fallback[k] = v
					}
				}
				if len(fallback) == 0 {
					return nil
				}
				return fallback
			}
		}
	}

	// For OpenAPI 3.x or remaining args, include non-path/query/header params
	for k, v := range arguments {
		if !pathAndQueryParams[k] {
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
	for _, pair := range strings.Split(headers, ",") {
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
