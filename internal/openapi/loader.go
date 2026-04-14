package openapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/caioreix/swagger-mcp/internal/logging"
)

const httpClientTimeout = 30 * time.Second

type SavedDefinition struct {
	FilePath string `json:"filePath"`
	URL      string `json:"url"`
	Type     string `json:"type"`
}

type SourceResolver struct {
	WorkingDir string
	SwaggerURL string
	Client     *http.Client
	logger     *slog.Logger
	cacheMu    *sync.Mutex
}

func NewSourceResolver(workingDir, swaggerURL string, loggers ...*slog.Logger) SourceResolver {
	logger := slog.Default()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	return SourceResolver{
		WorkingDir: workingDir,
		SwaggerURL: strings.TrimSpace(swaggerURL),
		Client:     &http.Client{Timeout: httpClientTimeout},
		logger:     logging.WithComponent(logger, "openapi.resolver"),
		cacheMu:    &sync.Mutex{},
	}
}

func (r SourceResolver) Load(ctx context.Context, swaggerFilePath string) (map[string]any, error) {
	resolvedPath, err := r.ResolvePath(ctx, swaggerFilePath)
	if err != nil {
		return nil, err
	}
	document, err := ReadDefinitionFromFile(resolvedPath)
	if err != nil {
		return nil, err
	}
	r.logger.Debug("loaded swagger definition from disk", "path", resolvedPath)
	return document, nil
}

func (r SourceResolver) Preload() error {
	if r.SwaggerURL == "" {
		return nil
	}
	r.logger.Info("preloading swagger definition from startup URL", "url", r.SwaggerURL)
	if _, err := r.cachedOrDownload(context.Background(), r.SwaggerURL); err != nil {
		return fmt.Errorf("preload swagger definition from %s: %w", r.SwaggerURL, err)
	}
	return nil
}

func (r SourceResolver) ResolvePath(ctx context.Context, swaggerFilePath string) (string, error) {
	if r.SwaggerURL != "" {
		r.logger.Debug("resolving swagger definition from startup URL", "url", r.SwaggerURL)
		return r.cachedOrDownload(ctx, r.SwaggerURL)
	}

	if strings.TrimSpace(swaggerFilePath) != "" {
		candidate := swaggerFilePath
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(r.WorkingDir, candidate)
		}
		if _, err := os.Stat(candidate); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("swagger file not found at %s", candidate)
			}
			return "", fmt.Errorf("stat swagger file %s: %w", candidate, err)
		}
		r.logger.Debug("resolving swagger definition from explicit file path", "path", candidate)
		return candidate, nil
	}

	mappedPath, err := readProjectMapping(r.WorkingDir)
	if err == nil {
		if _, statErr := os.Stat(mappedPath); statErr == nil {
			r.logger.Debug("resolving swagger definition from project mapping", "path", mappedPath)
			return mappedPath, nil
		}
		r.logger.Warn("project mapping points to missing swagger file", "path", mappedPath)
		return "", fmt.Errorf("swagger file from .swagger-mcp not found at %s", mappedPath)
	}

	return "", errors.New(
		"swagger URL or file path is required. Provide --swagger-url=<url>, swaggerFilePath parameter, or .swagger-mcp mapping",
	)
}
