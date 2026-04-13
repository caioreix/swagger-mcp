package mcp

import (
	"github.com/caioreix/swagger-mcp/internal/config"
	"github.com/caioreix/swagger-mcp/internal/openapi"
)

// ProxyInputSchema is a test export of proxyInputSchema.
func ProxyInputSchema(document map[string]any, operation map[string]any) map[string]any {
	return proxyInputSchema(document, operation)
}

// BuildProxyTools is a test export of buildProxyTools.
func BuildProxyTools(
	document map[string]any,
	baseURL string,
	filter *openapi.EndpointFilter,
	apiName string,
	auth config.AuthConfig,
	headers string,
) ([]proxyTool, error) {
	return buildProxyTools(document, baseURL, filter, apiName, auth, headers)
}

// BuildRequestBody is a test export of buildRequestBody.
func BuildRequestBody(operation map[string]any, arguments map[string]any) any {
	return buildRequestBody(operation, arguments)
}

// ProxyToolName is a test export of proxyToolName.
func ProxyToolName(ep openapi.Endpoint, apiName, apiTitle string) string {
	return proxyToolName(ep, apiName, apiTitle)
}
