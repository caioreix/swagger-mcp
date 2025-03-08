package codegen

import (
	"strings"
	"testing"

	"github.com/caioreix/swagger-mcp/internal/openapi"
	"github.com/caioreix/swagger-mcp/internal/testutil"
)

func TestGenerateEndpointToolCodeGoldenPetstore(t *testing.T) {
	document, err := openapi.ReadDefinitionFromFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	generatedCode, err := GenerateEndpointToolCode(document, GenerateEndpointToolCodeParams{
		Path:                    "/pets",
		Method:                  "POST",
		SingularizeResourceName: true,
	})
	if err != nil {
		t.Fatalf("GenerateEndpointToolCode returned error: %v", err)
	}
	expected := testutil.ReadGolden(t, "petstore-add-pet-tool.golden.go")
	if generatedCode != expected {
		t.Fatalf("unexpected golden output\nexpected:\n%s\nactual:\n%s", expected, generatedCode)
	}
}

func TestGenerateEndpointToolCodeYAMLDateTime(t *testing.T) {
	document, err := openapi.ReadDefinitionFromFile(testutil.FixturePath(t, "date-time-test.yml"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	generatedCode, err := GenerateEndpointToolCode(document, GenerateEndpointToolCodeParams{
		Path:                    "/events",
		Method:                  "POST",
		SingularizeResourceName: true,
	})
	if err != nil {
		t.Fatalf("GenerateEndpointToolCode returned error: %v", err)
	}

	checks := []string{
		`Name: "createEvent"`,
		`"format": "date-time"`,
		`"type": "string"`,
		`HandleCreateEvent`,
	}
	for _, check := range checks {
		if !strings.Contains(generatedCode, check) {
			t.Fatalf("expected generated code to contain %q\n%s", check, generatedCode)
		}
	}
}

func TestGenerateEndpointToolCodeOpenAPI31(t *testing.T) {
	document, err := openapi.ReadDefinitionFromFile(testutil.FixturePath(t, "openapi-3.1.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	generatedCode, err := GenerateEndpointToolCode(document, GenerateEndpointToolCodeParams{
		Path:                    "/widgets",
		Method:                  "POST",
		SingularizeResourceName: true,
	})
	if err != nil {
		t.Fatalf("GenerateEndpointToolCode returned error: %v", err)
	}
	if !strings.Contains(generatedCode, `"requestBody": map[string]any{`) {
		t.Fatalf("expected OpenAPI 3.1 requestBody schema in generated code\n%s", generatedCode)
	}
}

func TestGenerateEndpointToolCodeMissingMethod(t *testing.T) {
	document, err := openapi.ReadDefinitionFromFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	_, err = GenerateEndpointToolCode(document, GenerateEndpointToolCodeParams{Path: "/pets"})
	if err == nil {
		t.Fatal("expected missing method to fail")
	}
	if !strings.Contains(err.Error(), "method parameter is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
