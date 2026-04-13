package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// mockClientSession implements mcpgoserver.ClientSession for testing.
type mockClientSession struct {
	id string
}

func (m *mockClientSession) SessionID() string                                    { return m.id }
func (m *mockClientSession) Initialize()                                          {}
func (m *mockClientSession) Initialized() bool                                    { return true }
func (m *mockClientSession) NotificationChannel() chan<- mcpgo.JSONRPCNotification {
	return make(chan<- mcpgo.JSONRPCNotification, 1)
}

func newTestAuditLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func decodeAuditLogEntry(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to decode log entry: %v\nraw: %s", err, buf.Bytes())
	}
	return entry
}

// TestSanitizeArguments tests sanitizeArguments with table-driven cases.
func TestSanitizeArguments(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]any
		want  map[string]any
	}{
		{
			name:  "nil input",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty map",
			input: map[string]any{},
			want:  map[string]any{},
		},
		{
			name:  "token key redacted",
			input: map[string]any{"token": "secret"},
			want:  map[string]any{"token": "[REDACTED]"},
		},
		{
			name:  "password key redacted",
			input: map[string]any{"password": "hunter2"},
			want:  map[string]any{"password": "[REDACTED]"},
		},
		{
			name:  "authorization key redacted",
			input: map[string]any{"authorization": "Bearer abc"},
			want:  map[string]any{"authorization": "[REDACTED]"},
		},
		{
			name:  "api_key redacted (contains key)",
			input: map[string]any{"api_key": "mykey"},
			want:  map[string]any{"api_key": "[REDACTED]"},
		},
		{
			name:  "username not redacted",
			input: map[string]any{"username": "admin"},
			want:  map[string]any{"username": "admin"},
		},
		{
			name: "mixed sensitive and non-sensitive",
			input: map[string]any{
				"token":    "secret",
				"username": "admin",
				"url":      "https://example.com",
			},
			want: map[string]any{
				"token":    "[REDACTED]",
				"username": "admin",
				"url":      "https://example.com",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeArguments(tc.input)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("length mismatch: expected %v, got %v", tc.want, got)
			}
			for k, wantVal := range tc.want {
				gotVal, ok := got[k]
				if !ok {
					t.Fatalf("missing key %q in result", k)
				}
				if gotVal != wantVal {
					t.Fatalf("key %q: expected %v, got %v", k, wantVal, gotVal)
				}
			}
		})
	}
}

// TestSanitizeArgumentsNoMutation verifies the original map is not mutated.
func TestSanitizeArgumentsNoMutation(t *testing.T) {
	original := map[string]any{"token": "secret", "username": "admin"}
	before := map[string]any{"token": "secret", "username": "admin"}

	sanitizeArguments(original)

	for k, v := range before {
		if original[k] != v {
			t.Fatalf("original map was mutated: key %q changed from %v to %v", k, v, original[k])
		}
	}
}

// TestNewAuditHooksNotNil verifies NewAuditHooks returns a non-nil instance.
func TestNewAuditHooksNotNil(t *testing.T) {
	hooks := NewAuditHooks(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	if hooks == nil {
		t.Fatal("expected non-nil hooks")
	}
}

// TestAuditHookBeforeCallTool verifies the tool_call event fields.
func TestAuditHookBeforeCallTool(t *testing.T) {
	var buf bytes.Buffer
	hooks := NewAuditHooks(newTestAuditLogger(&buf))

	req := &mcpgo.CallToolRequest{}
	req.Params.Name = "my_tool"
	req.Params.Arguments = map[string]any{"param": "value"}

	hooks.OnBeforeCallTool[0](context.Background(), "req-1", req)

	entry := decodeAuditLogEntry(t, &buf)
	if entry["event"] != "audit.tool_call" {
		t.Errorf("expected event=audit.tool_call, got %v", entry["event"])
	}
	if entry["tool"] != "my_tool" {
		t.Errorf("expected tool=my_tool, got %v", entry["tool"])
	}
	if entry["request_id"] != "req-1" {
		t.Errorf("expected request_id=req-1, got %v", entry["request_id"])
	}
}

// TestAuditHookAfterCallToolSuccess verifies is_error=false for a successful result.
func TestAuditHookAfterCallToolSuccess(t *testing.T) {
	var buf bytes.Buffer
	hooks := NewAuditHooks(newTestAuditLogger(&buf))

	req := &mcpgo.CallToolRequest{}
	req.Params.Name = "my_tool"

	hooks.OnAfterCallTool[0](context.Background(), "req-2", req, &mcpgo.CallToolResult{IsError: false})

	entry := decodeAuditLogEntry(t, &buf)
	if entry["event"] != "audit.tool_result" {
		t.Errorf("expected event=audit.tool_result, got %v", entry["event"])
	}
	if entry["tool"] != "my_tool" {
		t.Errorf("expected tool=my_tool, got %v", entry["tool"])
	}
	if entry["is_error"] != false {
		t.Errorf("expected is_error=false, got %v", entry["is_error"])
	}
}

// TestAuditHookAfterCallToolIsError verifies is_error=true when result has IsError set.
func TestAuditHookAfterCallToolIsError(t *testing.T) {
	var buf bytes.Buffer
	hooks := NewAuditHooks(newTestAuditLogger(&buf))

	req := &mcpgo.CallToolRequest{}
	req.Params.Name = "error_tool"

	hooks.OnAfterCallTool[0](context.Background(), "req-3", req, &mcpgo.CallToolResult{IsError: true})

	entry := decodeAuditLogEntry(t, &buf)
	if entry["is_error"] != true {
		t.Errorf("expected is_error=true, got %v", entry["is_error"])
	}
}

// TestAuditHookAfterCallToolDurationMs verifies duration_ms is a non-negative number.
func TestAuditHookAfterCallToolDurationMs(t *testing.T) {
	var buf bytes.Buffer
	hooks := NewAuditHooks(newTestAuditLogger(&buf))

	req := &mcpgo.CallToolRequest{}
	req.Params.Name = "timed_tool"

	// BeforeCallTool stores the start time; AfterCallTool computes the duration.
	hooks.OnBeforeCallTool[0](context.Background(), "req-4", req)
	buf.Reset()
	hooks.OnAfterCallTool[0](context.Background(), "req-4", req, &mcpgo.CallToolResult{})

	entry := decodeAuditLogEntry(t, &buf)
	durationMs, ok := entry["duration_ms"].(float64)
	if !ok {
		t.Fatalf("expected duration_ms to be a number, got %T: %v", entry["duration_ms"], entry["duration_ms"])
	}
	if durationMs < 0 {
		t.Errorf("expected duration_ms >= 0, got %v", durationMs)
	}
}

// TestAuditHookOnRegisterSession verifies session_start event.
func TestAuditHookOnRegisterSession(t *testing.T) {
	var buf bytes.Buffer
	hooks := NewAuditHooks(newTestAuditLogger(&buf))

	hooks.OnRegisterSession[0](context.Background(), &mockClientSession{id: "session-123"})

	entry := decodeAuditLogEntry(t, &buf)
	if entry["event"] != "audit.session_start" {
		t.Errorf("expected event=audit.session_start, got %v", entry["event"])
	}
	if entry["session_id"] != "session-123" {
		t.Errorf("expected session_id=session-123, got %v", entry["session_id"])
	}
}

// TestAuditHookOnUnregisterSession verifies session_end event.
func TestAuditHookOnUnregisterSession(t *testing.T) {
	var buf bytes.Buffer
	hooks := NewAuditHooks(newTestAuditLogger(&buf))

	hooks.OnUnregisterSession[0](context.Background(), &mockClientSession{id: "session-456"})

	entry := decodeAuditLogEntry(t, &buf)
	if entry["event"] != "audit.session_end" {
		t.Errorf("expected event=audit.session_end, got %v", entry["event"])
	}
	if entry["session_id"] != "session-456" {
		t.Errorf("expected session_id=session-456, got %v", entry["session_id"])
	}
}

// TestAuditHookOnError verifies audit.error event and error field.
func TestAuditHookOnError(t *testing.T) {
	var buf bytes.Buffer
	hooks := NewAuditHooks(newTestAuditLogger(&buf))

	hooks.OnError[0](context.Background(), "req-5", mcpgo.MCPMethod("tools/call"), nil, errors.New("something went wrong"))

	entry := decodeAuditLogEntry(t, &buf)
	if entry["event"] != "audit.error" {
		t.Errorf("expected event=audit.error, got %v", entry["event"])
	}
	if entry["error"] != "something went wrong" {
		t.Errorf("expected error=something went wrong, got %v", entry["error"])
	}
}

// TestAuditHookBeforeAny verifies audit.request event and method field.
func TestAuditHookBeforeAny(t *testing.T) {
	var buf bytes.Buffer
	hooks := NewAuditHooks(newTestAuditLogger(&buf))

	hooks.OnBeforeAny[0](context.Background(), "req-6", mcpgo.MCPMethod("tools/call"), nil)

	entry := decodeAuditLogEntry(t, &buf)
	if entry["event"] != "audit.request" {
		t.Errorf("expected event=audit.request, got %v", entry["event"])
	}
	if entry["method"] != "tools/call" {
		t.Errorf("expected method=tools/call, got %v", entry["method"])
	}
}

// TestAuditHookBeforeCallToolSanitizesArguments verifies sensitive args are redacted in the log.
func TestAuditHookBeforeCallToolSanitizesArguments(t *testing.T) {
	var buf bytes.Buffer
	hooks := NewAuditHooks(newTestAuditLogger(&buf))

	req := &mcpgo.CallToolRequest{}
	req.Params.Name = "auth_tool"
	req.Params.Arguments = map[string]any{"token": "supersecret", "username": "admin"}

	hooks.OnBeforeCallTool[0](context.Background(), "req-7", req)

	entry := decodeAuditLogEntry(t, &buf)
	arguments, ok := entry["arguments"].(map[string]any)
	if !ok {
		t.Fatalf("expected arguments to be a map, got %T: %v", entry["arguments"], entry["arguments"])
	}
	if arguments["token"] != "[REDACTED]" {
		t.Errorf("expected token=[REDACTED], got %v", arguments["token"])
	}
	if arguments["username"] != "admin" {
		t.Errorf("expected username=admin, got %v", arguments["username"])
	}
}
