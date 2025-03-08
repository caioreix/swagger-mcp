package testutil

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
)

func JSONRPCRequest(tb testing.TB, id any, method string, params any) []byte {
	tb.Helper()

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		payload["params"] = params
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		tb.Fatalf("marshal JSON-RPC request: %v", err)
	}
	return encoded
}

func DecodeJSONMap(tb testing.TB, data []byte) map[string]any {
	tb.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		tb.Fatalf("unmarshal JSON object: %v\n%s", err, string(data))
	}
	return decoded
}

func DecodeJSONLines(tb testing.TB, data string) []map[string]any {
	tb.Helper()

	scanner := bufio.NewScanner(strings.NewReader(data))
	responses := make([]map[string]any, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		responses = append(responses, DecodeJSONMap(tb, []byte(line)))
	}
	if err := scanner.Err(); err != nil {
		tb.Fatalf("scan JSON lines: %v", err)
	}
	return responses
}
