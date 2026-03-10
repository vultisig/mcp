package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/vultisig/mcp/internal/vault"
	"github.com/vultisig/mcp/internal/verifier"
)

const testPluginPubKey = "038e9b3ae4e94e9b9a0b561d23a11b8f794bd45a6f7f65a2293a0283004f9937d2"

func setupPluginVault(t *testing.T) *vault.Store {
	t.Helper()
	store := vault.NewStore()
	store.Set("default", vault.Info{
		ECDSAPublicKey: testPluginPubKey,
		ChainCode:      testChainCode,
	})
	return store
}

func TestCheckPluginInstalled_NoVault(t *testing.T) {
	store := vault.NewStore()
	vc := verifier.NewClient("http://example.com", "key")
	handler := handleCheckPluginInstalled(store, vc)

	req := callToolReq("check_plugin_installed", map[string]any{
		"plugin_id": "vultisig-dca",
	})
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error when no vault is set")
	}
}

func TestCheckPluginInstalled_Installed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"code": 200,
			"data": map[string]any{
				"plugins": []map[string]any{
					{"id": "vultisig-dca"},
				},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	store := setupPluginVault(t)
	vc := verifier.NewClient(srv.URL, "test-key")
	handler := handleCheckPluginInstalled(store, vc)

	req := callToolReq("check_plugin_installed", map[string]any{
		"plugin_id": "vultisig-dca",
	})
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok || res.IsError {
		t.Fatalf("unexpected tool error: %v", res.Content)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["installed"] != true {
		t.Errorf("expected installed=true, got %v", result["installed"])
	}
}

func TestCheckBillingStatus_NoVault(t *testing.T) {
	store := vault.NewStore()
	vc := verifier.NewClient("http://example.com", "key")
	handler := handleCheckBillingStatus(store, vc)

	req := callToolReq("check_billing_status", map[string]any{})
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error when no vault is set")
	}
}

func TestCheckBillingStatus_TrialActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(map[string]any{
			"is_trial_active": true,
			"trial_remaining": 7,
			"unpaid_amount":   0,
		})
		if err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	store := setupPluginVault(t)
	vc := verifier.NewClient(srv.URL, "test-key")
	handler := handleCheckBillingStatus(store, vc)

	req := callToolReq("check_billing_status", map[string]any{})
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok || res.IsError {
		t.Fatalf("unexpected tool error: %v", res.Content)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result["billing_ok"] != true {
		t.Errorf("expected billing_ok=true, got %v", result["billing_ok"])
	}
	if result["is_trial_active"] != true {
		t.Errorf("expected is_trial_active=true, got %v", result["is_trial_active"])
	}
}
