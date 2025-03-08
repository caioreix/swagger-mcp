package mcp

import (
	"fmt"
	"strings"
)

func (s *server) promptDefinitions() []promptDefinition {
	return []promptDefinition{{
		Name:        "add-endpoint",
		Description: "Guide through the process of adding a new endpoint using Swagger MCP tools",
		Arguments: []promptArgument{
			{Name: "swaggerUrl", Description: "URL of the Swagger definition (optional if already configured)"},
			{Name: "endpointPath", Description: "Path of the endpoint to implement (e.g., /pets/{id})"},
			{Name: "httpMethod", Description: "HTTP method of the endpoint (e.g., GET, POST, PUT, DELETE)"},
		},
	}}
}

func (s *server) getPrompt(name string, arguments map[string]string) (map[string]any, error) {
	switch name {
	case "add-endpoint":
		swaggerURL := strings.TrimSpace(arguments["swaggerUrl"])
		endpointPath := strings.TrimSpace(arguments["endpointPath"])
		httpMethod := strings.TrimSpace(arguments["httpMethod"])
		target := ""
		if endpointPath != "" {
			target = fmt.Sprintf(" (%s %s)", strings.TrimSpace(httpMethod), endpointPath)
		}
		source := ""
		if swaggerURL != "" {
			source = fmt.Sprintf(" from the Swagger definition at %s", swaggerURL)
		}
		messages := []map[string]any{
			{"role": "user", "content": map[string]any{"type": "text", "text": fmt.Sprintf("I need to add a new endpoint to my project%s%s.", target, source)}},
			{"role": "assistant", "content": map[string]any{"type": "text", "text": "I'll guide you through adding a new endpoint using the Swagger MCP tools. Let's follow these steps in order:"}},
			{"role": "assistant", "content": map[string]any{"type": "text", "text": "**Step 1:** First, we need to get the Swagger definition.\n\nI'll use the `mcp__getSwaggerDefinition` tool to download and save it locally."}},
			{"role": "user", "content": map[string]any{"type": "text", "text": "Great, please proceed with downloading the Swagger definition."}},
			{"role": "assistant", "content": map[string]any{"type": "text", "text": "**Step 2:** Now I'll list all available endpoints using the `mcp__listEndpoints` tool.\n\nThis will help us understand the API structure and confirm the endpoint we want to implement."}},
			{"role": "user", "content": map[string]any{"type": "text", "text": "Please show me the available endpoints."}},
			{"role": "assistant", "content": map[string]any{"type": "text", "text": "**Step 3:** Let's identify the models used by our target endpoint with the `mcp__listEndpointModels` tool.\n\nThis will show us all the data models we need to generate."}},
			{"role": "user", "content": map[string]any{"type": "text", "text": "Please identify the models for this endpoint."}},
			{"role": "assistant", "content": map[string]any{"type": "text", "text": "**Step 4:** Now I'll generate Go model code for each model using the `mcp__generateModelCode` tool.\n\nI'll do this for each model identified in the previous step."}},
			{"role": "user", "content": map[string]any{"type": "text", "text": "Please generate the model code."}},
			{"role": "assistant", "content": map[string]any{"type": "text", "text": "**Step 5:** Finally, I'll generate the MCP tool definition scaffold using the `mcp__generateEndpointToolCode` tool.\n\nThis will create a Go-based MCP tool scaffold for the endpoint."}},
			{"role": "user", "content": map[string]any{"type": "text", "text": "Please generate the endpoint tool code."}},
			{"role": "assistant", "content": map[string]any{"type": "text", "text": "**Step 6:** Now we need to implement any additional logic needed for the tool handler:\n\n- Authentication handling\n- Error handling\n- File upload/download support (if needed)\n- Integration with your application's services"}},
			{"role": "user", "content": map[string]any{"type": "text", "text": "What else do I need to do to complete the implementation?"}},
			{"role": "assistant", "content": map[string]any{"type": "text", "text": "**Step 7:** The final steps are:\n\n1. Register the new tool in your MCP server configuration\n2. Test the endpoint with sample requests\n\nWould you like me to help with any of these steps?"}},
		}
		return map[string]any{"messages": messages}, nil
	default:
		return nil, fmt.Errorf("unknown prompt: %s", name)
	}
}
