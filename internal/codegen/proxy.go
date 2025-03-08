package codegen

import (
	"fmt"
	"strings"

	"github.com/caioreix/swagger-mcp/internal/openapi"
)

// GenerateProxyHandler generates a fully functional handler that forwards
// HTTP requests to the original REST API.
func GenerateProxyHandler(toolName, method, endpointPath, baseURL string, operation map[string]any, schemes []openapi.SecurityScheme) string {
	handlerName := "Handle" + toExportedIdentifier(toolName)
	upperMethod := strings.ToUpper(method)
	fileOp := DetectFileOperation(operation)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("// %s proxies %s %s to the target API.\n", handlerName, upperMethod, endpointPath))
	b.WriteString(fmt.Sprintf("func %s(input map[string]any) (result map[string]any) {\n", handlerName))
	b.WriteString(generatePanicRecovery(handlerName))

	// URL construction
	b.WriteString(generateURLConstruction(endpointPath, baseURL, operation))

	// Build request
	if fileOp == FileOpUpload {
		b.WriteString(fmt.Sprintf("\treq, err := buildMultipartRequest(targetURL, %q, input)\n", upperMethod))
		b.WriteString("\tif err != nil {\n\t\treturn mcpError(-32603, fmt.Sprintf(\"failed to build request: %%v\", err))\n\t}\n")
	} else {
		b.WriteString(generateJSONRequestBuilder(upperMethod))
	}

	// Auth
	if len(schemes) > 0 {
		b.WriteString("\tapplyAuth(req)\n")
	}

	// Send request
	b.WriteString(generateHTTPSend())

	// Handle response
	if fileOp == FileOpDownload {
		b.WriteString("\tif resp.StatusCode < 200 || resp.StatusCode >= 300 {\n")
		b.WriteString("\t\tbody, _ := io.ReadAll(resp.Body)\n")
		b.WriteString("\t\treturn httpStatusToMCPError(resp.StatusCode, string(body))\n")
		b.WriteString("\t}\n")
		b.WriteString("\treturn handleBinaryResponse(resp)\n")
	} else {
		b.WriteString(generateErrorResponseCheck())
		b.WriteString("\treturn mcpTextResult(string(respBody))\n")
	}

	b.WriteString("}\n")
	return b.String()
}

func generateURLConstruction(endpointPath, baseURL string, operation map[string]any) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\tbaseURL := %q\n", baseURL))
	b.WriteString(fmt.Sprintf("\tpath := %q\n", endpointPath))

	// Replace path parameters
	pathParams := extractPathParams(operation)
	if len(pathParams) > 0 {
		for _, param := range pathParams {
			b.WriteString(fmt.Sprintf("\tif v, ok := input[%q]; ok {\n", param))
			b.WriteString(fmt.Sprintf("\t\tpath = strings.ReplaceAll(path, \"{%s}\", fmt.Sprint(v))\n", param))
			b.WriteString("\t}\n")
		}
	}

	b.WriteString("\ttargetURL := baseURL + path\n")

	// Add query parameters
	queryParams := extractQueryParams(operation)
	if len(queryParams) > 0 {
		b.WriteString("\tqueryValues := url.Values{}\n")
		for _, param := range queryParams {
			b.WriteString(fmt.Sprintf("\tif v, ok := input[%q]; ok {\n", param))
			b.WriteString(fmt.Sprintf("\t\tqueryValues.Set(%q, fmt.Sprint(v))\n", param))
			b.WriteString("\t}\n")
		}
		b.WriteString("\tif encoded := queryValues.Encode(); encoded != \"\" {\n")
		b.WriteString("\t\ttargetURL += \"?\" + encoded\n")
		b.WriteString("\t}\n")
	}

	return b.String()
}

func generateJSONRequestBuilder(method string) string {
	if method == "GET" || method == "DELETE" || method == "HEAD" {
		return fmt.Sprintf(`	req, err := http.NewRequest(%q, targetURL, nil)
	if err != nil {
		return mcpError(-32603, fmt.Sprintf("failed to create request: %%v", err))
	}
`, method)
	}
	return fmt.Sprintf(`	bodyJSON, err := json.Marshal(input)
	if err != nil {
		return mcpError(-32603, fmt.Sprintf("failed to marshal request body: %%v", err))
	}
	req, err := http.NewRequest(%q, targetURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return mcpError(-32603, fmt.Sprintf("failed to create request: %%v", err))
	}
	req.Header.Set("Content-Type", "application/json")
`, method)
}

func generateHTTPSend() string {
	return `	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return mcpError(-32603, fmt.Sprintf("HTTP request failed: %v", err))
	}
	defer resp.Body.Close()
`
}

func extractPathParams(operation map[string]any) []string {
	return extractParamsByIn(operation, "path")
}

func extractQueryParams(operation map[string]any) []string {
	return extractParamsByIn(operation, "query")
}

func extractParamsByIn(operation map[string]any, in string) []string {
	params, ok := operation["parameters"].([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, rawParam := range params {
		param, ok := rawParam.(map[string]any)
		if !ok {
			continue
		}
		if stringValue(param["in"]) == in {
			if name := stringValue(param["name"]); name != "" {
				result = append(result, name)
			}
		}
	}
	return result
}

// proxyImports returns the import paths needed for proxy handler code.
func proxyImports(hasBodyMethods, hasFileOps bool) []string {
	imports := []string{"fmt", "io", "net/http", "net/url", "strings", "time"}
	if hasBodyMethods {
		imports = append(imports, "bytes", "encoding/json")
	}
	return imports
}
