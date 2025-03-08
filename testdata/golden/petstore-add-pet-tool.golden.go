// AddPetTool defines the MCP tool metadata for POST /pets.
var AddPetTool = struct {
Name        string
Description string
InputSchema map[string]any
}{
Name: "addPet",
Description: "Creates a new pet in the store. AI INSTRUCTIONS: This endpoint allows you to addpet resources. It requires the following parameters: pet. On success, it returns a pet response.",
InputSchema: map[string]any{
			"properties": map[string]any{
				"pet": map[string]any{
					"description": "Pet to add to the store",
					"properties": map[string]any{
						"category": map[string]any{
							"properties": map[string]any{
								"id": map[string]any{
									"format": "int64",
									"type": "integer",
								},
								"name": map[string]any{
									"type": "string",
								},
							},
							"required": []string{},
							"type": "object",
						},
						"name": map[string]any{
							"type": "string",
						},
						"status": map[string]any{
							"description": "pet status in the store",
							"enum": []any{
								"available",
								"pending",
								"sold",
							},
							"type": "string",
						},
						"tag": map[string]any{
							"type": "string",
						},
					},
					"required": []string{
						"name",
					},
					"type": "object",
				},
			},
			"required": []string{
				"pet",
			},
			"type": "object",
		},
}

// HandleAddPet is a scaffold for calling POST /pets.
func HandleAddPet(input map[string]any) map[string]any {
_ = input
return map[string]any{
"content": []map[string]any{
{
"type": "text",
"text": "{\n  \"success\": true,\n  \"message\": \"Not implemented yet\"\n}",
},
},
}
}