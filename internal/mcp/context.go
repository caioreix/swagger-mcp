package mcp

import "context"

type proxyHeadersKey struct{}

// WithProxyHeaders returns a context carrying extra HTTP headers to forward to proxy calls.
func WithProxyHeaders(ctx context.Context, headers map[string]string) context.Context {
	if len(headers) == 0 {
		return ctx
	}
	return context.WithValue(ctx, proxyHeadersKey{}, headers)
}

// ProxyHeadersFromContext retrieves proxy headers injected by the transport layer.
func ProxyHeadersFromContext(ctx context.Context) map[string]string {
	if h, ok := ctx.Value(proxyHeadersKey{}).(map[string]string); ok {
		return h
	}
	return nil
}
