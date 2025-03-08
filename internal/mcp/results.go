package mcp

import "encoding/json"

func textResult(text string, isError bool) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isError}
}

func jsonTextResult(value any, isError bool) (map[string]any, error) {
	formatted, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return textResult(string(formatted), isError), nil
}
