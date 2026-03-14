package format

import (
	"bufio"
	"bytes"
	"fmt"
	"maps"
	"strconv"
	"strings"
)

// ParseYAML parses YAML-encoded data into a native Go value (map, slice, or scalar).
func ParseYAML(data []byte) (any, error) {
	lines, err := tokenizeYAML(data)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return map[string]any{}, nil
	}

	index := 0
	value, err := parseYAMLBlock(lines, &index, lines[0].indent)
	if err != nil {
		return nil, err
	}
	return value, nil
}

type yamlLine struct {
	indent int
	text   string
}

func tokenizeYAML(data []byte) ([]yamlLine, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	lines := make([]yamlLine, 0)
	for scanner.Scan() {
		raw := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lines = append(lines, yamlLine{indent: countLeadingSpaces(raw), text: trimmed})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan yaml: %w", err)
	}
	return lines, nil
}

func parseYAMLBlock(lines []yamlLine, index *int, indent int) (any, error) {
	if *index >= len(lines) {
		return map[string]any{}, nil
	}
	if strings.HasPrefix(lines[*index].text, "-") {
		return parseYAMLList(lines, index, indent)
	}
	return parseYAMLMap(lines, index, indent)
}

func parseYAMLMap(lines []yamlLine, index *int, indent int) (map[string]any, error) {
	result := make(map[string]any)
	for *index < len(lines) {
		line := lines[*index]
		if line.indent < indent {
			break
		}
		if line.indent > indent {
			return nil, fmt.Errorf("unexpected indentation at line %d", *index+1)
		}

		key, rawValue, ok := splitYAMLKeyValue(line.text)
		if !ok {
			return nil, fmt.Errorf("invalid YAML mapping at line %d", *index+1)
		}
		*index++

		if rawValue == "" {
			if *index < len(lines) && lines[*index].indent > indent {
				child, err := parseYAMLBlock(lines, index, lines[*index].indent)
				if err != nil {
					return nil, err
				}
				result[key] = child
			} else {
				result[key] = ""
			}
			continue
		}

		parsedValue, err := parseYAMLScalar(rawValue)
		if err != nil {
			return nil, err
		}
		result[key] = parsedValue
	}
	return result, nil
}

func parseYAMLList(lines []yamlLine, index *int, indent int) ([]any, error) {
	result := make([]any, 0)
	for *index < len(lines) {
		line := lines[*index]
		if line.indent < indent {
			break
		}
		if line.indent > indent {
			return nil, fmt.Errorf("unexpected indentation inside sequence at line %d", *index+1)
		}
		if !strings.HasPrefix(line.text, "-") {
			break
		}

		itemText := strings.TrimSpace(strings.TrimPrefix(line.text, "-"))
		*index++

		switch {
		case itemText == "":
			if *index < len(lines) && lines[*index].indent > indent {
				child, err := parseYAMLBlock(lines, index, lines[*index].indent)
				if err != nil {
					return nil, err
				}
				result = append(result, child)
			} else {
				result = append(result, nil)
			}
		case looksLikeYAMLMapping(itemText):
			key, rawValue, ok := splitYAMLKeyValue(itemText)
			if !ok {
				return nil, fmt.Errorf("invalid YAML sequence item at line %d", *index)
			}
			itemMap := map[string]any{}
			if rawValue == "" {
				if *index < len(lines) && lines[*index].indent > indent {
					child, err := parseYAMLBlock(lines, index, lines[*index].indent)
					if err != nil {
						return nil, err
					}
					itemMap[key] = child
				} else {
					itemMap[key] = ""
				}
			} else {
				parsedValue, err := parseYAMLScalar(rawValue)
				if err != nil {
					return nil, err
				}
				itemMap[key] = parsedValue
			}
			if *index < len(lines) && lines[*index].indent > indent {
				extra, err := parseYAMLMap(lines, index, lines[*index].indent)
				if err != nil {
					return nil, err
				}
				maps.Copy(itemMap, extra)
			}
			result = append(result, itemMap)
		default:
			parsedValue, err := parseYAMLScalar(itemText)
			if err != nil {
				return nil, err
			}
			result = append(result, parsedValue)
		}
	}
	return result, nil
}

func splitYAMLKeyValue(text string) (string, string, bool) {
	inSingle := false
	inDouble := false
	for i, r := range text {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ':':
			if inSingle || inDouble {
				continue
			}
			key := strings.TrimSpace(text[:i])
			value := strings.TrimSpace(text[i+1:])
			return key, value, key != ""
		}
	}
	return "", "", false
}

func looksLikeYAMLMapping(text string) bool {
	_, _, ok := splitYAMLKeyValue(text)
	return ok
}

func parseYAMLScalar(raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'") && len(raw) >= 2 {
		return strings.ReplaceAll(raw[1:len(raw)-1], "''", "'"), nil
	}
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") && len(raw) >= 2 {
		unquoted, err := strconv.Unquote(raw)
		if err != nil {
			return nil, fmt.Errorf("unquote yaml string %q: %w", raw, err)
		}
		return unquoted, nil
	}

	switch strings.ToLower(raw) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null", "~":
		return nil, nil
	}

	if integer, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return integer, nil
	}
	if number, err := strconv.ParseFloat(raw, 64); err == nil && strings.Count(raw, ".") == 1 {
		return number, nil
	}

	return raw, nil
}

func countLeadingSpaces(text string) int {
	count := 0
	for _, r := range text {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}
