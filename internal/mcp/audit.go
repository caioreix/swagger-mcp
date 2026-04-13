package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpgoserver "github.com/mark3labs/mcp-go/server"
)

// NewAuditHooks returns a configured Hooks instance that emits structured audit
// log events for every significant MCP lifecycle event.
func NewAuditHooks(logger *slog.Logger) *mcpgoserver.Hooks {
	var toolStartTimes sync.Map

	hooks := &mcpgoserver.Hooks{}

	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcpgo.MCPMethod, message any) {
		logger.DebugContext(ctx, "audit",
			"event", "audit.request",
			"method", string(method),
			"request_id", fmt.Sprint(id),
		)
	})

	hooks.AddOnError(func(ctx context.Context, id any, method mcpgo.MCPMethod, message any, err error) {
		logger.ErrorContext(ctx, "audit",
			"event", "audit.error",
			"method", string(method),
			"request_id", fmt.Sprint(id),
			"error", err.Error(),
		)
	})

	hooks.AddOnSuccess(func(ctx context.Context, id any, method mcpgo.MCPMethod, message any, result any) {
		logger.DebugContext(ctx, "audit",
			"event", "audit.success",
			"method", string(method),
			"request_id", fmt.Sprint(id),
		)
	})

	hooks.AddOnRegisterSession(func(ctx context.Context, session mcpgoserver.ClientSession) {
		logger.InfoContext(ctx, "audit",
			"event", "audit.session_start",
			"session_id", session.SessionID(),
		)
	})

	hooks.AddOnUnregisterSession(func(ctx context.Context, session mcpgoserver.ClientSession) {
		logger.InfoContext(ctx, "audit",
			"event", "audit.session_end",
			"session_id", session.SessionID(),
		)
	})

	hooks.AddBeforeInitialize(func(ctx context.Context, id any, message *mcpgo.InitializeRequest) {
		logger.InfoContext(ctx, "audit",
			"event", "audit.client_connect",
			"request_id", fmt.Sprint(id),
			"client_name", message.Params.ClientInfo.Name,
			"client_version", message.Params.ClientInfo.Version,
		)
	})

	hooks.AddAfterInitialize(func(ctx context.Context, id any, message *mcpgo.InitializeRequest, result *mcpgo.InitializeResult) {
		logger.InfoContext(ctx, "audit",
			"event", "audit.client_connected",
			"request_id", fmt.Sprint(id),
			"server_name", result.ServerInfo.Name,
			"server_version", result.ServerInfo.Version,
			"protocol_version", result.ProtocolVersion,
		)
	})

	hooks.AddBeforeCallTool(func(ctx context.Context, id any, message *mcpgo.CallToolRequest) {
		toolStartTimes.Store(fmt.Sprint(id), time.Now())
		logger.InfoContext(ctx, "audit",
			"event", "audit.tool_call",
			"request_id", fmt.Sprint(id),
			"tool", message.Params.Name,
			"arguments", sanitizeArguments(message.GetArguments()),
		)
	})

	hooks.AddAfterCallTool(func(ctx context.Context, id any, message *mcpgo.CallToolRequest, result any) {
		reqID := fmt.Sprint(id)
		var durationMs int64
		if v, ok := toolStartTimes.LoadAndDelete(reqID); ok {
			durationMs = time.Since(v.(time.Time)).Milliseconds()
		}

		isError := false
		if r, ok := result.(*mcpgo.CallToolResult); ok {
			isError = r.IsError
		}

		logger.InfoContext(ctx, "audit",
			"event", "audit.tool_result",
			"request_id", reqID,
			"tool", message.Params.Name,
			"duration_ms", durationMs,
			"is_error", isError,
		)
	})

	hooks.AddBeforeListTools(func(ctx context.Context, id any, message *mcpgo.ListToolsRequest) {
		logger.InfoContext(ctx, "audit",
			"event", "audit.list_tools",
			"request_id", fmt.Sprint(id),
		)
	})

	return hooks
}

// sanitizeArguments returns a copy of args with sensitive values redacted.
func sanitizeArguments(args map[string]any) map[string]any {
	if args == nil {
		return nil
	}

	sensitiveKeys := []string{"password", "token", "secret", "key", "auth", "credential", "authorization"}

	result := make(map[string]any, len(args))
	for k, v := range args {
		lower := strings.ToLower(k)
		redact := false
		for _, sensitive := range sensitiveKeys {
			if strings.Contains(lower, sensitive) {
				redact = true
				break
			}
		}
		if redact {
			result[k] = "[REDACTED]"
		} else {
			result[k] = v
		}
	}
	return result
}
