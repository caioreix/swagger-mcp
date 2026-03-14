package codegen

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/caioreix/swagger-mcp/internal/openapi"
)

func GenerateModelCode(document map[string]any, modelName string) (string, error) {
	schema, ok := openapi.LookupSchema(document, modelName)
	if !ok {
		return "", fmt.Errorf("model '%s' not found in Swagger definition", modelName)
	}

	builder := &strings.Builder{}
	writeModelDefinition(builder, toExportedIdentifier(modelName), schema, document, "")
	return strings.TrimSpace(builder.String()) + "\n", nil
}

func writeModelDefinition(builder *strings.Builder, typeName string, schema map[string]any, document map[string]any, indent string) {
	if description := stringValue(schema["description"]); description != "" {
		builder.WriteString(indent + "// " + typeName + " " + sanitizeComment(description) + "\n")
	}
	builder.WriteString(indent + "type " + typeName + " struct {\n")

	mergedProperties, mergedRequired := collectProperties(schema, document)
	propertyNames := make([]string, 0, len(mergedProperties))
	for propertyName := range mergedProperties {
		propertyNames = append(propertyNames, propertyName)
	}
	sort.Strings(propertyNames)

	for _, propertyName := range propertyNames {
		propertySchema, _ := mergedProperties[propertyName].(map[string]any)
		if description := stringValue(propertySchema["description"]); description != "" {
			builder.WriteString(indent + "\t// " + sanitizeComment(description) + "\n")
		}
		fieldName := toExportedIdentifier(propertyName)
		fieldType := goType(propertySchema, document, indent+"\t")
		tag := fmt.Sprintf("`json:\"%s,omitempty\"`", propertyName)
		if mergedRequired[propertyName] {
			tag = fmt.Sprintf("`json:\"%s\"`", propertyName)
		}
		builder.WriteString(fmt.Sprintf("%s\t%s %s %s\n", indent, fieldName, fieldType, tag))
	}

	builder.WriteString(indent + "}\n")
}

func collectProperties(schema map[string]any, document map[string]any) (map[string]any, map[string]bool) {
	properties := make(map[string]any)
	required := make(map[string]bool)

	mergeSchemaProperties(properties, required, schema)

	if allOfValues, ok := schema["allOf"].([]any); ok {
		for _, rawValue := range allOfValues {
			subSchema, ok := rawValue.(map[string]any)
			if !ok {
				continue
			}
			if ref, ok := subSchema["$ref"].(string); ok {
				resolved, err := openapi.ResolveRef(document, ref)
				if err == nil {
					if resolvedMap, ok := resolved.(map[string]any); ok {
						mergeSchemaProperties(properties, required, resolvedMap)
					}
				}
				continue
			}
			mergeSchemaProperties(properties, required, subSchema)
		}
	}

	return properties, required
}

func mergeSchemaProperties(properties map[string]any, required map[string]bool, schema map[string]any) {
	if schemaProperties, ok := schema["properties"].(map[string]any); ok {
		maps.Copy(properties, schemaProperties)
	}
	if requiredList, ok := schema["required"].([]any); ok {
		for _, rawValue := range requiredList {
			if propertyName, ok := rawValue.(string); ok {
				required[propertyName] = true
			}
		}
	}
}

func goType(schema map[string]any, document map[string]any, indent string) string {
	if ref, ok := schema["$ref"].(string); ok {
		modelName := ref[strings.LastIndex(ref, "/")+1:]
		if specialType, ok := specialRefType(document, modelName); ok {
			return specialType
		}
		return toExportedIdentifier(modelName)
	}

	if allOfValues, ok := schema["allOf"].([]any); ok {
		mergedSchema := map[string]any{"type": "object", "properties": map[string]any{}, "required": []any{}}
		mergedProperties := mergedSchema["properties"].(map[string]any)
		required := make([]any, 0)
		for _, rawValue := range allOfValues {
			subSchema, ok := rawValue.(map[string]any)
			if !ok {
				continue
			}
			if ref, ok := subSchema["$ref"].(string); ok {
				resolved, err := openapi.ResolveRef(document, ref)
				if err == nil {
					if resolvedMap, ok := resolved.(map[string]any); ok {
						subSchema = resolvedMap
					}
				}
			}
			if properties, ok := subSchema["properties"].(map[string]any); ok {
				maps.Copy(mergedProperties, properties)
			}
			if requiredValues, ok := subSchema["required"].([]any); ok {
				required = append(required, requiredValues...)
			}
		}
		mergedSchema["required"] = required
		return goType(mergedSchema, document, indent)
	}

	if oneOfValues, ok := schema["oneOf"].([]any); ok && len(oneOfValues) > 0 {
		return "any"
	}
	if anyOfValues, ok := schema["anyOf"].([]any); ok && len(anyOfValues) > 0 {
		return "any"
	}

	schemaType := stringValue(schema["type"])
	switch schemaType {
	case "string":
		return "string"
	case "integer":
		if stringValue(schema["format"]) == "int64" {
			return "int64"
		}
		return "int"
	case "number":
		return "float64"
	case "boolean":
		return "bool"
	case "array":
		if items, ok := schema["items"].(map[string]any); ok {
			return "[]" + goType(items, document, indent)
		}
		return "[]any"
	case "object":
		if additional, ok := schema["additionalProperties"].(map[string]any); ok {
			return "map[string]" + goType(additional, document, indent)
		}
		if properties, ok := schema["properties"].(map[string]any); ok && len(properties) > 0 {
			return renderInlineStruct(schema, document, indent)
		}
		return "map[string]any"
	default:
		if properties, ok := schema["properties"].(map[string]any); ok && len(properties) > 0 {
			return renderInlineStruct(schema, document, indent)
		}
		return "any"
	}
}

func renderInlineStruct(schema map[string]any, document map[string]any, indent string) string {
	var builder strings.Builder
	builder.WriteString("struct {\n")
	properties, required := collectProperties(schema, document)
	names := make([]string, 0, len(properties))
	for propertyName := range properties {
		names = append(names, propertyName)
	}
	sort.Strings(names)

	for _, propertyName := range names {
		propertySchema, _ := properties[propertyName].(map[string]any)
		if description := stringValue(propertySchema["description"]); description != "" {
			builder.WriteString(indent + "\t// " + sanitizeComment(description) + "\n")
		}
		fieldType := goType(propertySchema, document, indent+"\t")
		tag := fmt.Sprintf("`json:\"%s,omitempty\"`", propertyName)
		if required[propertyName] {
			tag = fmt.Sprintf("`json:\"%s\"`", propertyName)
		}
		builder.WriteString(fmt.Sprintf("%s\t%s %s %s\n", indent, toExportedIdentifier(propertyName), fieldType, tag))
	}
	builder.WriteString(indent + "}")
	return builder.String()
}

func specialRefType(document map[string]any, modelName string) (string, bool) {
	schema, ok := openapi.LookupSchema(document, modelName)
	if !ok {
		switch {
		case strings.Contains(modelName, "Date"), strings.Contains(modelName, "Time"), strings.Contains(modelName, "Duration"):
			return "string", true
		case strings.Contains(modelName, "Int64Slice"), strings.Contains(modelName, "IntSlice"):
			return "[]int64", true
		case strings.Contains(modelName, "Float64Slice"):
			return "[]float64", true
		}
		return "", false
	}

	if properties, ok := schema["properties"].(map[string]any); ok && len(properties) == 1 {
		if valueProperty, ok := properties["value"].(map[string]any); ok {
			return goType(valueProperty, document, ""), true
		}
	}

	if description := strings.ToLower(stringValue(schema["description"])); strings.Contains(description, "unmarshal") {
		return "string", true
	}

	if strings.Contains(modelName, "Date") || strings.Contains(modelName, "Time") || strings.Contains(modelName, "Duration") {
		return "string", true
	}
	return "", false
}

func toExportedIdentifier(name string) string {
	separators := func(r rune) bool {
		return r == '.' || r == '-' || r == '_' || r == '/' || r == '{' || r == '}' || r == ' ' || r == ':'
	}
	parts := strings.FieldsFunc(name, separators)
	if len(parts) == 0 {
		return "Value"
	}

	var builder strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		cleaned := strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z':
				return r
			case r >= 'A' && r <= 'Z':
				return r
			case r >= '0' && r <= '9':
				return r
			default:
				return -1
			}
		}, part)
		if cleaned == "" {
			continue
		}
		builder.WriteString(strings.ToUpper(cleaned[:1]))
		if len(cleaned) > 1 {
			builder.WriteString(cleaned[1:])
		}
	}

	result := builder.String()
	if result == "" {
		return "Value"
	}
	if result[0] >= '0' && result[0] <= '9' {
		return "Value" + result
	}
	return result
}

func sanitizeComment(text string) string {
	text = strings.ReplaceAll(text, "\n", " ")
	return strings.TrimSpace(text)
}

func stringValue(value any) string {
	stringValue, _ := value.(string)
	return stringValue
}
