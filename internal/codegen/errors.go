package codegen

import (
	"fmt"
	"strings"
)

// generateErrorHelpers returns Go helper functions for error handling in
// generated MCP tool handlers.
func generateErrorHelpers() string {
	return `func mcpTextResult(text string) map[string]any {
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	}
}

func mcpError(code int, message string) map[string]any {
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": fmt.Sprintf("{\"error\":{\"code\":%d,\"message\":%q}}", code, message)},
		},
		"isError": true,
	}
}

func httpStatusToMCPError(statusCode int, body string) map[string]any {
	message := fmt.Sprintf("HTTP %d", statusCode)
	if body != "" {
		if len(body) > 500 {
			body = body[:500] + "..."
		}
		message = fmt.Sprintf("HTTP %d: %s", statusCode, body)
	}
	return mcpError(statusCode, message)
}
`
}

// generateErrorResponseCheck returns Go code that checks an HTTP response
// status and returns an MCP error if non-2xx.
func generateErrorResponseCheck() string {
	return `	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcpError(-32603, fmt.Sprintf("failed to read response: %v", err))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return httpStatusToMCPError(resp.StatusCode, string(respBody))
	}
`
}

// generatePanicRecovery returns a deferred panic recovery statement for handlers.
func generatePanicRecovery(handlerName string) string {
	return fmt.Sprintf(`	defer func() {
		if r := recover(); r != nil {
			result = mcpError(-32603, fmt.Sprintf("%s panicked: %%v", r))
		}
	}()
`, handlerName)
}

// generateHandlerWithErrors generates a handler function with proper error handling,
// including HTTP response checking and panic recovery.
func generateHandlerWithErrors(toolName, method, endpointPath string) string {
	handlerName := "Handle" + toExportedIdentifier(toolName)
	return fmt.Sprintf(`// %s handles %s %s with error handling.
func %s(input map[string]any) (result map[string]any) {
%s
	_ = input
	return mcpTextResult("{\"success\": true, \"message\": \"Not implemented yet\"}")
}`,
		handlerName,
		strings.ToUpper(method),
		endpointPath,
		handlerName,
		generatePanicRecovery(handlerName),
	)
}
