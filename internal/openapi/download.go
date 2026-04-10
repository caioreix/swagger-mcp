package openapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const cacheMetadataSuffix = ".metadata.json"

var errInvalidCacheMetadata = errors.New("invalid cached swagger metadata")

type cacheMetadata struct {
	URL            string `json:"url"`
	CacheKey       string `json:"cacheKey"`
	ContentHash    string `json:"contentHash"`
	DefinitionType string `json:"definitionType"`
	DownloadedAt   string `json:"downloadedAt"`
	ValidatedAt    string `json:"validatedAt,omitempty"`
	ETag           string `json:"etag,omitempty"`
	LastModified   string `json:"lastModified,omitempty"`
}

type definitionFetchResult struct {
	document       map[string]any
	definitionType string
	etag           string
	lastModified   string
	statusCode     int
}

func (r SourceResolver) DownloadDefinition(url, saveLocation string) (SavedDefinition, error) {
	if strings.TrimSpace(url) == "" {
		return SavedDefinition{}, errors.New("URL is required")
	}
	if strings.TrimSpace(saveLocation) == "" {
		return SavedDefinition{}, errors.New("save location is required")
	}

	fetched, err := r.fetchDefinition(url, nil)
	if err != nil {
		return SavedDefinition{}, err
	}

	if mkdirErr := os.MkdirAll(saveLocation, 0o750); mkdirErr != nil {
		return SavedDefinition{}, fmt.Errorf("create save directory: %w", mkdirErr)
	}

	filePath := filepath.Join(saveLocation, hashURL(url)+".json")
	formatted, _, err := formatDefinition(fetched.document)
	if err != nil {
		return SavedDefinition{}, fmt.Errorf("marshal swagger definition: %w", err)
	}
	if writeErr := os.WriteFile(filePath, formatted, 0o600); writeErr != nil {
		return SavedDefinition{}, fmt.Errorf("write swagger definition: %w", writeErr)
	}

	r.logger.Info("downloaded swagger definition", "url", url, "path", filePath, "type", fetched.definitionType)
	return SavedDefinition{FilePath: filePath, URL: url, Type: fetched.definitionType}, nil
}

func (r SourceResolver) cachedOrDownload(url string) (string, error) {
	cacheDir := filepath.Join(r.WorkingDir, "swagger-cache")
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return "", fmt.Errorf("create cache directory: %w", err)
	}

	cacheKey := hashURL(url)
	cachePath := filepath.Join(cacheDir, cacheKey+".json")
	metadataPath := filepath.Join(cacheDir, cacheKey+cacheMetadataSuffix)

	metadata, validCache, err := r.loadValidCache(cachePath, metadataPath, url, cacheKey)
	if err != nil {
		return "", err
	}

	if !validCache {
		fetched, fetchErr := r.fetchDefinition(url, nil)
		if fetchErr != nil {
			return "", fetchErr
		}
		if writeErr := writeCachedDefinition(
			cachePath,
			metadataPath,
			newCacheMetadata(url, cacheKey, fetched, currentTimestamp()),
			fetched.document,
		); writeErr != nil {
			return "", writeErr
		}
		r.logger.Info("cached swagger definition", "url", url, "path", cachePath)
		return cachePath, nil
	}

	fetched, err := r.fetchDefinition(url, &metadata)
	if err != nil {
		return "", err
	}

	validationTimestamp := currentTimestamp()
	if fetched.statusCode == http.StatusNotModified {
		if writeMetaErr := writeCacheMetadata(
			metadataPath,
			metadataForNotModified(metadata, fetched, validationTimestamp),
		); writeMetaErr != nil {
			return "", writeMetaErr
		}
		r.logger.Info("reusing cached swagger definition", "url", url, "path", cachePath, "result", "not_modified")
		return cachePath, nil
	}

	content, contentHash, err := formatDefinition(fetched.document)
	if err != nil {
		return "", fmt.Errorf("marshal cached swagger definition: %w", err)
	}
	if contentHash == metadata.ContentHash {
		if writeMetaErr := writeCacheMetadata(
			metadataPath,
			newCacheMetadata(url, cacheKey, fetched, validationTimestamp).withContentHash(contentHash),
		); writeMetaErr != nil {
			return "", writeMetaErr
		}
		r.logger.Info("reusing cached swagger definition", "url", url, "path", cachePath, "result", "unchanged")
		return cachePath, nil
	}

	if writeErr := writeCachedDefinitionContent(
		cachePath,
		metadataPath,
		newCacheMetadata(url, cacheKey, fetched, validationTimestamp).withContentHash(contentHash),
		content,
	); writeErr != nil {
		return "", writeErr
	}
	r.logger.Info("updated cached swagger definition", "url", url, "path", cachePath)
	return cachePath, nil
}

func (r SourceResolver) fetchDefinition(url string, metadata *cacheMetadata) (definitionFetchResult, error) {
	conditionalRequest := metadata != nil && (metadata.ETag != "" || metadata.LastModified != "")
	r.logger.Info("fetching swagger definition", "url", url, "conditional", conditionalRequest)

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return definitionFetchResult{}, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Accept", "application/json, application/yaml, text/yaml, application/x-yaml")
	if metadata != nil {
		if metadata.ETag != "" {
			request.Header.Set("If-None-Match", metadata.ETag)
		}
		if metadata.LastModified != "" {
			request.Header.Set("If-Modified-Since", metadata.LastModified)
		}
	}

	response, err := r.Client.Do(request)
	if err != nil {
		return definitionFetchResult{}, fmt.Errorf("failed to download Swagger definition from %s: %w", url, err)
	}
	defer response.Body.Close()

	result := definitionFetchResult{
		etag:         strings.TrimSpace(response.Header.Get("ETag")),
		lastModified: strings.TrimSpace(response.Header.Get("Last-Modified")),
		statusCode:   response.StatusCode,
	}
	if response.StatusCode == http.StatusNotModified {
		r.logger.Debug("swagger definition not modified", "url", url)
		return result, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		r.logger.Warn("swagger definition request failed", "url", url, "status", response.Status)
		return definitionFetchResult{}, fmt.Errorf(
			"failed to download Swagger definition from %s: unexpected status %s",
			url,
			response.Status,
		)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return definitionFetchResult{}, fmt.Errorf("read Swagger response body: %w", err)
	}

	document, definitionType, err := parseDefinitionBytes(body, response.Header.Get("Content-Type"))
	if err != nil {
		return definitionFetchResult{}, fmt.Errorf("failed to parse Swagger definition from %s: %w", url, err)
	}
	result.document = document
	result.definitionType = definitionType
	r.logger.Debug("fetched swagger definition", "url", url, "type", definitionType)
	return result, nil
}

func (r SourceResolver) loadValidCache(cachePath, metadataPath, url, cacheKey string) (cacheMetadata, bool, error) {
	content, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return cacheMetadata{}, false, nil
		}
		return cacheMetadata{}, false, fmt.Errorf("read cached swagger definition %s: %w", cachePath, err)
	}

	metadata, err := readCacheMetadata(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			r.logger.Warn("cached swagger metadata missing; refreshing cache", "url", url, "path", metadataPath)
			return cacheMetadata{}, false, nil
		}
		if errors.Is(err, errInvalidCacheMetadata) {
			r.logger.Warn(
				"cached swagger metadata is invalid; refreshing cache",
				"url",
				url,
				"path",
				metadataPath,
				"error",
				err.Error(),
			)
			return cacheMetadata{}, false, nil
		}
		return cacheMetadata{}, false, err
	}

	if validErr := validateCacheMetadata(metadata, url, cacheKey); validErr != nil {
		r.logger.Warn(
			"cached swagger metadata does not match cache entry; refreshing cache",
			"url",
			url,
			"path",
			metadataPath,
			"error",
			validErr.Error(),
		)
		return cacheMetadata{}, false, nil
	}
	if hashContent(content) != metadata.ContentHash {
		r.logger.Warn("cached swagger definition hash mismatch; refreshing cache", "url", url, "path", cachePath)
		return cacheMetadata{}, false, nil
	}
	return metadata, true, nil
}

func formatDefinition(document map[string]any) ([]byte, string, error) {
	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return content, hashContent(content), nil
}

func writeCachedDefinition(cachePath, metadataPath string, metadata cacheMetadata, document map[string]any) error {
	content, contentHash, err := formatDefinition(document)
	if err != nil {
		return fmt.Errorf("marshal cached swagger definition: %w", err)
	}
	return writeCachedDefinitionContent(cachePath, metadataPath, metadata.withContentHash(contentHash), content)
}

func writeCachedDefinitionContent(cachePath, metadataPath string, metadata cacheMetadata, content []byte) error {
	if err := os.WriteFile(cachePath, content, 0o600); err != nil {
		return fmt.Errorf("write cached swagger definition: %w", err)
	}
	if err := writeCacheMetadata(metadataPath, metadata); err != nil {
		return err
	}
	return nil
}

func readCacheMetadata(path string) (cacheMetadata, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return cacheMetadata{}, err
	}
	var metadata cacheMetadata
	if unmarshalErr := json.Unmarshal(content, &metadata); unmarshalErr != nil {
		return cacheMetadata{}, fmt.Errorf("%w: %w", errInvalidCacheMetadata, unmarshalErr)
	}
	metadata.URL = strings.TrimSpace(metadata.URL)
	metadata.CacheKey = strings.TrimSpace(metadata.CacheKey)
	metadata.ContentHash = strings.TrimSpace(metadata.ContentHash)
	metadata.DefinitionType = strings.TrimSpace(metadata.DefinitionType)
	metadata.DownloadedAt = strings.TrimSpace(metadata.DownloadedAt)
	metadata.ValidatedAt = strings.TrimSpace(metadata.ValidatedAt)
	metadata.ETag = strings.TrimSpace(metadata.ETag)
	metadata.LastModified = strings.TrimSpace(metadata.LastModified)
	return metadata, nil
}

func writeCacheMetadata(path string, metadata cacheMetadata) error {
	content, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cached swagger metadata: %w", err)
	}
	if writeErr := os.WriteFile(path, content, 0o600); writeErr != nil {
		return fmt.Errorf("write cached swagger metadata: %w", writeErr)
	}
	return nil
}

func validateCacheMetadata(metadata cacheMetadata, url, cacheKey string) error {
	switch {
	case metadata.URL == "":
		return errors.New("missing url")
	case metadata.URL != url:
		return errors.New("url mismatch")
	case metadata.CacheKey == "":
		return errors.New("missing cache key")
	case metadata.CacheKey != cacheKey:
		return errors.New("cache key mismatch")
	case metadata.ContentHash == "":
		return errors.New("missing content hash")
	default:
		return nil
	}
}

func newCacheMetadata(url, cacheKey string, fetched definitionFetchResult, timestamp string) cacheMetadata {
	return cacheMetadata{
		URL:            strings.TrimSpace(url),
		CacheKey:       strings.TrimSpace(cacheKey),
		DefinitionType: strings.TrimSpace(fetched.definitionType),
		DownloadedAt:   timestamp,
		ValidatedAt:    timestamp,
		ETag:           strings.TrimSpace(fetched.etag),
		LastModified:   strings.TrimSpace(fetched.lastModified),
	}
}

func metadataForNotModified(existing cacheMetadata, fetched definitionFetchResult, timestamp string) cacheMetadata {
	existing.ValidatedAt = timestamp
	if fetched.etag != "" {
		existing.ETag = strings.TrimSpace(fetched.etag)
	}
	if fetched.lastModified != "" {
		existing.LastModified = strings.TrimSpace(fetched.lastModified)
	}
	return existing
}

func (metadata cacheMetadata) withContentHash(contentHash string) cacheMetadata {
	metadata.ContentHash = strings.TrimSpace(contentHash)
	return metadata
}

func currentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func hashContent(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func hashURL(url string) string {
	return hashContent([]byte(url))
}
