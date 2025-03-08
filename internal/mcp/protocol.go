package mcp

import "encoding/json"

const latestProtocolVersion = "2024-11-05"

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
}

func (s *server) respond(id any, result any) ([]byte, error) {
	return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *server) errorResponse(id any, code int, message string) jsonRPCResponse {
	return jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: map[string]any{"code": code, "message": message}}
}
