package codegen

import (
	"fmt"
	"strings"
)

// generateValidationCode returns Go code that validates input against the given
// JSON Schema at runtime. It generates a validateInput function.
func generateValidationCode(inputSchema map[string]any) string {
	var b strings.Builder
	b.WriteString(`// validateInput validates the input map against the tool's input schema.
func validateInput(input map[string]any) []string {
	var errors []string
`)
	generateFieldValidation(&b, inputSchema, "input", "\t")
	b.WriteString(`	return errors
}
`)
	return b.String()
}

func generateFieldValidation(b *strings.Builder, schema map[string]any, accessor string, indent string) {
	// Required fields check
	if required, ok := schema["required"].([]any); ok && len(required) > 0 {
		for _, rawField := range required {
			field, ok := rawField.(string)
			if !ok {
				continue
			}
			b.WriteString(fmt.Sprintf(`%sif _, ok := %s[%q]; !ok {
%s	errors = append(errors, "missing required field: %s")
%s}
`, indent, accessor, field, indent, field, indent))
		}
	}

	// Property validation
	properties, _ := schema["properties"].(map[string]any)
	for name, rawProp := range properties {
		prop, ok := rawProp.(map[string]any)
		if !ok {
			continue
		}
		generatePropertyValidation(b, name, prop, accessor, indent)
	}
}

func generatePropertyValidation(b *strings.Builder, name string, prop map[string]any, accessor, indent string) {
	varName := "val_" + sanitizeVarName(name)
	b.WriteString(fmt.Sprintf(`%sif %s, ok := %s[%q]; ok {
`, indent, varName, accessor, name))

	propType := stringValue(prop["type"])

	// Type checking
	switch propType {
	case "string":
		generateStringValidation(b, name, varName, prop, indent+"\t")
	case "integer", "number":
		generateNumericValidation(b, name, varName, prop, propType, indent+"\t")
	case "boolean":
		b.WriteString(fmt.Sprintf(`%sif _, ok := %s.(bool); !ok {
%s	errors = append(errors, "field %s must be a boolean")
%s}
`, indent+"\t", varName, indent+"\t", name, indent+"\t"))
	case "array":
		generateArrayValidation(b, name, varName, prop, indent+"\t")
	case "object":
		b.WriteString(fmt.Sprintf(`%sif _, ok := %s.(map[string]any); !ok {
%s	errors = append(errors, "field %s must be an object")
%s}
`, indent+"\t", varName, indent+"\t", name, indent+"\t"))
	}

	b.WriteString(fmt.Sprintf("%s}\n", indent))
}

func generateStringValidation(b *strings.Builder, name, varName string, prop map[string]any, indent string) {
	b.WriteString(fmt.Sprintf(`%sstrVal, ok := %s.(string)
%sif !ok {
%s	errors = append(errors, "field %s must be a string")
%s} else {
%s	_ = strVal
`, indent, varName, indent, indent, name, indent, indent))

	// Enum validation
	if enum, ok := prop["enum"].([]any); ok && len(enum) > 0 {
		allowed := make([]string, 0, len(enum))
		for _, e := range enum {
			if s, ok := e.(string); ok {
				allowed = append(allowed, fmt.Sprintf("%q", s))
			}
		}
		if len(allowed) > 0 {
			b.WriteString(fmt.Sprintf(`%s	validEnum := map[string]bool{%s: true}
%s	if !validEnum[strVal] {
%s		errors = append(errors, fmt.Sprintf("field %s must be one of: %s, got %%q", strVal))
%s	}
`, indent, strings.Join(allowedToBoolMap(allowed), ", "), indent, indent, name, strings.Join(enumDisplay(enum), ", "), indent))
		}
	}

	// MinLength
	if minLen, ok := numericValue(prop["minLength"]); ok {
		b.WriteString(fmt.Sprintf(`%s	if len(strVal) < %d {
%s		errors = append(errors, fmt.Sprintf("field %s must be at least %d characters, got %%d", len(strVal)))
%s	}
`, indent, int(minLen), indent, name, int(minLen), indent))
	}

	// MaxLength
	if maxLen, ok := numericValue(prop["maxLength"]); ok {
		b.WriteString(fmt.Sprintf(`%s	if len(strVal) > %d {
%s		errors = append(errors, fmt.Sprintf("field %s must be at most %d characters, got %%d", len(strVal)))
%s	}
`, indent, int(maxLen), indent, name, int(maxLen), indent))
	}

	// Pattern
	if pattern := stringValue(prop["pattern"]); pattern != "" {
		b.WriteString(fmt.Sprintf(`%s	if matched, _ := regexp.MatchString(%q, strVal); !matched {
%s		errors = append(errors, "field %s does not match required pattern")
%s	}
`, indent, pattern, indent, name, indent))
	}

	b.WriteString(fmt.Sprintf("%s}\n", indent))
}

func generateNumericValidation(b *strings.Builder, name, varName string, prop map[string]any, propType, indent string) {
	b.WriteString(fmt.Sprintf(`%snumVal, ok := toFloat64(%s)
%sif !ok {
%s	errors = append(errors, "field %s must be a number")
%s} else {
%s	_ = numVal
`, indent, varName, indent, indent, name, indent, indent))

	if minimum, ok := numericValue(prop["minimum"]); ok {
		b.WriteString(fmt.Sprintf(`%s	if numVal < %v {
%s		errors = append(errors, fmt.Sprintf("field %s must be >= %v, got %%v", numVal))
%s	}
`, indent, minimum, indent, name, minimum, indent))
	}

	if maximum, ok := numericValue(prop["maximum"]); ok {
		b.WriteString(fmt.Sprintf(`%s	if numVal > %v {
%s		errors = append(errors, fmt.Sprintf("field %s must be <= %v, got %%v", numVal))
%s	}
`, indent, maximum, indent, name, maximum, indent))
	}

	b.WriteString(fmt.Sprintf("%s}\n", indent))
}

func generateArrayValidation(b *strings.Builder, name, varName string, prop map[string]any, indent string) {
	b.WriteString(fmt.Sprintf(`%sarrVal, ok := %s.([]any)
%sif !ok {
%s	errors = append(errors, "field %s must be an array")
%s} else {
%s	_ = arrVal
`, indent, varName, indent, indent, name, indent, indent))

	if minItems, ok := numericValue(prop["minItems"]); ok {
		b.WriteString(fmt.Sprintf(`%s	if len(arrVal) < %d {
%s		errors = append(errors, fmt.Sprintf("field %s must have at least %d items, got %%d", len(arrVal)))
%s	}
`, indent, int(minItems), indent, name, int(minItems), indent))
	}

	if maxItems, ok := numericValue(prop["maxItems"]); ok {
		b.WriteString(fmt.Sprintf(`%s	if len(arrVal) > %d {
%s		errors = append(errors, fmt.Sprintf("field %s must have at most %d items, got %%d", len(arrVal)))
%s	}
`, indent, int(maxItems), indent, name, int(maxItems), indent))
	}

	b.WriteString(fmt.Sprintf("%s}\n", indent))
}

// generateValidationHelpers returns helper functions used by generated validation code.
func generateValidationHelpers() string {
	return `func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
`
}

func sanitizeVarName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

func allowedToBoolMap(quoted []string) []string {
	result := make([]string, len(quoted))
	for i, q := range quoted {
		result[i] = q + ": true"
	}
	return result
}

func enumDisplay(values []any) []string {
	result := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// validationImports returns imports needed for validation code.
func validationImports(schema map[string]any) []string {
	imports := []string{"encoding/json", "fmt"}
	if hasPatternValidation(schema) {
		imports = append(imports, "regexp")
	}
	return imports
}

func hasPatternValidation(schema map[string]any) bool {
	properties, _ := schema["properties"].(map[string]any)
	for _, rawProp := range properties {
		if prop, ok := rawProp.(map[string]any); ok {
			if pattern := stringValue(prop["pattern"]); pattern != "" {
				return true
			}
		}
	}
	return false
}
