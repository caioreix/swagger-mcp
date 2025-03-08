package openapi

import (
	"fmt"
	"regexp"
	"strings"
)

// EndpointFilter holds compiled regex patterns and method lists for filtering endpoints.
type EndpointFilter struct {
	IncludePaths   []*regexp.Regexp
	ExcludePaths   []*regexp.Regexp
	IncludeMethods map[string]bool
	ExcludeMethods map[string]bool
}

// NewEndpointFilter compiles the given patterns into an EndpointFilter.
// Each paths argument is a comma-separated list of regex patterns.
// Each methods argument is a comma-separated list of HTTP methods.
func NewEndpointFilter(includePaths, excludePaths, includeMethods, excludeMethods string) (*EndpointFilter, error) {
	f := &EndpointFilter{}
	var err error
	if f.IncludePaths, err = compilePatterns(includePaths); err != nil {
		return nil, fmt.Errorf("include-paths: %w", err)
	}
	if f.ExcludePaths, err = compilePatterns(excludePaths); err != nil {
		return nil, fmt.Errorf("exclude-paths: %w", err)
	}
	f.IncludeMethods = parseMethods(includeMethods)
	f.ExcludeMethods = parseMethods(excludeMethods)
	return f, nil
}

// IsEmpty returns true if no filters are configured.
func (f *EndpointFilter) IsEmpty() bool {
	return f == nil ||
		(len(f.IncludePaths) == 0 && len(f.ExcludePaths) == 0 &&
			len(f.IncludeMethods) == 0 && len(f.ExcludeMethods) == 0)
}

// Match returns true if the given path and method pass the filter.
func (f *EndpointFilter) Match(path, method string) bool {
	if f.IsEmpty() {
		return true
	}
	upperMethod := strings.ToUpper(method)

	if len(f.IncludeMethods) > 0 && !f.IncludeMethods[upperMethod] {
		return false
	}
	if len(f.ExcludeMethods) > 0 && f.ExcludeMethods[upperMethod] {
		return false
	}

	if len(f.IncludePaths) > 0 {
		matched := false
		for _, re := range f.IncludePaths {
			if re.MatchString(path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	for _, re := range f.ExcludePaths {
		if re.MatchString(path) {
			return false
		}
	}

	return true
}

// FilterEndpoints returns only the endpoints that pass the filter.
func FilterEndpoints(endpoints []Endpoint, filter *EndpointFilter) []Endpoint {
	if filter == nil || filter.IsEmpty() {
		return endpoints
	}
	filtered := make([]Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if filter.Match(ep.Path, ep.Method) {
			filtered = append(filtered, ep)
		}
	}
	return filtered
}

func compilePatterns(raw string) ([]*regexp.Regexp, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	patterns := make([]*regexp.Regexp, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", p, err)
		}
		patterns = append(patterns, re)
	}
	return patterns, nil
}

func parseMethods(raw string) map[string]bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	methods := make(map[string]bool)
	for _, m := range strings.Split(raw, ",") {
		m = strings.TrimSpace(strings.ToUpper(m))
		if m != "" {
			methods[m] = true
		}
	}
	if len(methods) == 0 {
		return nil
	}
	return methods
}
