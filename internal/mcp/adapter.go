package mcp

import (
	"context"
	"encoding/json"

	mcpgoserver "github.com/mark3labs/mcp-go/server"
)

// ServerAdapter wraps a mcp-go MCPServer and exposes HandleJSON / HandleJSONWithHeaders
// for use with transport layers that speak raw JSON-RPC (SSE, stdio, web UI).
type ServerAdapter struct {
	server *mcpgoserver.MCPServer
}

// NewServerAdapter creates an adapter that forwards raw JSON-RPC messages to the
// underlying mcp-go MCPServer via its HandleMessage method.
func NewServerAdapter(s *mcpgoserver.MCPServer) *ServerAdapter {
	return &ServerAdapter{server: s}
}

// HandleJSON processes a single JSON-RPC message and returns the response bytes.
// Returns nil, nil for notifications (no ID).
func (a *ServerAdapter) HandleJSON(line []byte) ([]byte, error) {
	return a.HandleJSONWithHeaders(line, nil)
}

// HandleJSONWithHeaders processes a JSON-RPC message with extra HTTP headers
// injected into the context (for proxy tool forwarding).
func (a *ServerAdapter) HandleJSONWithHeaders(line []byte, extraHeaders map[string]string) ([]byte, error) {
	ctx := context.Background()
	if len(extraHeaders) > 0 {
		ctx = WithProxyHeaders(ctx, extraHeaders)
	}

	response := a.server.HandleMessage(ctx, json.RawMessage(line))
	if response == nil {
		return nil, nil
	}

	data, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return data, nil
}
