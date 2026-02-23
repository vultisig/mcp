package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestRegisterMCPResources(t *testing.T) {
	// Verify embedded files were loaded
	if len(fileContents) == 0 {
		t.Fatal("fileContents is empty — embed did not load any skill files")
	}
	t.Logf("fileContents has %d entries", len(fileContents))
	for name := range fileContents {
		t.Logf("  embedded file: %s", name)
	}

	s := server.NewMCPServer("test", "0.1.0",
		server.WithResourceCapabilities(false, true),
	)
	RegisterMCPResources(s)

	// Simulate resources/list via HandleMessage
	resp := s.HandleMessage(
		context.Background(),
		json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`),
	)

	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Printf("resources/list response:\n%s\n", string(out))

	// Check that we got a valid response with resources
	raw, _ := json.Marshal(resp)
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	resultRaw, ok := parsed["result"]
	if !ok {
		t.Fatalf("no 'result' in response: %s", string(raw))
	}

	var result struct {
		Resources []mcp.Resource `json:"resources"`
	}
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if len(result.Resources) == 0 {
		t.Fatalf("expected resources, got 0")
	}

	t.Logf("got %d resources:", len(result.Resources))
	for _, r := range result.Resources {
		t.Logf("  uri=%s name=%s", r.URI, r.Name)
	}
}
