package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewToolMiddleware returns a ToolHandlerMiddleware that logs every tool call
// with its arguments, duration, and outcome.
func NewToolMiddleware(logger *log.Logger) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			tool := req.Params.Name
			sessionID := sessionIDFromCtx(ctx)
			args := formatArgs(req.GetArguments())

			logger.Printf("[CALL]  tool=%-20s session=%-10s args=%s", tool, sessionID, args)

			start := time.Now()
			result, err := next(ctx, req)
			duration := time.Since(start)

			if err != nil {
				logger.Printf("[ERROR] tool=%-20s session=%-10s duration=%-12s error=%v", tool, sessionID, duration, err)
			} else if result != nil && result.IsError {
				errText := extractText(result)
				logger.Printf("[FAIL]  tool=%-20s session=%-10s duration=%-12s error=%s", tool, sessionID, duration, errText)
			} else {
				preview := extractText(result)
				logger.Printf("[OK]    tool=%-20s session=%-10s duration=%-12s result=%s", tool, sessionID, duration, preview)
			}

			return result, err
		}
	}
}

// NewHooks returns Hooks that log session lifecycle and connection events.
func NewHooks(logger *log.Logger) *server.Hooks {
	hooks := &server.Hooks{}

	hooks.OnRegisterSession = append(hooks.OnRegisterSession,
		func(ctx context.Context, session server.ClientSession) {
			logger.Printf("[SESSION]  registered  session=%s", session.SessionID())
		},
	)

	hooks.OnUnregisterSession = append(hooks.OnUnregisterSession,
		func(ctx context.Context, session server.ClientSession) {
			logger.Printf("[SESSION]  unregistered session=%s", session.SessionID())
		},
	)

	hooks.OnAfterInitialize = append(hooks.OnAfterInitialize,
		func(ctx context.Context, id any, message *mcp.InitializeRequest, result *mcp.InitializeResult) {
			clientName := message.Params.ClientInfo.Name
			clientVersion := message.Params.ClientInfo.Version
			logger.Printf("[INIT]     client=%s version=%s", clientName, clientVersion)
		},
	)

	hooks.OnError = append(hooks.OnError,
		func(ctx context.Context, id any, method mcp.MCPMethod, message any, err error) {
			logger.Printf("[RPC_ERR]  method=%-30s id=%v error=%v", method, id, err)
		},
	)

	return hooks
}

func sessionIDFromCtx(ctx context.Context) string {
	if sess := server.ClientSessionFromContext(ctx); sess != nil {
		return sess.SessionID()
	}
	return "default"
}

// formatArgs produces a compact JSON representation of tool arguments.
func formatArgs(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	b, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf("%v", args)
	}
	return string(b)
}

// extractText pulls the first text content from a CallToolResult, truncated.
func extractText(result *mcp.CallToolResult) string {
	if result == nil {
		return "<nil>"
	}
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			text := tc.Text
			// Collapse to single line for log readability.
			text = strings.ReplaceAll(text, "\n", " | ")
			if len(text) > 120 {
				text = text[:120] + "..."
			}
			return text
		}
	}
	return "<no text>"
}
