package codegen

import (
	"fmt"
	"strings"

	"github.com/caioreix/swagger-mcp/internal/openapi"
)

// generateAuthHelpers returns Go helper functions for authentication in
// generated MCP tool handlers.
func generateAuthHelpers(schemes []openapi.SecurityScheme) string {
	if len(schemes) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("// applyAuth sets authentication headers/params on the HTTP request\n")
	b.WriteString("// based on environment variables.\n")
	b.WriteString("func applyAuth(req *http.Request) {\n")

	for _, scheme := range schemes {
		switch {
		case scheme.Type == "apiKey":
			b.WriteString(generateAPIKeyAuth(scheme))
		case scheme.Type == "http" && scheme.Scheme == "bearer":
			b.WriteString(generateBearerAuth())
		case scheme.Type == "http" && scheme.Scheme == "basic":
			b.WriteString(generateBasicAuth())
		case scheme.Type == "oauth2":
			b.WriteString(generateOAuth2Auth(scheme))
		}
	}

	b.WriteString("}\n")
	return b.String()
}

func generateAPIKeyAuth(scheme openapi.SecurityScheme) string {
	headerName := scheme.ParamName
	if headerName == "" {
		headerName = "X-API-Key"
	}
	location := scheme.In
	if location == "" {
		location = "header"
	}

	if location == "query" {
		return fmt.Sprintf(`	if apiKey := os.Getenv("API_KEY"); apiKey != "" {
		q := req.URL.Query()
		q.Set(%q, apiKey)
		req.URL.RawQuery = q.Encode()
	}
`, headerName)
	}
	// header or cookie
	return fmt.Sprintf(`	if apiKey := os.Getenv("API_KEY"); apiKey != "" {
		req.Header.Set(%q, apiKey)
	}
`, headerName)
}

func generateBearerAuth() string {
	return `	if token := os.Getenv("BEARER_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
`
}

func generateBasicAuth() string {
	return `	if user := os.Getenv("BASIC_AUTH_USER"); user != "" {
		req.SetBasicAuth(user, os.Getenv("BASIC_AUTH_PASS"))
	}
`
}

func generateOAuth2Auth(scheme openapi.SecurityScheme) string {
	return `	if clientID := os.Getenv("OAUTH2_CLIENT_ID"); clientID != "" {
		token := fetchOAuth2Token()
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
`
}

// generateOAuth2TokenFetcher returns Go code for an OAuth2 client_credentials
// token fetcher with simple caching.
func generateOAuth2TokenFetcher() string {
	return `var (
	cachedOAuth2Token     string
	cachedOAuth2TokenTime time.Time
)

func fetchOAuth2Token() string {
	if cachedOAuth2Token != "" && time.Since(cachedOAuth2TokenTime) < 50*time.Minute {
		return cachedOAuth2Token
	}

	tokenURL := os.Getenv("OAUTH2_TOKEN_URL")
	clientID := os.Getenv("OAUTH2_CLIENT_ID")
	clientSecret := os.Getenv("OAUTH2_CLIENT_SECRET")
	scopes := os.Getenv("OAUTH2_SCOPES")

	if tokenURL == "" || clientID == "" {
		return ""
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}
	if scopes != "" {
		data.Set("scope", strings.ReplaceAll(scopes, ",", " "))
	}

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string ` + "`json:\"access_token\"`" + `
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	cachedOAuth2Token = result.AccessToken
	cachedOAuth2TokenTime = time.Now()
	return cachedOAuth2Token
}
`
}

// generateAuthSetup generates the auth setup block that goes inside a handler function.
func generateAuthSetup(schemes []openapi.SecurityScheme) string {
	if len(schemes) == 0 {
		return ""
	}
	return "\tapplyAuth(req)\n"
}

// needsOAuth2Fetcher returns true if any scheme is OAuth2.
func needsOAuth2Fetcher(schemes []openapi.SecurityScheme) bool {
	for _, s := range schemes {
		if s.Type == "oauth2" {
			return true
		}
	}
	return false
}

// authImports returns the import paths needed for auth code.
func authImports(schemes []openapi.SecurityScheme) []string {
	if len(schemes) == 0 {
		return nil
	}
	imports := []string{"net/http", "os"}
	if needsOAuth2Fetcher(schemes) {
		imports = append(imports, "encoding/json", "net/url", "strings", "time")
	}
	return imports
}
