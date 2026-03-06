package tools

import "github.com/mark3labs/mcp-go/mcp"

// WithCategory returns a ToolOption that sets the "categories" key in _meta.
// This metadata is read by the agent-backend to filter tools by query keywords.
// Each tool can belong to multiple categories (e.g. "send", "bitcoin").
func WithCategory(categories ...string) mcp.ToolOption {
	return func(tool *mcp.Tool) {
		if tool.Meta == nil {
			tool.Meta = &mcp.Meta{}
		}
		if tool.Meta.AdditionalFields == nil {
			tool.Meta.AdditionalFields = make(map[string]any)
		}
		tool.Meta.AdditionalFields["categories"] = categories
	}
}
