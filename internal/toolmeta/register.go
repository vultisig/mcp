package toolmeta

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Register adds a tool to the MCP server with required categories.
// The first category is a non-variadic parameter so the compiler
// rejects calls that omit categories entirely.
func Register(s *server.MCPServer, tool mcp.Tool, handler server.ToolHandlerFunc, firstCategory string, more ...string) {
	categories := append([]string{firstCategory}, more...)
	if tool.Meta == nil {
		tool.Meta = &mcp.Meta{}
	}
	if tool.Meta.AdditionalFields == nil {
		tool.Meta.AdditionalFields = make(map[string]any)
	}
	tool.Meta.AdditionalFields["categories"] = categories
	s.AddTool(tool, handler)
}
