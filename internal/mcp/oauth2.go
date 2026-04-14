package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/caioreix/swagger-mcp/internal/config"
)

// proxyHTTPClient is shared across proxy and OAuth2 token requests.
var proxyHTTPClient = &http.Client{Timeout: httpClientTimeoutSeconds * time.Second}

type oauth2Token struct {
	AccessToken string
	ExpiresAt   time.Time
}

func (t *oauth2Token) IsExpired() bool {
	// Refresh 30 seconds before actual expiry.
	return time.Now().Add(30 * time.Second).After(t.ExpiresAt)
}

type oauth2TokenCache struct {
	mu    sync.Mutex
	cache map[string]*oauth2Token // key: tokenURL+"|"+clientID
}

var globalOAuth2Cache = &oauth2TokenCache{
	cache: make(map[string]*oauth2Token),
}

func (c *oauth2TokenCache) Get(key string) (*oauth2Token, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.cache[key]
	if !ok || t.IsExpired() {
		return nil, false
	}
	return t, true
}

func (c *oauth2TokenCache) Set(key string, token *oauth2Token) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = token
}

// fetchOAuth2Token performs the client_credentials grant and returns a cached token.
func fetchOAuth2Token(ctx context.Context, auth config.AuthConfig) (string, error) {
	cacheKey := auth.OAuth2URL + "|" + auth.OAuth2ID
	if t, ok := globalOAuth2Cache.Get(cacheKey); ok {
		return t.AccessToken, nil
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {auth.OAuth2ID},
		"client_secret": {auth.OAuth2Secret},
	}
	if auth.OAuth2Scopes != "" {
		data.Set("scope", strings.ReplaceAll(auth.OAuth2Scopes, ",", " "))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, auth.OAuth2URL,
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("create oauth2 token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := proxyHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth2 token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("oauth2 token request failed with status %s", resp.Status)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode oauth2 token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("oauth2 token response missing access_token")
	}

	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600 // default 1 hour
	}

	token := &oauth2Token{
		AccessToken: tokenResp.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(expiresIn) * time.Second),
	}
	globalOAuth2Cache.Set(cacheKey, token)

	return token.AccessToken, nil
}
