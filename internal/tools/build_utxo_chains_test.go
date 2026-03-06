package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/vultisig/mcp/internal/mayachain"
	"github.com/vultisig/mcp/internal/thorchain"
	"github.com/vultisig/mcp/internal/vault"
)

func mockThorchainMulti(t *testing.T, gasRates map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var addresses []map[string]any
		for chain, rate := range gasRates {
			addresses = append(addresses, map[string]any{
				"chain": chain, "address": "fakefake", "gas_rate": rate, "gas_rate_units": "satsperbyte", "halted": false,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(addresses)
		if err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
}

func mockMayachainServer(t *testing.T, gasRates map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var addresses []map[string]any
		for chain, rate := range gasRates {
			addresses = append(addresses, map[string]any{
				"chain": chain, "address": "fakefake", "gas_rate": rate, "gas_rate_units": "satsperbyte", "halted": false,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(addresses)
		if err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
}

func deriveChainAddr(t *testing.T, chain common.Chain) string {
	t.Helper()
	addr, _, _, err := address.GetAddress(testECDSAPubKey, testChainCode, chain)
	if err != nil {
		t.Fatalf("derive address for %v: %v", chain, err)
	}
	return addr
}

func setupVaultForChain(t *testing.T) *vault.Store {
	t.Helper()
	store := vault.NewStore()
	store.Set("default", vault.Info{
		ECDSAPublicKey: testECDSAPubKey,
		ChainCode:      testChainCode,
	})
	return store
}

func TestLTCFeeRate(t *testing.T) {
	srv := mockThorchainMulti(t, map[string]string{"LTC": "12"})
	defer srv.Close()

	handler := handleLTCFeeRate(thorchain.NewClient(srv.URL))
	res, err := handler(context.Background(), callToolReq("ltc_fee_rate", map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(t, res)
	var result feeRateResult
	err = json.Unmarshal([]byte(text), &result)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Chain != "Litecoin" {
		t.Errorf("chain: got %s, want Litecoin", result.Chain)
	}
	if result.Ticker != "LTC" {
		t.Errorf("ticker: got %s, want LTC", result.Ticker)
	}
	if result.FeeRate != 12 {
		t.Errorf("fee_rate: got %d, want 12", result.FeeRate)
	}
	if result.FeeRateUnit != "sat/vB" {
		t.Errorf("fee_rate_unit: got %s, want sat/vB", result.FeeRateUnit)
	}
}

func TestDOGEFeeRate(t *testing.T) {
	srv := mockThorchainMulti(t, map[string]string{"DOGE": "250000000"})
	defer srv.Close()

	handler := handleDOGEFeeRate(thorchain.NewClient(srv.URL))
	res, err := handler(context.Background(), callToolReq("doge_fee_rate", map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(t, res)
	var result feeRateResult
	err = json.Unmarshal([]byte(text), &result)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Chain != "Dogecoin" {
		t.Errorf("chain: got %s, want Dogecoin", result.Chain)
	}
	if result.Ticker != "DOGE" {
		t.Errorf("ticker: got %s, want DOGE", result.Ticker)
	}
	if result.FeeRate != 250000000 {
		t.Errorf("fee_rate: got %d, want 250000000", result.FeeRate)
	}
}

func TestBCHFeeRate(t *testing.T) {
	srv := mockThorchainMulti(t, map[string]string{"BCH": "3"})
	defer srv.Close()

	handler := handleBCHFeeRate(thorchain.NewClient(srv.URL))
	res, err := handler(context.Background(), callToolReq("bch_fee_rate", map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(t, res)
	var result feeRateResult
	err = json.Unmarshal([]byte(text), &result)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Chain != "Bitcoin-Cash" {
		t.Errorf("chain: got %s, want Bitcoin-Cash", result.Chain)
	}
	if result.Ticker != "BCH" {
		t.Errorf("ticker: got %s, want BCH", result.Ticker)
	}
	if result.FeeRate != 3 {
		t.Errorf("fee_rate: got %d, want 3", result.FeeRate)
	}
}

func TestDASHFeeRate(t *testing.T) {
	srv := mockMayachainServer(t, map[string]string{"DASH": "5"})
	defer srv.Close()

	handler := handleDASHFeeRate(mayachain.NewClient(srv.URL))
	res, err := handler(context.Background(), callToolReq("dash_fee_rate", map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(t, res)
	var result feeRateResult
	err = json.Unmarshal([]byte(text), &result)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Chain != "Dash" {
		t.Errorf("chain: got %s, want Dash", result.Chain)
	}
	if result.Ticker != "DASH" {
		t.Errorf("ticker: got %s, want DASH", result.Ticker)
	}
	if result.FeeRate != 5 {
		t.Errorf("fee_rate: got %d, want 5", result.FeeRate)
	}
}

func TestMayaFeeRate(t *testing.T) {
	srv := mockMayachainServer(t, map[string]string{"ZEC": "8", "DASH": "5"})
	defer srv.Close()

	handler := handleMayaFeeRate(mayachain.NewClient(srv.URL))
	res, err := handler(context.Background(), callToolReq("maya_fee_rate", map[string]any{"chain": "ZEC"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(t, res)
	var result feeRateResult
	err = json.Unmarshal([]byte(text), &result)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Chain != "ZEC" {
		t.Errorf("chain: got %s, want ZEC", result.Chain)
	}
	if result.FeeRate != 8 {
		t.Errorf("fee_rate: got %d, want 8", result.FeeRate)
	}
	if result.FeeRateUnit != "sat/vB" {
		t.Errorf("fee_rate_unit: got %s, want sat/vB", result.FeeRateUnit)
	}
}

func TestBuildLTCSend_Basic(t *testing.T) {
	store := setupVaultForChain(t)
	senderAddr := deriveChainAddr(t, common.Litecoin)

	handler := handleBuildLTCSend(store)

	req := callToolReq("build_ltc_send", map[string]any{
		"to_address": "LaMT348PWRnrqeeWArpwQPbuanWJByHxvT",
		"amount":     "100000",
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

	if result["chain"] != "Litecoin" {
		t.Errorf("expected chain Litecoin, got %v", result["chain"])
	}
	if result["from"] != senderAddr {
		t.Errorf("expected from %s, got %v", senderAddr, result["from"])
	}
	if result["action"] != "transfer" {
		t.Errorf("expected action transfer, got %v", result["action"])
	}
}

func TestBuildDOGESend_Basic(t *testing.T) {
	store := setupVaultForChain(t)
	senderAddr := deriveChainAddr(t, common.Dogecoin)

	handler := handleBuildDOGESend(store)

	req := callToolReq("build_doge_send", map[string]any{
		"to_address": "DDogepartyxxxxxxxxxxxxxxxxxxw1dfzr",
		"amount":     "100000000",
		"fee_rate":   float64(500000),
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

	if result["chain"] != "Dogecoin" {
		t.Errorf("expected chain Dogecoin, got %v", result["chain"])
	}
	if result["from"] != senderAddr {
		t.Errorf("expected from %s, got %v", senderAddr, result["from"])
	}
}

func TestBuildBCHSend_Basic(t *testing.T) {
	store := setupVaultForChain(t)
	senderAddr := deriveChainAddr(t, common.BitcoinCash)

	handler := handleBuildBCHSend(store)

	req := callToolReq("build_bch_send", map[string]any{
		"to_address": "bitcoincash:qp3wjpa3tjlj042z2wv7hahsldgwhwy0rq9sywjpyy",
		"amount":     "50000",
		"fee_rate":   float64(3),
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

	if result["chain"] != "Bitcoin-Cash" {
		t.Errorf("expected chain Bitcoin-Cash, got %v", result["chain"])
	}
	if result["from"] != senderAddr {
		t.Errorf("expected from %s, got %v", senderAddr, result["from"])
	}
}

func TestBuildDASHSend_Basic(t *testing.T) {
	store := setupVaultForChain(t)
	senderAddr := deriveChainAddr(t, common.Dash)

	handler := handleBuildDASHSend(store)

	req := callToolReq("build_dash_send", map[string]any{
		"to_address": "XqHiz8VqVZBmRFzYDZhZkLzuWEHvBa1Gy",
		"amount":     "100000000",
		"fee_rate":   float64(5),
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

	if result["chain"] != "Dash" {
		t.Errorf("expected chain Dash, got %v", result["chain"])
	}
	if result["from"] != senderAddr {
		t.Errorf("expected from %s, got %v", senderAddr, result["from"])
	}
}

func TestBuildZECSend_Basic(t *testing.T) {
	store := setupVaultForChain(t)
	senderAddr := deriveChainAddr(t, common.Zcash)

	handler := handleBuildZECSend(store)

	req := callToolReq("build_zec_send", map[string]any{
		"to_address": "t1VpYecAViRqAc73MAqnpDkMGt29EFAgm68",
		"amount":     "100000000",
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

	if result["chain"] != "Zcash" {
		t.Errorf("expected chain Zcash, got %v", result["chain"])
	}
	if result["from"] != senderAddr {
		t.Errorf("expected from %s, got %v", senderAddr, result["from"])
	}
	if result["action"] != "transfer" {
		t.Errorf("expected action transfer, got %v", result["action"])
	}
}

func TestBuildZECSend_WithMemo(t *testing.T) {
	store := setupVaultForChain(t)

	handler := handleBuildZECSend(store)

	req := callToolReq("build_zec_send", map[string]any{
		"to_address": "t1VpYecAViRqAc73MAqnpDkMGt29EFAgm68",
		"amount":     "100000000",
		"memo":       "SWAP:ETH.ETH:0xabc",
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
	if result["memo"] != "SWAP:ETH.ETH:0xabc" {
		t.Errorf("unexpected memo: %v", result["memo"])
	}
}
