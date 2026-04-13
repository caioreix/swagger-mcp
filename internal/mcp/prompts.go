package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpgoserver "github.com/mark3labs/mcp-go/server"
)

const (
	addEndpointPromptName       = "swagger_add_endpoint"
	legacyAddEndpointPromptName = "add-endpoint"
)

func registerPrompts(s *mcpgoserver.MCPServer) {
	for _, promptName := range []string{addEndpointPromptName, legacyAddEndpointPromptName} {
		description := "Guided workflow for adding a new endpoint using Swagger MCP tools"
		if promptName == legacyAddEndpointPromptName {
			description += " (legacy alias)"
		}

		prompt := mcpgo.NewPrompt(
			promptName,
			mcpgo.WithPromptDescription(description),
			mcpgo.WithArgument(
				"swagger_url",
				mcpgo.ArgumentDescription("URL of the Swagger definition (optional if already configured)"),
			),
			mcpgo.WithArgument(
				"endpoint_path",
				mcpgo.ArgumentDescription("Path of the endpoint to implement (e.g., /pets/{id})"),
			),
			mcpgo.WithArgument(
				"http_method",
				mcpgo.ArgumentDescription("HTTP method of the endpoint (e.g., GET, POST, PUT, DELETE)"),
			),
		)

		s.AddPrompt(prompt, func(_ context.Context, req mcpgo.GetPromptRequest) (*mcpgo.GetPromptResult, error) {
			args := req.Params.Arguments
			swaggerURL := firstNonEmptyArgument(args, "swagger_url", "swaggerUrl")
			endpointPath := firstNonEmptyArgument(args, "endpoint_path", "endpointPath")
			httpMethod := firstNonEmptyArgument(args, "http_method", "httpMethod")

			target := ""
			if endpointPath != "" {
				target = fmt.Sprintf(" (%s %s)", strings.TrimSpace(httpMethod), endpointPath)
			}
			source := ""
			if swaggerURL != "" {
				source = fmt.Sprintf(" from the Swagger definition at %s", swaggerURL)
			}

			messages := buildAddEndpointMessages(target, source)
			return mcpgo.NewGetPromptResult(
				"Guided workflow for adding a new endpoint using Swagger MCP tools",
				messages,
			), nil
		})
	}
}

func firstNonEmptyArgument(arguments map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(arguments[key]); value != "" {
			return value
		}
	}
	return ""
}

func buildAddEndpointMessages(target, source string) []mcpgo.PromptMessage {
	return []mcpgo.PromptMessage{
		mcpgo.NewPromptMessage(
			mcpgo.RoleUser,
			mcpgo.NewTextContent(fmt.Sprintf("I need to add a new endpoint to my project%s%s.", target, source)),
		),
		mcpgo.NewPromptMessage(
			mcpgo.RoleAssistant,
			mcpgo.NewTextContent(
				"I'll guide you through adding a new endpoint using the Swagger MCP tools. Let's follow these steps in order:",
			),
		),
		mcpgo.NewPromptMessage(
			mcpgo.RoleAssistant,
			mcpgo.NewTextContent(
				"**Step 1:** First, we need to get the Swagger definition.\n\nI'll use the `mcp__swagger_get_definition` tool to download and save it locally.",
			),
		),
		mcpgo.NewPromptMessage(
			mcpgo.RoleUser,
			mcpgo.NewTextContent("Great, please proceed with downloading the Swagger definition."),
		),
		mcpgo.NewPromptMessage(
			mcpgo.RoleAssistant,
			mcpgo.NewTextContent(
				"**Step 2:** Now I'll list all available endpoints using the `mcp__swagger_list_endpoints` tool.\n\nThis will help us understand the API structure and confirm the endpoint we want to implement.",
			),
		),
		mcpgo.NewPromptMessage(mcpgo.RoleUser, mcpgo.NewTextContent("Please show me the available endpoints.")),
		mcpgo.NewPromptMessage(
			mcpgo.RoleAssistant,
			mcpgo.NewTextContent(
				"**Step 3:** Let's identify the models used by our target endpoint with the `mcp__swagger_list_endpoint_models` tool.\n\nThis will show us the data models we need to understand before integrating the endpoint.",
			),
		),
		mcpgo.NewPromptMessage(
			mcpgo.RoleUser,
			mcpgo.NewTextContent("Please identify the models for this endpoint."),
		),
		mcpgo.NewPromptMessage(
			mcpgo.RoleAssistant,
			mcpgo.NewTextContent(
				"**Step 4:** Now we need to implement any additional logic needed for the tool handler:\n\n- Authentication handling\n- Error handling\n- File upload/download support (if needed)\n- Integration with your application's services",
			),
		),
		mcpgo.NewPromptMessage(
			mcpgo.RoleUser,
			mcpgo.NewTextContent("What else do I need to do to complete the implementation?"),
		),
		mcpgo.NewPromptMessage(
			mcpgo.RoleAssistant,
			mcpgo.NewTextContent(
				"**Step 5:** The final steps are:\n\n1. Register the new integration in your MCP server or application configuration\n2. Test the endpoint with sample requests\n\nWould you like me to help with any of these steps?",
			),
		),
	}
}
