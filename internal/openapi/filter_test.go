package openapi

import "testing"

func TestEndpointFilterMatchAll(t *testing.T) {
	filter, err := NewEndpointFilter("", "", "", "")
	if err != nil {
		t.Fatalf("NewEndpointFilter: %v", err)
	}
	if !filter.Match("/pets", "GET") {
		t.Fatal("empty filter should match everything")
	}
	if !filter.IsEmpty() {
		t.Fatal("empty filter should report IsEmpty")
	}
}

func TestEndpointFilterIncludePaths(t *testing.T) {
	filter, err := NewEndpointFilter("^/pets.*", "", "", "")
	if err != nil {
		t.Fatalf("NewEndpointFilter: %v", err)
	}
	if !filter.Match("/pets", "GET") {
		t.Fatal("should match /pets")
	}
	if !filter.Match("/pets/123", "GET") {
		t.Fatal("should match /pets/123")
	}
	if filter.Match("/users", "GET") {
		t.Fatal("should not match /users")
	}
}

func TestEndpointFilterExcludePaths(t *testing.T) {
	filter, err := NewEndpointFilter("", ".*delete.*", "", "")
	if err != nil {
		t.Fatalf("NewEndpointFilter: %v", err)
	}
	if !filter.Match("/pets", "GET") {
		t.Fatal("should match /pets")
	}
	if filter.Match("/pets/delete", "GET") {
		t.Fatal("should not match /pets/delete")
	}
}

func TestEndpointFilterIncludeMethods(t *testing.T) {
	filter, err := NewEndpointFilter("", "", "GET,POST", "")
	if err != nil {
		t.Fatalf("NewEndpointFilter: %v", err)
	}
	if !filter.Match("/pets", "GET") {
		t.Fatal("should match GET")
	}
	if !filter.Match("/pets", "post") {
		t.Fatal("should match POST (case-insensitive)")
	}
	if filter.Match("/pets", "DELETE") {
		t.Fatal("should not match DELETE")
	}
}

func TestEndpointFilterExcludeMethods(t *testing.T) {
	filter, err := NewEndpointFilter("", "", "", "DELETE,PATCH")
	if err != nil {
		t.Fatalf("NewEndpointFilter: %v", err)
	}
	if !filter.Match("/pets", "GET") {
		t.Fatal("should match GET")
	}
	if filter.Match("/pets", "DELETE") {
		t.Fatal("should not match DELETE")
	}
}

func TestEndpointFilterCombined(t *testing.T) {
	filter, err := NewEndpointFilter("^/pets.*,^/users.*", ".*admin.*", "GET,POST", "")
	if err != nil {
		t.Fatalf("NewEndpointFilter: %v", err)
	}
	if !filter.Match("/pets", "GET") {
		t.Fatal("should match /pets GET")
	}
	if filter.Match("/pets", "DELETE") {
		t.Fatal("should not match /pets DELETE (method not included)")
	}
	if filter.Match("/orders", "GET") {
		t.Fatal("should not match /orders (path not included)")
	}
	if filter.Match("/pets/admin", "GET") {
		t.Fatal("should not match /pets/admin (path excluded)")
	}
	if filter.Match("/users/admin/settings", "POST") {
		t.Fatal("should not match /users/admin/settings (path excluded)")
	}
}

func TestEndpointFilterInvalidRegex(t *testing.T) {
	_, err := NewEndpointFilter("[invalid", "", "", "")
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestEndpointFilterMultipleIncludePatterns(t *testing.T) {
	filter, err := NewEndpointFilter("^/v1/.*,^/v2/.*", "", "", "")
	if err != nil {
		t.Fatalf("NewEndpointFilter: %v", err)
	}
	if !filter.Match("/v1/pets", "GET") {
		t.Fatal("should match /v1/pets")
	}
	if !filter.Match("/v2/users", "GET") {
		t.Fatal("should match /v2/users")
	}
	if filter.Match("/v3/orders", "GET") {
		t.Fatal("should not match /v3/orders")
	}
}

func TestFilterEndpoints(t *testing.T) {
	endpoints := []Endpoint{
		{Path: "/pets", Method: "GET"},
		{Path: "/pets", Method: "POST"},
		{Path: "/pets/{id}", Method: "GET"},
		{Path: "/pets/{id}", Method: "DELETE"},
		{Path: "/users", Method: "GET"},
	}

	filter, err := NewEndpointFilter("^/pets.*", "", "GET", "")
	if err != nil {
		t.Fatalf("NewEndpointFilter: %v", err)
	}

	filtered := FilterEndpoints(endpoints, filter)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered endpoints, got %d", len(filtered))
	}
	if filtered[0].Path != "/pets" || filtered[0].Method != "GET" {
		t.Fatalf("expected /pets GET, got %s %s", filtered[0].Path, filtered[0].Method)
	}
	if filtered[1].Path != "/pets/{id}" || filtered[1].Method != "GET" {
		t.Fatalf("expected /pets/{id} GET, got %s %s", filtered[1].Path, filtered[1].Method)
	}
}

func TestFilterEndpointsNilFilter(t *testing.T) {
	endpoints := []Endpoint{{Path: "/pets", Method: "GET"}}
	filtered := FilterEndpoints(endpoints, nil)
	if len(filtered) != 1 {
		t.Fatalf("expected nil filter to return all endpoints, got %d", len(filtered))
	}
}
