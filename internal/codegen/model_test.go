package codegen

import (
	"strings"
	"testing"

	"github.com/caioreix/swagger-mcp/internal/openapi"
	"github.com/caioreix/swagger-mcp/internal/testutil"
)

func TestGenerateModelCodeGoldenPetstore(t *testing.T) {
	document, err := openapi.ReadDefinitionFromFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	generatedCode, err := GenerateModelCode(document, "Pet")
	if err != nil {
		t.Fatalf("GenerateModelCode returned error: %v", err)
	}
	expected := testutil.ReadGolden(t, "petstore-pet-model.golden.go")
	if generatedCode != expected {
		t.Fatalf("unexpected golden output\nexpected:\n%s\nactual:\n%s", expected, generatedCode)
	}
}

func TestGenerateModelCodeOpenAPI31(t *testing.T) {
	document, err := openapi.ReadDefinitionFromFile(testutil.FixturePath(t, "openapi-3.1.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	generatedCode, err := GenerateModelCode(document, "Widget")
	if err != nil {
		t.Fatalf("GenerateModelCode returned error: %v", err)
	}

	checks := []string{
		"type Widget struct",
		"Id string `json:\"id\"`",
		"Metadata map[string]string `json:\"metadata,omitempty\"`",
	}
	for _, check := range checks {
		if !strings.Contains(generatedCode, check) {
			t.Fatalf("expected generated code to contain %q\n%s", check, generatedCode)
		}
	}
}

func TestGenerateModelCodeMissingModel(t *testing.T) {
	document, err := openapi.ReadDefinitionFromFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}

	_, err = GenerateModelCode(document, "MissingModel")
	if err == nil {
		t.Fatal("expected missing model to fail")
	}
	if !strings.Contains(err.Error(), "model 'MissingModel' not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
