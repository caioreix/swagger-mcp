package codegen

import (
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"

	"github.com/caioreix/swagger-mcp/internal/openapi"
)

type GenerateEndpointToolCodeParams struct {
	Path                    string
	Method                  string
	IncludeAPIInName        bool
	IncludeVersionInName    bool
	SingularizeResourceName bool
}

type ValidationResult struct {
	IsValid bool
	Errors  []string
}

func GenerateEndpointToolCode(document map[string]any, params GenerateEndpointToolCodeParams) (string, error) {
	if strings.TrimSpace(params.Path) == "" {
		return "", fmt.Errorf("path parameter is required")
	}
	if strings.TrimSpace(params.Method) == "" {
		return "", fmt.Errorf("method parameter is required")
	}

	operation, err := openapi.FindOperation(document, params.Path, params.Method)
	if err != nil {
		return "", err
	}

	toolName := generateToolName(
		params.Method,
		params.Path,
		stringValue(operation["operationId"]),
		params.IncludeAPIInName,
		params.IncludeVersionInName,
		params.SingularizeResourceName,
	)
	inputSchema := generateInputSchema(document, operation)
	generatedCode := generateToolDefinition(toolName, params.Method, params.Path, operation, inputSchema) + "\n\n" + generateHandlerFunction(toolName, params.Method, params.Path)

	validation := ValidateToolCode(generatedCode)
	if !validation.IsValid {
		return FormatValidationErrors(validation.Errors), nil
	}
	return generatedCode, nil
}

func ValidateToolCode(toolCode string) ValidationResult {
	errors := make([]string, 0)
	if !strings.Contains(toolCode, "Name:") {
		errors = append(errors, "missing \"Name\" field in tool definition")
	}
	if !strings.Contains(toolCode, "Description:") {
		errors = append(errors, "missing \"Description\" field in tool definition")
	}
	if !strings.Contains(toolCode, "InputSchema:") {
		errors = append(errors, "missing \"InputSchema\" field in tool definition")
	}
	if !strings.Contains(toolCode, `"type": "object"`) {
		errors = append(errors, "missing or incorrect input schema type (must be object)")
	}
	if !strings.Contains(toolCode, "func Handle") {
		errors = append(errors, "missing handler function in tool definition")
	}
	return ValidationResult{IsValid: len(errors) == 0, Errors: errors}
}

func FormatValidationErrors(errors []string) string {
	return "\nMCP Schema Validation Failed\n============================\n\nThe generated tool definition does not comply with the MCP schema.\nPlease fix the following issues:\n\n- " + strings.Join(errors, "\n- ") + "\n"
}

func generateToolName(method, endpointPath, operationID string, includeAPIInName, includeVersionInName, singularizeResourceNames bool) string {
	methodPrefix := map[string]string{
		"GET":    "get",
		"POST":   "create",
		"PUT":    "update",
		"PATCH":  "update",
		"DELETE": "delete",
	}[strings.ToUpper(method)]
	if methodPrefix == "" {
		methodPrefix = strings.ToLower(method)
	}

	methodPrefixShort := map[string]string{
		"GET":    "get",
		"POST":   "crt",
		"PUT":    "upd",
		"PATCH":  "upd",
		"DELETE": "del",
	}[strings.ToUpper(method)]
	if methodPrefixShort == "" {
		methodPrefixShort = strings.ToLower(method)
	}

	if operationID != "" && !strings.Contains(operationID, "_") && !strings.Contains(operationID, ".") {
		if len(operationID) > 64 {
			return operationID[:64]
		}
		return operationID
	}

	abbreviationMap := map[string]string{
		"organization":   "org",
		"organizations":  "orgs",
		"generate":       "gen",
		"information":    "info",
		"application":    "app",
		"applications":   "apps",
		"identification": "id",
		"parameter":      "param",
		"parameters":     "params",
		"report":         "rpt",
		"configuration":  "config",
		"administrator":  "admin",
		"authentication": "auth",
		"authorization":  "authz",
		"notification":   "notice",
		"notifications":  "notices",
		"document":       "doc",
		"documents":      "docs",
		"category":       "cat",
		"categories":     "cats",
		"subscription":   "sub",
		"subscriptions":  "subs",
		"preference":     "pref",
		"preferences":    "prefs",
		"message":        "msg",
		"messages":       "msgs",
		"profile":        "prof",
		"profiles":       "profs",
		"setting":        "set",
		"settings":       "sets",
	}

	cleanPath := endpointPath
	if index := strings.Index(cleanPath, "?"); index >= 0 {
		cleanPath = cleanPath[:index]
	}
	if dot := strings.LastIndex(cleanPath, "."); dot > strings.LastIndex(cleanPath, "/") {
		cleanPath = cleanPath[:dot]
	}

	segments := make([]string, 0)
	for segment := range strings.SplitSeq(cleanPath, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}

	processedSegments := make([]string, 0, len(segments))
	for index, segment := range segments {
		lower := strings.ToLower(segment)
		if lower == "api" && !includeAPIInName {
			continue
		}
		if len(lower) > 1 && lower[0] == 'v' && isNumeric(lower[1:]) && !includeVersionInName {
			continue
		}
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			parameterName := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			processedSegments = append(processedSegments, strings.Title(parameterName))
			continue
		}

		processed := segment
		if singularizeResourceNames && index == len(segments)-1 && strings.HasSuffix(processed, "s") {
			if before, ok := strings.CutSuffix(processed, "ies"); ok {
				processed = before + "y"
			} else {
				processed = strings.TrimSuffix(processed, "s")
			}
		}
		if abbreviation, ok := abbreviationMap[strings.ToLower(processed)]; ok {
			processed = abbreviation
		}
		processedSegments = append(processedSegments, strings.Title(processed))
	}

	toolName := methodPrefix + strings.Join(processedSegments, "")
	if len(toolName) <= 64 {
		return toolName
	}

	reduced := methodPrefixShort
	for _, segment := range processedSegments {
		if len(reduced+segment) > 64 {
			break
		}
		reduced += segment
	}
	return reduced
}

func generateInputSchema(document map[string]any, operation map[string]any) map[string]any {
	inputSchema := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []string{},
	}
	properties := inputSchema["properties"].(map[string]any)
	required := make([]string, 0)

	if parameters, ok := operation["parameters"].([]any); ok {
		for _, rawParameter := range parameters {
			parameter := openapi.DerefForCodegen(document, rawParameter)
			location := strings.ToLower(stringValue(parameter["in"]))
			if location == "header" || location == "formdata" || location == "cookie" {
				continue
			}

			switch location {
			case "path", "query":
				name := stringValue(parameter["name"])
				propertySchema := parameterSchema(document, parameter)
				if description := stringValue(parameter["description"]); description != "" {
					propertySchema["description"] = description
				}
				properties[name] = propertySchema
				if boolFromAny(parameter["required"]) {
					required = append(required, name)
				}
			case "body":
				schema, ok := parameter["schema"].(map[string]any)
				if !ok {
					continue
				}
				name := sanitizeParameterName(stringValue(parameter["name"]))
				propertySchema := processSchema(document, schema)
				if description := stringValue(parameter["description"]); description != "" {
					propertySchema["description"] = description
				}
				properties[name] = propertySchema
				if boolFromAny(parameter["required"]) {
					required = append(required, name)
				}
			}
		}
	}

	if requestBody, ok := operation["requestBody"].(map[string]any); ok {
		if content, ok := requestBody["content"].(map[string]any); ok {
			mediaTypes := sortedMapKeys(content)
			selectedMediaType := ""
			if _, ok := content["application/json"]; ok {
				selectedMediaType = "application/json"
			} else if len(mediaTypes) > 0 {
				selectedMediaType = mediaTypes[0]
			}
			if selectedMediaType != "" {
				mediaValue, _ := content[selectedMediaType].(map[string]any)
				if schema, ok := mediaValue["schema"].(map[string]any); ok {
					propertySchema := processSchema(document, schema)
					if description := stringValue(requestBody["description"]); description != "" {
						propertySchema["description"] = description
					} else {
						propertySchema["description"] = "Request body"
					}
					properties["requestBody"] = propertySchema
					if boolFromAny(requestBody["required"]) {
						required = append(required, "requestBody")
					}
				}
			}
		}
	}

	sort.Strings(required)
	inputSchema["required"] = required
	return inputSchema
}

func parameterSchema(document map[string]any, parameter map[string]any) map[string]any {
	if schema, ok := parameter["schema"].(map[string]any); ok {
		return processSchema(document, schema)
	}

	propertySchema := map[string]any{
		"type": mapSwaggerTypeToJSONSchema(stringValue(parameter["type"])),
	}
	if items, ok := parameter["items"].(map[string]any); ok {
		propertySchema["items"] = processSchema(document, items)
	} else if mapSwaggerTypeToJSONSchema(stringValue(parameter["type"])) == "array" {
		propertySchema["items"] = map[string]any{"type": "string"}
	}
	if enumValues, ok := parameter["enum"].([]any); ok {
		propertySchema["enum"] = enumValues
	}
	if format := stringValue(parameter["format"]); format != "" {
		propertySchema["format"] = format
	}
	return propertySchema
}

func processSchema(document map[string]any, schema map[string]any) map[string]any {
	if schema == nil {
		return map[string]any{"type": "object"}
	}

	if ref, ok := schema["$ref"].(string); ok {
		modelName := ref[strings.LastIndex(ref, "/")+1:]
		if specialSchema, ok := processSpecialRef(document, modelName); ok {
			return specialSchema
		}
		return extractModelSchema(document, modelName)
	}

	if allOfValues, ok := schema["allOf"].([]any); ok {
		merged := map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}}
		mergedProperties := merged["properties"].(map[string]any)
		requiredSet := make(map[string]struct{})
		for _, rawValue := range allOfValues {
			subSchema, ok := rawValue.(map[string]any)
			if !ok {
				continue
			}
			processed := processSchema(document, subSchema)
			if properties, ok := processed["properties"].(map[string]any); ok {
				maps.Copy(mergedProperties, properties)
			}
			if requiredValues, ok := processed["required"].([]string); ok {
				for _, value := range requiredValues {
					requiredSet[value] = struct{}{}
				}
			}
		}
		required := make([]string, 0, len(requiredSet))
		for value := range requiredSet {
			required = append(required, value)
		}
		sort.Strings(required)
		merged["required"] = required
		return merged
	}

	if anyOfValues, ok := schema["anyOf"].([]any); ok && len(anyOfValues) > 0 {
		options := make([]any, 0, len(anyOfValues))
		for _, rawValue := range anyOfValues {
			if subSchema, ok := rawValue.(map[string]any); ok {
				options = append(options, processSchema(document, subSchema))
			}
		}
		return map[string]any{"anyOf": options}
	}

	if oneOfValues, ok := schema["oneOf"].([]any); ok && len(oneOfValues) > 0 {
		options := make([]any, 0, len(oneOfValues))
		for _, rawValue := range oneOfValues {
			if subSchema, ok := rawValue.(map[string]any); ok {
				options = append(options, processSchema(document, subSchema))
			}
		}
		return map[string]any{"oneOf": options}
	}

	schemaType := stringValue(schema["type"])
	switch schemaType {
	case "array":
		result := map[string]any{"type": "array"}
		if items, ok := schema["items"].(map[string]any); ok {
			result["items"] = processSchema(document, items)
		} else {
			result["items"] = map[string]any{"type": "string"}
		}
		if description := stringValue(schema["description"]); description != "" {
			result["description"] = description
		}
		return result
	case "object":
		result := map[string]any{"type": "object", "properties": map[string]any{}, "required": []string{}}
		if properties, ok := schema["properties"].(map[string]any); ok {
			renderedProperties := result["properties"].(map[string]any)
			for _, propertyName := range sortedMapKeys(properties) {
				propertySchema, _ := properties[propertyName].(map[string]any)
				renderedProperties[propertyName] = processSchema(document, propertySchema)
			}
		}
		if requiredValues, ok := schema["required"].([]any); ok {
			required := make([]string, 0, len(requiredValues))
			for _, rawValue := range requiredValues {
				if value, ok := rawValue.(string); ok {
					required = append(required, value)
				}
			}
			result["required"] = required
		}
		if additional, ok := schema["additionalProperties"].(map[string]any); ok {
			result["additionalProperties"] = processSchema(document, additional)
		}
		if description := stringValue(schema["description"]); description != "" {
			result["description"] = description
		}
		return result
	default:
		if properties, ok := schema["properties"].(map[string]any); ok && len(properties) > 0 {
			copied := map[string]any{"type": "object", "properties": properties, "required": schema["required"]}
			if description := stringValue(schema["description"]); description != "" {
				copied["description"] = description
			}
			return processSchema(document, copied)
		}
		result := map[string]any{"type": mapSwaggerTypeToJSONSchema(schemaType)}
		if description := stringValue(schema["description"]); description != "" {
			result["description"] = description
		}
		if enumValues, ok := schema["enum"].([]any); ok {
			result["enum"] = enumValues
		}
		if format := stringValue(schema["format"]); format != "" {
			result["format"] = format
		}
		return result
	}
}

func extractModelSchema(document map[string]any, modelName string) map[string]any {
	model, ok := openapi.LookupSchema(document, modelName)
	if !ok {
		return map[string]any{"type": "object", "description": fmt.Sprintf("Model '%s' not found", modelName)}
	}
	return processSchema(document, model)
}

func processSpecialRef(document map[string]any, modelName string) (map[string]any, bool) {
	model, ok := openapi.LookupSchema(document, modelName)
	if !ok {
		switch {
		case strings.Contains(modelName, "NullableDate"):
			return map[string]any{"type": "string", "format": "date", "description": "A nullable date value (format: YYYY-MM-DD)"}, true
		case strings.Contains(modelName, "Date"), strings.Contains(modelName, "Time"):
			format := "date"
			if strings.Contains(modelName, "Time") {
				format = "date-time"
			}
			return map[string]any{"type": "string", "format": format, "description": fmt.Sprintf("Model '%s' inferred as a date/time value", modelName)}, true
		case strings.Contains(modelName, "Slice"), strings.Contains(modelName, "Array"), strings.Contains(modelName, "List"):
			return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": fmt.Sprintf("Model '%s' inferred as an array", modelName)}, true
		}
		return nil, false
	}

	if description := strings.ToLower(stringValue(model["description"])); strings.Contains(description, "unmarshal") {
		if properties, ok := model["properties"].(map[string]any); ok {
			if valueSchema, ok := properties["value"].(map[string]any); ok {
				processed := processSchema(document, valueSchema)
				processed["description"] = stringValue(model["description"])
				return processed, true
			}
		}
		return map[string]any{"type": "string", "description": stringValue(model["description"])}, true
	}

	if properties, ok := model["properties"].(map[string]any); ok && len(properties) == 1 {
		if valueSchema, ok := properties["value"].(map[string]any); ok {
			processed := processSchema(document, valueSchema)
			if description := stringValue(model["description"]); description != "" {
				processed["description"] = description
			}
			return processed, true
		}
	}

	if strings.Contains(modelName, "Date") || strings.Contains(modelName, "Time") || strings.Contains(modelName, "Duration") {
		processed := map[string]any{"type": "string"}
		if description := stringValue(model["description"]); description != "" {
			processed["description"] = description
		}
		if strings.Contains(modelName, "DateTime") || strings.Contains(modelName, "Time") {
			processed["format"] = "date-time"
		} else if strings.Contains(modelName, "Date") {
			processed["format"] = "date"
		}
		return processed, true
	}
	return nil, false
}

func mapSwaggerTypeToJSONSchema(swaggerType string) string {
	switch swaggerType {
	case "integer", "number", "string", "boolean", "array", "object":
		return swaggerType
	case "file":
		return "string"
	default:
		return "string"
	}
}

func generateToolDefinition(toolName, method, endpointPath string, operation map[string]any, inputSchema map[string]any) string {
	descriptionParts := make([]string, 0, 3)
	if summary := stringValue(operation["summary"]); summary != "" {
		descriptionParts = append(descriptionParts, summary)
	}
	if description := stringValue(operation["description"]); description != "" {
		descriptionParts = append(descriptionParts, description)
	}

	aiDescription := fmt.Sprintf("AI INSTRUCTIONS: This endpoint allows you to %s resources.", strings.ToLower(toolName))
	requiredParameters := make([]string, 0)
	if parameters, ok := operation["parameters"].([]any); ok {
		for _, rawParameter := range parameters {
			parameter, ok := rawParameter.(map[string]any)
			if !ok {
				continue
			}
			if boolFromAny(parameter["required"]) {
				requiredParameters = append(requiredParameters, stringValue(parameter["name"]))
			}
		}
	}
	if len(requiredParameters) > 0 {
		aiDescription += " It requires the following parameters: " + strings.Join(requiredParameters, ", ") + "."
	}
	if responses, ok := operation["responses"].(map[string]any); ok {
		if successResponse, ok := responses["200"].(map[string]any); ok {
			if successDescription := strings.TrimSpace(stringValue(successResponse["description"])); successDescription != "" {
				aiDescription += " On success, it returns a " + successDescription + "."
			}
		}
	}
	descriptionParts = append(descriptionParts, aiDescription)

	variableName := toExportedIdentifier(toolName) + "Tool"
	renderedSchema := renderGoLiteral(inputSchema, 2)

	return fmt.Sprintf(`// %s defines the MCP tool metadata for %s %s.
var %s = struct {
Name        string
Description string
InputSchema map[string]any
}{
Name: %q,
Description: %q,
InputSchema: %s,
}`,
		variableName,
		strings.ToUpper(method),
		endpointPath,
		variableName,
		toolName,
		strings.Join(descriptionParts, ". "),
		renderedSchema,
	)
}

func generateHandlerFunction(toolName, method, endpointPath string) string {
	handlerName := "Handle" + toExportedIdentifier(toolName)
	return fmt.Sprintf(`// %s is a scaffold for calling %s %s.
func %s(input map[string]any) map[string]any {
_ = input
return map[string]any{
"content": []map[string]any{
{
"type": "text",
"text": %q,
},
},
}
}`,
		handlerName,
		strings.ToUpper(method),
		endpointPath,
		handlerName,
		"{\n  \"success\": true,\n  \"message\": \"Not implemented yet\"\n}",
	)
}

func renderGoLiteral(value any, indent int) string {
	padding := strings.Repeat("\t", indent)
	switch typedValue := value.(type) {
	case map[string]any:
		if len(typedValue) == 0 {
			return "map[string]any{}"
		}
		keys := make([]string, 0, len(typedValue))
		for key := range typedValue {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		lines := make([]string, 0, len(keys))
		for _, key := range keys {
			renderedValue := renderGoLiteral(typedValue[key], indent+1)
			lines = append(lines, fmt.Sprintf("%s%q: %s,", padding+"\t", key, renderedValue))
		}
		return "map[string]any{\n" + strings.Join(lines, "\n") + "\n" + padding + "}"
	case []any:
		if len(typedValue) == 0 {
			return "[]any{}"
		}
		lines := make([]string, 0, len(typedValue))
		for _, item := range typedValue {
			lines = append(lines, fmt.Sprintf("%s%s,", padding+"\t", renderGoLiteral(item, indent+1)))
		}
		return "[]any{\n" + strings.Join(lines, "\n") + "\n" + padding + "}"
	case []string:
		if len(typedValue) == 0 {
			return "[]string{}"
		}
		lines := make([]string, 0, len(typedValue))
		for _, item := range typedValue {
			lines = append(lines, fmt.Sprintf("%s%q,", padding+"\t", item))
		}
		return "[]string{\n" + strings.Join(lines, "\n") + "\n" + padding + "}"
	case string:
		return strconv.Quote(typedValue)
	case bool:
		if typedValue {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(typedValue)
	case int64:
		return strconv.FormatInt(typedValue, 10)
	case float64:
		if typedValue == float64(int64(typedValue)) {
			return strconv.FormatInt(int64(typedValue), 10)
		}
		return strconv.FormatFloat(typedValue, 'f', -1, 64)
	case nil:
		return "nil"
	default:
		return strconv.Quote(fmt.Sprint(typedValue))
	}
}

func sanitizeParameterName(name string) string {
	return strings.ReplaceAll(name, ".", "")
}

func sortedMapKeys(mapped map[string]any) []string {
	keys := make([]string, 0, len(mapped))
	for key := range mapped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func boolFromAny(value any) bool {
	booleanValue, _ := value.(bool)
	return booleanValue
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
