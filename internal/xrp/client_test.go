package xrp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL), srv
}

func rpcHandler(t *testing.T, result any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{"result": result}
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}
}

func rpcErrorHandler(t *testing.T, errCode, errMsg string) http.HandlerFunc {
	t.Helper()
	return rpcHandler(t, map[string]any{
		"error":         errCode,
		"error_message": errMsg,
	})
}

func TestGetAccountInfo(t *testing.T) {
	client, _ := newTestServer(t, rpcHandler(t, map[string]any{
		"account_data": map[string]any{
			"Account":  "rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN",
			"Balance":  "123456789",
			"Sequence": 42,
		},
	}))

	info, err := client.GetAccountInfo(context.Background(), "rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN")
	if err != nil {
		t.Fatalf("GetAccountInfo: %v", err)
	}
	if info.Sequence != 42 {
		t.Errorf("sequence = %d, want 42", info.Sequence)
	}
	if info.Balance != "123456789" {
		t.Errorf("balance = %q, want 123456789", info.Balance)
	}
}

func TestGetAccountInfo_StringSequence(t *testing.T) {
	client, _ := newTestServer(t, rpcHandler(t, map[string]any{
		"account_data": map[string]any{
			"Account":  "rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN",
			"Balance":  "100",
			"Sequence": "99",
		},
	}))

	info, err := client.GetAccountInfo(context.Background(), "rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN")
	if err != nil {
		t.Fatalf("GetAccountInfo: %v", err)
	}
	if info.Sequence != 99 {
		t.Errorf("sequence = %d, want 99", info.Sequence)
	}
}

func TestGetAccountInfo_RPCError(t *testing.T) {
	client, _ := newTestServer(t, rpcErrorHandler(t, "actNotFound", "Account not found."))

	_, err := client.GetAccountInfo(context.Background(), "rInvalid")
	if err == nil {
		t.Fatal("expected error for unknown account")
	}
}

func TestGetCurrentLedger(t *testing.T) {
	client, _ := newTestServer(t, rpcHandler(t, map[string]any{
		"ledger_index": 75801736,
	}))

	idx, err := client.GetCurrentLedger(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentLedger: %v", err)
	}
	if idx != 75801736 {
		t.Errorf("ledger index = %d, want 75801736", idx)
	}
}

func TestGetCurrentLedger_StringIndex(t *testing.T) {
	client, _ := newTestServer(t, rpcHandler(t, map[string]any{
		"ledger_index": "12345678",
	}))

	idx, err := client.GetCurrentLedger(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentLedger: %v", err)
	}
	if idx != 12345678 {
		t.Errorf("ledger index = %d, want 12345678", idx)
	}
}

func TestGetBaseFee(t *testing.T) {
	client, _ := newTestServer(t, rpcHandler(t, map[string]any{
		"drops": map[string]any{
			"base_fee": "12",
		},
	}))

	fee, err := client.GetBaseFee(context.Background())
	if err != nil {
		t.Fatalf("GetBaseFee: %v", err)
	}
	if fee != 12 {
		t.Errorf("fee = %d, want 12", fee)
	}
}

func TestGetBaseFee_HighFee(t *testing.T) {
	client, _ := newTestServer(t, rpcHandler(t, map[string]any{
		"drops": map[string]any{
			"base_fee": "5000",
		},
	}))

	fee, err := client.GetBaseFee(context.Background())
	if err != nil {
		t.Fatalf("GetBaseFee: %v", err)
	}
	if fee != 5000 {
		t.Errorf("fee = %d, want 5000", fee)
	}
}

func TestGetBaseFee_MinimumEnforced(t *testing.T) {
	client, _ := newTestServer(t, rpcHandler(t, map[string]any{
		"drops": map[string]any{
			"base_fee": "5",
		},
	}))

	fee, err := client.GetBaseFee(context.Background())
	if err != nil {
		t.Fatalf("GetBaseFee: %v", err)
	}
	if fee != 12 {
		t.Errorf("fee = %d, want 12 (minimum)", fee)
	}
}

func TestGetBaseFee_MissingField(t *testing.T) {
	client, _ := newTestServer(t, rpcHandler(t, map[string]any{
		"drops": map[string]any{},
	}))

	_, err := client.GetBaseFee(context.Background())
	if err == nil {
		t.Fatal("expected error for missing base_fee")
	}
}

func TestGetBaseFee_InvalidString(t *testing.T) {
	client, _ := newTestServer(t, rpcHandler(t, map[string]any{
		"drops": map[string]any{
			"base_fee": "not-a-number",
		},
	}))

	_, err := client.GetBaseFee(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid base_fee")
	}
}

func TestDo_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL)

	_, err := client.GetBaseFee(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}
