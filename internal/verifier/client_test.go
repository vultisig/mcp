package verifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHasAPIKey(t *testing.T) {
	c := NewClient("http://example.com", "")
	if c.HasAPIKey() {
		t.Error("expected HasAPIKey=false for empty key")
	}

	c2 := NewClient("http://example.com", "secret-key")
	if !c2.HasAPIKey() {
		t.Error("expected HasAPIKey=true for non-empty key")
	}
}

func TestGetFeeStatus_InvalidPublicKey(t *testing.T) {
	c := NewClient("http://example.com", "key")

	cases := []string{
		"",
		"notHex",
		"038e9b3ae4e94e9b9a0b561d23a11b8f794bd45a6f7f65a2293a0283004f9937d",   // 65 chars
		"038e9b3ae4e94e9b9a0b561d23a11b8f794bd45a6f7f65a2293a0283004f9937d22", // 67 chars
	}

	for _, key := range cases {
		_, err := c.GetFeeStatus(context.Background(), key)
		if err == nil {
			t.Errorf("expected error for public key %q", key)
		}
	}
}

func TestIsPluginInstalled_InvalidPublicKey(t *testing.T) {
	c := NewClient("http://example.com", "key")

	_, err := c.IsPluginInstalled(context.Background(), "bad-key", "some-plugin")
	if err == nil {
		t.Error("expected error for invalid public key")
	}
}

func TestGetFeeStatus_TrialActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/service/fee/status" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Service-Key") == "" {
			http.Error(w, "missing key", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(FeeStatus{
			IsTrialActive:  true,
			TrialRemaining: 5,
		})
		if err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	status, err := c.GetFeeStatus(context.Background(), "038e9b3ae4e94e9b9a0b561d23a11b8f794bd45a6f7f65a2293a0283004f9937d2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.IsTrialActive {
		t.Error("expected is_trial_active=true")
	}
	if status.TrialRemaining != 5 {
		t.Errorf("expected trial_remaining=5, got %d", status.TrialRemaining)
	}
}

func TestIsPluginInstalled_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/service/plugins/installed" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := installedPluginsResponse{
			Code: 200,
		}
		resp.Data.Plugins = []struct {
			ID string `json:"id"`
		}{
			{ID: "vultisig-dca"},
			{ID: "other-plugin"},
		}
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	installed, err := c.IsPluginInstalled(context.Background(), "038e9b3ae4e94e9b9a0b561d23a11b8f794bd45a6f7f65a2293a0283004f9937d2", "vultisig-dca")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !installed {
		t.Error("expected plugin to be installed")
	}
}

func TestIsPluginInstalled_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := installedPluginsResponse{Code: 200}
		resp.Data.Plugins = []struct {
			ID string `json:"id"`
		}{}
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	installed, err := c.IsPluginInstalled(context.Background(), "038e9b3ae4e94e9b9a0b561d23a11b8f794bd45a6f7f65a2293a0283004f9937d2", "missing-plugin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if installed {
		t.Error("expected plugin to not be installed")
	}
}
