package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/vultisig/mcp/internal/thorchain"
	"github.com/vultisig/mcp/internal/vault"
)

func setupBTCVault(t *testing.T) *vault.Store {
	t.Helper()
	store := vault.NewStore()
	store.Set("default", vault.Info{
		ECDSAPublicKey: "02f6a8148a62320e149cb15c544fe8a25ab483a0095d2280d03b8a00a7feada13d",
		ChainCode:      "27f8e61e8116e3cf83dbbfde96f23c3c9cd78e3d44695abdc5c6f2d58e92fc67",
	})
	return store
}

func deriveBTCAddress(t *testing.T, store *vault.Store) string {
	t.Helper()
	v, ok := store.Get("default")
	if !ok {
		t.Fatal("vault not found")
	}
	addr, _, _, err := address.GetAddress(v.ECDSAPublicKey, v.ChainCode, common.Bitcoin)
	if err != nil {
		t.Fatalf("derive BTC address: %v", err)
	}
	return addr
}

func mockThorchainServer(t *testing.T, feeRate uint64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addresses := []map[string]any{
			{
				"chain":          "BTC",
				"address":        "fakefake",
				"gas_rate":       strconv.FormatUint(feeRate, 10),
				"gas_rate_units": "satsperbyte",
				"halted":         false,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(addresses)
		if err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
}

func TestBTCFeeRate(t *testing.T) {
	srv := mockThorchainServer(t, 15)
	defer srv.Close()

	tcClient := thorchain.NewClient(srv.URL)
	handler := handleBTCFeeRate(tcClient)

	req := callToolReq("btc_fee_rate", map[string]any{})
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %v", res.Content)
	}

	var resp map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &resp)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["fee_rate"] == nil {
		t.Error("expected fee_rate in response")
	}
}

func TestBTCFeeRate_HaltedChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"current_chain_heights": map[string]any{},
			"chains": []map[string]any{
				{
					"chain":    "BTC",
					"gas_rate": uint64(15),
					"halted":   true,
					"outbound": false,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	tcClient := thorchain.NewClient(srv.URL)
	handler := handleBTCFeeRate(tcClient)

	req := callToolReq("btc_fee_rate", map[string]any{})
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for halted chain")
	}
}

func TestBuildBTCSend_Basic(t *testing.T) {
	store := setupBTCVault(t)
	senderAddr := deriveBTCAddress(t, store)

	handler := handleBuildBTCSend(store)

	req := callToolReq("build_btc_send", map[string]any{
		"to_address": "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		"amount":     "50000",
		"fee_rate":   float64(10),
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %v", res.Content)
	}

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &result)
	if err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result["chain"] != "Bitcoin" {
		t.Errorf("expected chain Bitcoin, got %v", result["chain"])
	}
	if result["from"] != senderAddr {
		t.Errorf("expected from %s, got %v", senderAddr, result["from"])
	}
	if result["to"] != "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4" {
		t.Errorf("unexpected to: %v", result["to"])
	}
	if result["action"] != "transfer" {
		t.Errorf("expected action transfer, got %v", result["action"])
	}
	if result["fee_rate"] == nil {
		t.Error("expected fee_rate in result")
	}
}

func TestBuildBTCSend_WithMemo(t *testing.T) {
	store := setupBTCVault(t)

	handler := handleBuildBTCSend(store)

	req := callToolReq("build_btc_send", map[string]any{
		"to_address": "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		"amount":     "50000",
		"fee_rate":   float64(10),
		"memo":       "SWAP:ETH.ETH:0x1234",
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %v", res.Content)
	}

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcp.TextContent).Text), &result)
	if err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result["action"] != "swap" {
		t.Errorf("expected action swap, got %v", result["action"])
	}
	if result["memo"] != "SWAP:ETH.ETH:0x1234" {
		t.Errorf("unexpected memo: %v", result["memo"])
	}
}

func TestBuildBTCSend_MemoTooLong(t *testing.T) {
	store := setupBTCVault(t)

	handler := handleBuildBTCSend(store)

	memo := ""
	for i := 0; i < 81; i++ {
		memo += "x"
	}

	req := callToolReq("build_btc_send", map[string]any{
		"to_address": "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		"amount":     "50000",
		"fee_rate":   float64(10),
		"memo":       memo,
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error for memo too long")
	}
}

func TestBuildBTCSend_InvalidAddress(t *testing.T) {
	store := setupBTCVault(t)

	handler := handleBuildBTCSend(store)

	req := callToolReq("build_btc_send", map[string]any{
		"to_address": "not-a-valid-btc-address",
		"amount":     "50000",
		"fee_rate":   float64(10),
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error for invalid address")
	}
}

func TestBuildBTCSend_NoVault(t *testing.T) {
	store := vault.NewStore()

	handler := handleBuildBTCSend(store)

	req := callToolReq("build_btc_send", map[string]any{
		"to_address": "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		"amount":     "50000",
		"fee_rate":   float64(10),
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error when vault not set")
	}
}
