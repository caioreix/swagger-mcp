package openapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caioreix/swagger-mcp/internal/testutil"
)

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func cachePathsForURL(workingDir, url string) (string, string) {
	cacheKey := hashURL(url)
	cacheDir := filepath.Join(workingDir, "swagger-cache")
	return filepath.Join(cacheDir, cacheKey+".json"), filepath.Join(cacheDir, cacheKey+cacheMetadataSuffix)
}

func readTestCacheMetadata(t *testing.T, path string) cacheMetadata {
	t.Helper()
	metadata, err := readCacheMetadata(path)
	if err != nil {
		t.Fatalf("read cache metadata: %v", err)
	}
	return metadata
}

func TestReadDefinitionFromFileJSON(t *testing.T) {
	document, err := ReadDefinitionFromFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}
	if document["swagger"] != "2.0" {
		t.Fatalf("expected swagger version 2.0, got %#v", document["swagger"])
	}
}

func TestReadDefinitionFromFileYAML(t *testing.T) {
	document, err := ReadDefinitionFromFile(testutil.FixturePath(t, "date-time-test.yml"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}
	paths, ok := document["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected paths object, got %#v", document["paths"])
	}
	if _, ok := paths["/events"]; !ok {
		t.Fatalf("expected /events path in YAML fixture")
	}
}

func TestReadDefinitionFromFileOpenAPI31JSON(t *testing.T) {
	document, err := ReadDefinitionFromFile(testutil.FixturePath(t, "openapi-3.1.json"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}
	if document["openapi"] != "3.1.0" {
		t.Fatalf("expected openapi 3.1.0, got %#v", document["openapi"])
	}
}

func TestReadDefinitionFromFileOpenAPI31YAML(t *testing.T) {
	document, err := ReadDefinitionFromFile(testutil.FixturePath(t, "openapi-3.1.yml"))
	if err != nil {
		t.Fatalf("ReadDefinitionFromFile returned error: %v", err)
	}
	if document["openapi"] != "3.1.0" {
		t.Fatalf("expected openapi 3.1.0, got %#v", document["openapi"])
	}
}

func TestReadDefinitionRejectsUnsupportedDocument(t *testing.T) {
	temporaryDir := t.TempDir()
	path := filepath.Join(temporaryDir, "invalid.json")
	if err := os.WriteFile(path, []byte(`{"info":{"title":"invalid"}}`), 0o644); err != nil {
		t.Fatalf("write invalid fixture: %v", err)
	}
	_, err := ReadDefinitionFromFile(path)
	if err == nil {
		t.Fatal("expected invalid swagger document to fail")
	}
	if !strings.Contains(err.Error(), `missing required "openapi" or "swagger" field`) {
		t.Fatalf("expected missing openapi/swagger error, got %v", err)
	}
}

func TestSourceResolverUsesProjectMapping(t *testing.T) {
	temporaryDir := t.TempDir()
	fixtureBytes, err := os.ReadFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	swaggerPath := filepath.Join(temporaryDir, "petstore.json")
	if err := os.WriteFile(swaggerPath, fixtureBytes, 0o644); err != nil {
		t.Fatalf("write swagger fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(temporaryDir, ".swagger-mcp"), []byte("SWAGGER_FILEPATH="+swaggerPath+"\n"), 0o644); err != nil {
		t.Fatalf("write .swagger-mcp: %v", err)
	}

	resolver := NewSourceResolver(temporaryDir, "", silentLogger())
	document, err := resolver.Load("")
	if err != nil {
		t.Fatalf("resolver.Load returned error: %v", err)
	}
	if document["swagger"] != "2.0" {
		t.Fatalf("expected swagger version 2.0, got %#v", document["swagger"])
	}
}

func TestSourceResolverPrefersCLIURLOverInputPath(t *testing.T) {
	payload, err := os.ReadFile(testutil.FixturePath(t, "openapi-3.1.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	resolver := NewSourceResolver(t.TempDir(), server.URL, silentLogger())
	document, err := resolver.Load("/definitely/missing.json")
	if err != nil {
		t.Fatalf("resolver.Load returned error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected exactly one HTTP request, got %d", requests)
	}
	if document["openapi"] != "3.1.0" {
		t.Fatalf("expected openapi 3.1.0 document, got %#v", document["openapi"])
	}
}

func TestResolvePathErrorsWhenMappedFileMissing(t *testing.T) {
	temporaryDir := t.TempDir()
	mappedPath := filepath.Join(temporaryDir, "missing.json")
	if err := os.WriteFile(filepath.Join(temporaryDir, ".swagger-mcp"), []byte("SWAGGER_FILEPATH="+mappedPath+"\n"), 0o644); err != nil {
		t.Fatalf("write .swagger-mcp: %v", err)
	}

	resolver := NewSourceResolver(temporaryDir, "", silentLogger())
	_, err := resolver.ResolvePath("")
	if err == nil {
		t.Fatal("expected missing mapped file to fail")
	}
	if !strings.Contains(err.Error(), mappedPath) {
		t.Fatalf("expected missing mapped path in error, got %v", err)
	}
}

func TestDownloadDefinition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"swagger": "2.0",
			"paths":   map[string]any{},
		})
	}))
	defer server.Close()

	temporaryDir := t.TempDir()
	resolver := NewSourceResolver(temporaryDir, "", silentLogger())
	savedDefinition, err := resolver.DownloadDefinition(server.URL, temporaryDir)
	if err != nil {
		t.Fatalf("DownloadDefinition returned error: %v", err)
	}
	if filepath.Ext(savedDefinition.FilePath) != ".json" {
		t.Fatalf("expected .json saved file, got %s", savedDefinition.FilePath)
	}
	if _, err := os.Stat(savedDefinition.FilePath); err != nil {
		t.Fatalf("expected saved file to exist: %v", err)
	}
}

func TestDownloadDefinitionReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	resolver := NewSourceResolver(t.TempDir(), "", silentLogger())
	_, err := resolver.DownloadDefinition(server.URL, t.TempDir())
	if err == nil {
		t.Fatal("expected download failure")
	}
	if !strings.Contains(err.Error(), "unexpected status") {
		t.Fatalf("expected unexpected status in error, got %v", err)
	}
}

func TestCachedOrDownloadRevalidatesWithETag(t *testing.T) {
	payload, err := os.ReadFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	requests := 0
	conditionalRequests := 0
	const etag = `"petstore-v1"`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Header.Get("If-None-Match") == etag {
			conditionalRequests++
			writer.Header().Set("ETag", etag)
			writer.WriteHeader(http.StatusNotModified)
			return
		}
		if got := request.Header.Get("If-None-Match"); got != "" {
			t.Fatalf("unexpected If-None-Match header %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("ETag", etag)
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	temporaryDir := t.TempDir()
	resolver := NewSourceResolver(temporaryDir, server.URL, silentLogger())
	first, err := resolver.ResolvePath("")
	if err != nil {
		t.Fatalf("first ResolvePath returned error: %v", err)
	}
	second, err := resolver.ResolvePath("")
	if err != nil {
		t.Fatalf("second ResolvePath returned error: %v", err)
	}
	if first != second {
		t.Fatalf("expected cache path reuse, got %s and %s", first, second)
	}
	if requests != 2 {
		t.Fatalf("expected initial download plus conditional revalidation, got %d requests", requests)
	}
	if conditionalRequests != 1 {
		t.Fatalf("expected one conditional request, got %d", conditionalRequests)
	}

	cachePath, metadataPath := cachePathsForURL(temporaryDir, server.URL)
	if first != cachePath {
		t.Fatalf("expected cached path %s, got %s", cachePath, first)
	}
	metadata := readTestCacheMetadata(t, metadataPath)
	if metadata.ETag != etag {
		t.Fatalf("expected ETag %s, got %s", etag, metadata.ETag)
	}
	if metadata.ContentHash == "" {
		t.Fatal("expected content hash to be stored in metadata")
	}
	if metadata.ValidatedAt == "" {
		t.Fatal("expected validatedAt to be updated after 304 revalidation")
	}
}

func TestCachedOrDownloadRedownloadsWhenMetadataMissing(t *testing.T) {
	payload, err := os.ReadFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	requests := 0
	const etag = `"petstore-v1"`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if got := request.Header.Get("If-None-Match"); got != "" {
			t.Fatalf("expected missing metadata to force a fresh GET, got If-None-Match=%q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("ETag", etag)
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	temporaryDir := t.TempDir()
	resolver := NewSourceResolver(temporaryDir, server.URL, silentLogger())
	first, err := resolver.ResolvePath("")
	if err != nil {
		t.Fatalf("first ResolvePath returned error: %v", err)
	}

	_, metadataPath := cachePathsForURL(temporaryDir, server.URL)
	if err := os.Remove(metadataPath); err != nil {
		t.Fatalf("remove metadata: %v", err)
	}

	second, err := resolver.ResolvePath("")
	if err != nil {
		t.Fatalf("second ResolvePath returned error: %v", err)
	}
	if first != second {
		t.Fatalf("expected cache path reuse after metadata rebuild, got %s and %s", first, second)
	}
	if requests != 2 {
		t.Fatalf("expected redownload after metadata removal, got %d requests", requests)
	}
	metadata := readTestCacheMetadata(t, metadataPath)
	if metadata.ETag != etag {
		t.Fatalf("expected recreated metadata ETag %s, got %s", etag, metadata.ETag)
	}
}

func TestCachedOrDownloadRedownloadsWhenCachedFileHashMismatches(t *testing.T) {
	payload, err := os.ReadFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	temporaryDir := t.TempDir()
	resolver := NewSourceResolver(temporaryDir, server.URL, silentLogger())
	resolvedPath, err := resolver.ResolvePath("")
	if err != nil {
		t.Fatalf("first ResolvePath returned error: %v", err)
	}
	if err := os.WriteFile(resolvedPath, []byte("corrupted cache"), 0o644); err != nil {
		t.Fatalf("corrupt cache file: %v", err)
	}

	resolvedPath, err = resolver.ResolvePath("")
	if err != nil {
		t.Fatalf("second ResolvePath returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected redownload after hash mismatch, got %d requests", requests)
	}
	document, err := ReadDefinitionFromFile(resolvedPath)
	if err != nil {
		t.Fatalf("expected refreshed cache file to be readable: %v", err)
	}
	if document["swagger"] != "2.0" {
		t.Fatalf("expected refreshed swagger document, got %#v", document["swagger"])
	}
}

func TestCachedOrDownloadUpdatesCacheWhenRemoteContentChanges(t *testing.T) {
	initialPayload, err := os.ReadFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("read initial fixture: %v", err)
	}
	updatedPayload := []byte(`{"swagger":"2.0","info":{"title":"Updated API","version":"1.0.0"},"paths":{}}`)
	requests := 0
	const firstETag = `"petstore-v1"`
	const secondETag = `"petstore-v2"`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		writer.Header().Set("Content-Type", "application/json")
		switch requests {
		case 1:
			writer.Header().Set("ETag", firstETag)
			_, _ = writer.Write(initialPayload)
		case 2:
			if got := request.Header.Get("If-None-Match"); got != firstETag {
				t.Fatalf("expected If-None-Match=%s, got %q", firstETag, got)
			}
			writer.Header().Set("ETag", secondETag)
			_, _ = writer.Write(updatedPayload)
		default:
			t.Fatalf("unexpected request %d", requests)
		}
	}))
	defer server.Close()

	temporaryDir := t.TempDir()
	resolver := NewSourceResolver(temporaryDir, server.URL, silentLogger())
	resolvedPath, err := resolver.ResolvePath("")
	if err != nil {
		t.Fatalf("first ResolvePath returned error: %v", err)
	}

	_, metadataPath := cachePathsForURL(temporaryDir, server.URL)
	initialMetadata := readTestCacheMetadata(t, metadataPath)

	resolvedPath, err = resolver.ResolvePath("")
	if err != nil {
		t.Fatalf("second ResolvePath returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected remote update to trigger a second request, got %d", requests)
	}

	document, err := ReadDefinitionFromFile(resolvedPath)
	if err != nil {
		t.Fatalf("read refreshed cache file: %v", err)
	}
	info, ok := document["info"].(map[string]any)
	if !ok {
		t.Fatalf("expected info object, got %#v", document["info"])
	}
	if info["title"] != "Updated API" {
		t.Fatalf("expected updated title, got %#v", info["title"])
	}

	updatedMetadata := readTestCacheMetadata(t, metadataPath)
	if updatedMetadata.ETag != secondETag {
		t.Fatalf("expected updated ETag %s, got %s", secondETag, updatedMetadata.ETag)
	}
	if updatedMetadata.ContentHash == initialMetadata.ContentHash {
		t.Fatal("expected content hash to change after remote update")
	}
}

func TestCachedOrDownloadFallsBackToGETWhenValidatorsAreUnavailable(t *testing.T) {
	initialPayload, err := os.ReadFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("read initial fixture: %v", err)
	}
	updatedPayload := []byte(`{"swagger":"2.0","info":{"title":"Fallback API","version":"1.0.0"},"paths":{}}`)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if got := request.Header.Get("If-None-Match"); got != "" {
			t.Fatalf("expected fallback GET without If-None-Match, got %q", got)
		}
		if got := request.Header.Get("If-Modified-Since"); got != "" {
			t.Fatalf("expected fallback GET without If-Modified-Since, got %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			_, _ = writer.Write(initialPayload)
			return
		}
		_, _ = writer.Write(updatedPayload)
	}))
	defer server.Close()

	temporaryDir := t.TempDir()
	resolver := NewSourceResolver(temporaryDir, server.URL, silentLogger())
	resolvedPath, err := resolver.ResolvePath("")
	if err != nil {
		t.Fatalf("first ResolvePath returned error: %v", err)
	}
	_, metadataPath := cachePathsForURL(temporaryDir, server.URL)
	initialMetadata := readTestCacheMetadata(t, metadataPath)

	resolvedPath, err = resolver.ResolvePath("")
	if err != nil {
		t.Fatalf("second ResolvePath returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected fallback validation to fetch twice, got %d requests", requests)
	}

	document, err := ReadDefinitionFromFile(resolvedPath)
	if err != nil {
		t.Fatalf("read refreshed fallback cache file: %v", err)
	}
	info, ok := document["info"].(map[string]any)
	if !ok {
		t.Fatalf("expected info object, got %#v", document["info"])
	}
	if info["title"] != "Fallback API" {
		t.Fatalf("expected fallback-updated title, got %#v", info["title"])
	}

	updatedMetadata := readTestCacheMetadata(t, metadataPath)
	if updatedMetadata.ETag != "" {
		t.Fatalf("expected no ETag metadata when server does not send validators, got %q", updatedMetadata.ETag)
	}
	if updatedMetadata.LastModified != "" {
		t.Fatalf("expected no Last-Modified metadata when server does not send validators, got %q", updatedMetadata.LastModified)
	}
	if updatedMetadata.ContentHash == initialMetadata.ContentHash {
		t.Fatal("expected content hash to change after fallback refresh")
	}
}

func TestSourceResolverPreloadUsesLastModifiedForLaterLoads(t *testing.T) {
	payload, err := os.ReadFile(testutil.FixturePath(t, "petstore.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	requests := 0
	conditionalRequests := 0
	const lastModified = "Wed, 21 Oct 2015 07:28:00 GMT"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Header.Get("If-Modified-Since") == lastModified {
			conditionalRequests++
			writer.Header().Set("Last-Modified", lastModified)
			writer.WriteHeader(http.StatusNotModified)
			return
		}
		if got := request.Header.Get("If-Modified-Since"); got != "" {
			t.Fatalf("unexpected If-Modified-Since header %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Last-Modified", lastModified)
		_, _ = writer.Write(payload)
	}))
	defer server.Close()

	temporaryDir := t.TempDir()
	resolver := NewSourceResolver(temporaryDir, server.URL, silentLogger())
	if err := resolver.Preload(); err != nil {
		t.Fatalf("Preload returned error: %v", err)
	}
	resolvedPath, err := resolver.ResolvePath("")
	if err != nil {
		t.Fatalf("ResolvePath returned error after preload: %v", err)
	}
	if _, err := os.Stat(resolvedPath); err != nil {
		t.Fatalf("expected preloaded swagger file to exist: %v", err)
	}
	if requests != 2 {
		t.Fatalf("expected preload plus conditional revalidation, got %d requests", requests)
	}
	if conditionalRequests != 1 {
		t.Fatalf("expected one If-Modified-Since request, got %d", conditionalRequests)
	}

	_, metadataPath := cachePathsForURL(temporaryDir, server.URL)
	metadata := readTestCacheMetadata(t, metadataPath)
	if metadata.LastModified != lastModified {
		t.Fatalf("expected Last-Modified %q, got %q", lastModified, metadata.LastModified)
	}
}
