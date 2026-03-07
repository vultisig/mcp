package toolmeta

import "github.com/mark3labs/mcp-go/mcp"

// WithMeta sets custom metadata fields on a tool via _meta.
// Agent-backend reads these to drive generic tool behaviors
// (address injection, result interception, etc.).
func WithMeta(fields map[string]any) mcp.ToolOption {
	return func(t *mcp.Tool) {
		if t.Meta == nil {
			t.Meta = &mcp.Meta{}
		}
		if t.Meta.AdditionalFields == nil {
			t.Meta.AdditionalFields = make(map[string]any)
		}
		for k, v := range fields {
			t.Meta.AdditionalFields[k] = v
		}
	}
}
