package openapi

import (
	"fmt"
	"sort"
	"strings"
)

type Endpoint struct {
	Path        string   `json:"path"`
	Method      string   `json:"method"`
	Summary     string   `json:"summary,omitempty"`
	Description string   `json:"description,omitempty"`
	OperationID string   `json:"operationId,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type Model struct {
	Name   string `json:"name"`
	Schema any    `json:"schema,omitempty"`
}

func ListEndpoints(document map[string]any) ([]Endpoint, error) {
	paths, ok := asMap(document["paths"])
	if !ok {
		return []Endpoint{}, nil
	}

	endpoints := make([]Endpoint, 0)
	supportedMethods := []string{"get", "post", "put", "delete", "patch", "options", "head"}
	for _, endpointPath := range sortedKeys(paths) {
		pathItem, ok := asMap(paths[endpointPath])
		if !ok {
			continue
		}
		for _, method := range supportedMethods {
			operation, ok := asMap(pathItem[method])
			if !ok {
				continue
			}
			endpoints = append(endpoints, Endpoint{
				Path:        endpointPath,
				Method:      strings.ToUpper(method),
				Summary:     stringValue(operation["summary"]),
				Description: stringValue(operation["description"]),
				OperationID: stringValue(operation["operationId"]),
				Tags:        stringSlice(operation["tags"]),
			})
		}
	}
	return endpoints, nil
}

func FindOperation(document map[string]any, endpointPath, method string) (map[string]any, error) {
	paths, ok := asMap(document["paths"])
	if !ok {
		return nil, fmt.Errorf("Swagger definition has no paths")
	}
	pathItem, ok := asMap(paths[endpointPath])
	if !ok {
		return nil, fmt.Errorf("endpoint path '%s' not found in Swagger definition", endpointPath)
	}
	operation, ok := asMap(pathItem[strings.ToLower(method)])
	if !ok {
		return nil, fmt.Errorf("invalid or unsupported HTTP method '%s' for endpoint path '%s'", method, endpointPath)
	}
	return operation, nil
}

func ListEndpointModels(document map[string]any, endpointPath, method string) ([]Model, error) {
	operation, err := FindOperation(document, endpointPath, method)
	if err != nil {
		return nil, err
	}

	models := make([]Model, 0)
	processedRefs := make(map[string]struct{})

	if parameters, ok := asSlice(operation["parameters"]); ok {
		for _, rawParameter := range parameters {
			parameter := DerefForCodegen(document, rawParameter)
			if schema, ok := asMap(parameter["schema"]); ok {
				extractReferencedModels(document, schema, processedRefs, &models)
			}
		}
	}

	if requestBody, ok := asMap(operation["requestBody"]); ok {
		if content, ok := asMap(requestBody["content"]); ok {
			for _, mediaType := range sortedKeys(content) {
				mediaValue, _ := asMap(content[mediaType])
				if schema, ok := asMap(mediaValue["schema"]); ok {
					extractReferencedModels(document, schema, processedRefs, &models)
				}
			}
		}
	}

	if responses, ok := asMap(operation["responses"]); ok {
		for _, statusCode := range sortedKeys(responses) {
			response := DerefForCodegen(document, responses[statusCode])
			if schema, ok := asMap(response["schema"]); ok {
				extractReferencedModels(document, schema, processedRefs, &models)
			}
			if content, ok := asMap(response["content"]); ok {
				for _, mediaType := range sortedKeys(content) {
					mediaValue, _ := asMap(content[mediaType])
					if schema, ok := asMap(mediaValue["schema"]); ok {
						extractReferencedModels(document, schema, processedRefs, &models)
					}
				}
			}
		}
	}

	return models, nil
}

func LookupSchema(document map[string]any, modelName string) (map[string]any, bool) {
	if definitions, ok := asMap(document["definitions"]); ok {
		if schema, ok := asMap(definitions[modelName]); ok {
			return schema, true
		}
	}
	if components, ok := asMap(document["components"]); ok {
		if schemas, ok := asMap(components["schemas"]); ok {
			if schema, ok := asMap(schemas[modelName]); ok {
				return schema, true
			}
		}
	}
	return nil, false
}

// SchemaDefinition represents a top-level model schema from the OpenAPI document.
type SchemaDefinition struct {
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"`
	Properties int    `json:"properties,omitempty"`
	Schema     any    `json:"schema,omitempty"`
}

// ListSchemas returns all top-level model schemas defined in the document.
// For Swagger 2.0 these are under "definitions"; for OpenAPI 3.x under "components.schemas".
func ListSchemas(document map[string]any) []SchemaDefinition {
	collect := func(defs map[string]any) []SchemaDefinition {
		result := make([]SchemaDefinition, 0, len(defs))
		for _, name := range sortedKeys(defs) {
			if def, ok := asMap(defs[name]); ok {
				sd := SchemaDefinition{
					Name:   name,
					Type:   stringValue(def["type"]),
					Schema: def,
				}
				if props, ok := asMap(def["properties"]); ok {
					sd.Properties = len(props)
				}
				result = append(result, sd)
			}
		}
		return result
	}

	// Swagger 2.0: definitions
	if definitions, ok := asMap(document["definitions"]); ok {
		return collect(definitions)
	}

	// OpenAPI 3.x: components.schemas
	if components, ok := asMap(document["components"]); ok {
		if schemas, ok := asMap(components["schemas"]); ok {
			return collect(schemas)
		}
	}

	return []SchemaDefinition{}
}

func ResolveRef(document map[string]any, ref string) (any, error) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, fmt.Errorf("unsupported reference %q", ref)
	}
	current := any(document)
	parts := strings.SplitSeq(strings.TrimPrefix(ref, "#/"), "/")
	for part := range parts {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		object, ok := asMap(current)
		if !ok {
			return nil, fmt.Errorf("reference %q does not point to an object", ref)
		}
		next, ok := object[part]
		if !ok {
			return nil, fmt.Errorf("reference %q not found", ref)
		}
		current = next
	}
	return current, nil
}

func DerefForCodegen(document map[string]any, value any) map[string]any {
	object, ok := asMap(value)
	if !ok {
		return map[string]any{}
	}
	if ref, ok := object["$ref"].(string); ok {
		resolved, err := ResolveRef(document, ref)
		if err == nil {
			if resolvedMap, ok := asMap(resolved); ok {
				return resolvedMap
			}
		}
	}
	return object
}

func extractReferencedModels(document map[string]any, schema map[string]any, processedRefs map[string]struct{}, models *[]Model) {
	if ref, ok := schema["$ref"].(string); ok {
		if _, seen := processedRefs[ref]; seen {
			return
		}
		processedRefs[ref] = struct{}{}
		modelName := ref[strings.LastIndex(ref, "/")+1:]
		resolved, err := ResolveRef(document, ref)
		if err == nil {
			*models = append(*models, Model{Name: modelName, Schema: resolved})
			if resolvedMap, ok := asMap(resolved); ok {
				extractReferencedModels(document, resolvedMap, processedRefs, models)
			}
		}
		return
	}

	if items, ok := asMap(schema["items"]); ok {
		extractReferencedModels(document, items, processedRefs, models)
	}
	if properties, ok := asMap(schema["properties"]); ok {
		for _, propertyName := range sortedKeys(properties) {
			if propertySchema, ok := asMap(properties[propertyName]); ok {
				extractReferencedModels(document, propertySchema, processedRefs, models)
			}
		}
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		if values, ok := asSlice(schema[keyword]); ok {
			for _, rawValue := range values {
				if valueMap, ok := asMap(rawValue); ok {
					extractReferencedModels(document, valueMap, processedRefs, models)
				}
			}
		}
	}
}

func asMap(value any) (map[string]any, bool) {
	mapped, ok := value.(map[string]any)
	return mapped, ok
}

func asSlice(value any) ([]any, bool) {
	slice, ok := value.([]any)
	return slice, ok
}

func sortedKeys(mapped map[string]any) []string {
	keys := make([]string, 0, len(mapped))
	for key := range mapped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stringValue(value any) string {
	stringValue, _ := value.(string)
	return stringValue
}

// StringValuePublic is the exported version of stringValue for use by other packages.
func StringValuePublic(value any) string {
	return stringValue(value)
}

func stringSlice(value any) []string {
	items, ok := asSlice(value)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// ExtractBaseURL returns the base URL of the API from the OpenAPI document.
// Swagger 2.0: constructs from host, basePath, and schemes.
// OpenAPI 3.x: returns the first server URL with variables resolved.
func ExtractBaseURL(document map[string]any) string {
	// OpenAPI 3.x: servers array
	if servers, ok := asSlice(document["servers"]); ok && len(servers) > 0 {
		if server, ok := asMap(servers[0]); ok {
			url := stringValue(server["url"])
			if variables, ok := asMap(server["variables"]); ok {
				for name, rawVar := range variables {
					if varObj, ok := asMap(rawVar); ok {
						defaultVal := stringValue(varObj["default"])
						if defaultVal != "" {
							url = strings.ReplaceAll(url, "{"+name+"}", defaultVal)
						}
					}
				}
			}
			return strings.TrimRight(url, "/")
		}
	}

	// Swagger 2.0: host + basePath + schemes
	host := stringValue(document["host"])
	if host == "" {
		return ""
	}
	basePath := stringValue(document["basePath"])
	scheme := "https"
	if schemes, ok := asSlice(document["schemes"]); ok && len(schemes) > 0 {
		if s := stringValue(schemes[0]); s != "" {
			scheme = s
		}
	}
	return strings.TrimRight(scheme+"://"+host+basePath, "/")
}

// ExtractBasePath returns the basePath from a Swagger 2.0 document (e.g. "/api/v1").
// Returns "" for OpenAPI 3.x documents, where the base path is embedded in server URLs.
func ExtractBasePath(document map[string]any) string {
	// OpenAPI 3.x uses servers array — basePath is already part of those URLs.
	if servers, ok := asSlice(document["servers"]); ok && len(servers) > 0 {
		return ""
	}
	bp := stringValue(document["basePath"])
	if bp == "/" {
		return ""
	}
	return bp
}

// SecurityScheme represents a parsed security scheme from the OpenAPI document.
type SecurityScheme struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`      // "apiKey", "http", "oauth2", "openIdConnect"
	In        string            `json:"in"`        // "header", "query", "cookie" (for apiKey)
	ParamName string            `json:"paramName"` // the header/query parameter name (for apiKey)
	Scheme    string            `json:"scheme"`    // "bearer", "basic" (for http)
	BearerFmt string            `json:"bearerFormat,omitempty"`
	FlowType  string            `json:"flowType,omitempty"` // "clientCredentials", "authorizationCode", etc.
	TokenURL  string            `json:"tokenUrl,omitempty"` // for oauth2
	AuthURL   string            `json:"authorizationUrl,omitempty"`
	Scopes    map[string]string `json:"scopes,omitempty"`
}

// SecurityRequirement maps scheme name to required scopes.
type SecurityRequirement struct {
	SchemeName string   `json:"schemeName"`
	Scopes     []string `json:"scopes,omitempty"`
}

// ExtractSecuritySchemes returns all security schemes defined in the document.
func ExtractSecuritySchemes(document map[string]any) []SecurityScheme {
	schemes := make([]SecurityScheme, 0)

	// Swagger 2.0: securityDefinitions
	if defs, ok := asMap(document["securityDefinitions"]); ok {
		for _, name := range sortedKeys(defs) {
			def, ok := asMap(defs[name])
			if !ok {
				continue
			}
			s := parseSecuritySchemeSwagger2(name, def)
			schemes = append(schemes, s)
		}
		return schemes
	}

	// OpenAPI 3.x: components.securitySchemes
	if components, ok := asMap(document["components"]); ok {
		if secSchemes, ok := asMap(components["securitySchemes"]); ok {
			for _, name := range sortedKeys(secSchemes) {
				def, ok := asMap(secSchemes[name])
				if !ok {
					continue
				}
				s := parseSecuritySchemeOpenAPI3(name, def)
				schemes = append(schemes, s)
			}
		}
	}
	return schemes
}

func parseSecuritySchemeSwagger2(name string, def map[string]any) SecurityScheme {
	s := SecurityScheme{Name: name, Type: stringValue(def["type"])}
	switch s.Type {
	case "apiKey":
		s.In = stringValue(def["in"])
		s.ParamName = stringValue(def["name"])
	case "basic":
		s.Type = "http"
		s.Scheme = "basic"
	case "oauth2":
		s.FlowType = stringValue(def["flow"])
		s.TokenURL = stringValue(def["tokenUrl"])
		s.AuthURL = stringValue(def["authorizationUrl"])
		if scopeMap, ok := asMap(def["scopes"]); ok {
			s.Scopes = make(map[string]string)
			for k := range scopeMap {
				s.Scopes[k] = stringValue(scopeMap[k])
			}
		}
	}
	return s
}

func parseSecuritySchemeOpenAPI3(name string, def map[string]any) SecurityScheme {
	s := SecurityScheme{Name: name, Type: stringValue(def["type"])}
	switch s.Type {
	case "apiKey":
		s.In = stringValue(def["in"])
		s.ParamName = stringValue(def["name"])
	case "http":
		s.Scheme = strings.ToLower(stringValue(def["scheme"]))
		s.BearerFmt = stringValue(def["bearerFormat"])
	case "oauth2":
		if flows, ok := asMap(def["flows"]); ok {
			for _, flowType := range []string{"clientCredentials", "authorizationCode", "implicit", "password"} {
				if flow, ok := asMap(flows[flowType]); ok {
					s.FlowType = flowType
					s.TokenURL = stringValue(flow["tokenUrl"])
					s.AuthURL = stringValue(flow["authorizationUrl"])
					if scopeMap, ok := asMap(flow["scopes"]); ok {
						s.Scopes = make(map[string]string)
						for k := range scopeMap {
							s.Scopes[k] = stringValue(scopeMap[k])
						}
					}
					break
				}
			}
		}
	}
	return s
}

// ExtractEndpointSecurity returns the security requirements for a specific operation.
// Falls back to global security if the operation doesn't define its own.
func ExtractEndpointSecurity(document map[string]any, endpointPath, method string) ([]SecurityRequirement, error) {
	operation, err := FindOperation(document, endpointPath, method)
	if err != nil {
		return nil, err
	}

	// Operation-level security takes precedence
	if security, ok := asSlice(operation["security"]); ok {
		return parseSecurityRequirements(security), nil
	}

	// Fall back to global security
	if security, ok := asSlice(document["security"]); ok {
		return parseSecurityRequirements(security), nil
	}

	return nil, nil
}

func parseSecurityRequirements(security []any) []SecurityRequirement {
	reqs := make([]SecurityRequirement, 0)
	for _, item := range security {
		if reqMap, ok := asMap(item); ok {
			for name := range reqMap {
				req := SecurityRequirement{SchemeName: name}
				if scopes, ok := asSlice(reqMap[name]); ok {
					for _, scope := range scopes {
						if s, ok := scope.(string); ok {
							req.Scopes = append(req.Scopes, s)
						}
					}
				}
				reqs = append(reqs, req)
			}
		}
	}
	return reqs
}
