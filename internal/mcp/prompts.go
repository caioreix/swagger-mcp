package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpgoserver "github.com/mark3labs/mcp-go/server"
)

func registerPrompts(s *mcpgoserver.MCPServer) {
	prompt := mcpgo.NewPrompt(
		"add-endpoint",
		mcpgo.WithPromptDescription("Guide through the process of adding a new endpoint using Swagger MCP tools"),
		mcpgo.WithArgument(
			"swaggerUrl",
			mcpgo.ArgumentDescription("URL of the Swagger definition (optional if already configured)"),
		),
		mcpgo.WithArgument(
			"endpointPath",
			mcpgo.ArgumentDescription("Path of the endpoint to implement (e.g., /pets/{id})"),
		),
		mcpgo.WithArgument(
			"httpMethod",
			mcpgo.ArgumentDescription("HTTP method of the endpoint (e.g., GET, POST, PUT, DELETE)"),
		),
	)

	s.AddPrompt(prompt, func(_ context.Context, req mcpgo.GetPromptRequest) (*mcpgo.GetPromptResult, error) {
		args := req.Params.Arguments
		swaggerURL := strings.TrimSpace(args["swaggerUrl"])
		endpointPath := strings.TrimSpace(args["endpointPath"])
		httpMethod := strings.TrimSpace(args["httpMethod"])

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
			"Guide through the process of adding a new endpoint using Swagger MCP tools",
			messages,
		), nil
	})
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
				"**Step 1:** First, we need to get the Swagger definition.\n\nI'll use the `mcp__getSwaggerDefinition` tool to download and save it locally.",
			),
		),
		mcpgo.NewPromptMessage(
			mcpgo.RoleUser,
			mcpgo.NewTextContent("Great, please proceed with downloading the Swagger definition."),
		),
		mcpgo.NewPromptMessage(
			mcpgo.RoleAssistant,
			mcpgo.NewTextContent(
				"**Step 2:** Now I'll list all available endpoints using the `mcp__listEndpoints` tool.\n\nThis will help us understand the API structure and confirm the endpoint we want to implement.",
			),
		),
		mcpgo.NewPromptMessage(mcpgo.RoleUser, mcpgo.NewTextContent("Please show me the available endpoints.")),
		mcpgo.NewPromptMessage(
			mcpgo.RoleAssistant,
			mcpgo.NewTextContent(
				"**Step 3:** Let's identify the models used by our target endpoint with the `mcp__listEndpointModels` tool.\n\nThis will show us all the data models we need to generate.",
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
				"**Step 5:** The final steps are:\n\n1. Register the new tool in your MCP server configuration\n2. Test the endpoint with sample requests\n\nWould you like me to help with any of these steps?",
			),
		),
	}
}
