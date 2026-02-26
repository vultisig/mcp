package tools

import (
	"context"
	"encoding/json"
	"testing"

	evmclient "github.com/vultisig/mcp/internal/evm"
	"github.com/vultisig/mcp/internal/vault"
)

// ---------------------------------------------------------------------------
// TestBuildEVMTx_ChainParam verifies that the chain parameter correctly
// sets the chain ID in the output transaction for all supported chains.
// ---------------------------------------------------------------------------

func TestBuildEVMTx_ChainParam(t *testing.T) {
	handler := handleBuildEVMTx()
	ctx := context.Background()

	tests := []struct {
		chain       string
		wantChainID string
	}{
		{"Ethereum", "1"},
		{"BSC", "56"},
		{"Polygon", "137"},
		{"Avalanche", "43114"},
		{"Arbitrum", "42161"},
		{"Optimism", "10"},
		{"Base", "8453"},
		{"Blast", "81457"},
		{"Mantle", "5000"},
		{"Zksync", "324"},
	}

	for _, tt := range tests {
		t.Run(tt.chain, func(t *testing.T) {
			req := callToolReq("build_evm_tx", map[string]any{
				"chain":                    tt.chain,
				"to":                       "0x0000000000000000000000000000000000000001",
				"value":                    "0",
				"nonce":                    "1",
				"gas_limit":                "21000",
				"max_fee_per_gas":          "1000000000",
				"max_priority_fee_per_gas": "1000000",
			})
			res, err := handler(ctx, req)
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}

			text := resultText(t, res)

			var txResult struct {
				Transactions []struct {
					ChainID string `json:"chain_id"`
				} `json:"transactions"`
			}
			if err := json.Unmarshal([]byte(text), &txResult); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(txResult.Transactions) != 1 {
				t.Fatalf("expected 1 transaction, got %d", len(txResult.Transactions))
			}
			if txResult.Transactions[0].ChainID != tt.wantChainID {
				t.Errorf("chain_id: got %q, want %q", txResult.Transactions[0].ChainID, tt.wantChainID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBuildEVMTx_ChainIDOverride verifies that an explicit chain_id param
// overrides the chain name lookup.
// ---------------------------------------------------------------------------

func TestBuildEVMTx_ChainIDOverride(t *testing.T) {
	handler := handleBuildEVMTx()
	ctx := context.Background()

	req := callToolReq("build_evm_tx", map[string]any{
		"chain":                    "BSC",
		"to":                       "0x0000000000000000000000000000000000000001",
		"value":                    "0",
		"nonce":                    "1",
		"gas_limit":                "21000",
		"max_fee_per_gas":          "1000000000",
		"max_priority_fee_per_gas": "1000000",
		"chain_id":                 "999",
	})
	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	text := resultText(t, res)

	var txResult struct {
		Transactions []struct {
			ChainID string `json:"chain_id"`
		} `json:"transactions"`
	}
	if err := json.Unmarshal([]byte(text), &txResult); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(txResult.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txResult.Transactions))
	}
	if txResult.Transactions[0].ChainID != "999" {
		t.Errorf("chain_id: got %q, want %q", txResult.Transactions[0].ChainID, "999")
	}
}

// ---------------------------------------------------------------------------
// TestBuildEVMTx_DefaultChainIsEthereum verifies that omitting the chain
// parameter results in Ethereum (chain_id=1).
// ---------------------------------------------------------------------------

func TestBuildEVMTx_DefaultChainIsEthereum(t *testing.T) {
	handler := handleBuildEVMTx()
	ctx := context.Background()

	req := callToolReq("build_evm_tx", map[string]any{
		"to":                       "0x0000000000000000000000000000000000000001",
		"value":                    "0",
		"nonce":                    "1",
		"gas_limit":                "21000",
		"max_fee_per_gas":          "1000000000",
		"max_priority_fee_per_gas": "1000000",
	})
	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	text := resultText(t, res)

	var txResult struct {
		Transactions []struct {
			ChainID string `json:"chain_id"`
		} `json:"transactions"`
	}
	if err := json.Unmarshal([]byte(text), &txResult); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(txResult.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txResult.Transactions))
	}
	if txResult.Transactions[0].ChainID != "1" {
		t.Errorf("chain_id: got %q, want %q", txResult.Transactions[0].ChainID, "1")
	}
}

// ---------------------------------------------------------------------------
// Pool error path tests — verify that all pool-dependent tools return a
// tool error (not a Go error) when the requested chain has no RPC URL.
// ---------------------------------------------------------------------------

func TestEVMGetBalance_UnknownChain(t *testing.T) {
	pool := evmclient.NewPool(map[string]string{})
	store := vault.NewStore()
	handler := handleEVMGetBalance(store, pool)
	ctx := context.Background()

	req := callToolReq("evm_get_balance", map[string]any{
		"chain":   "Ethereum",
		"address": "0x0000000000000000000000000000000000000001",
	})
	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("expected nil Go error, got: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for chain with no URL, got success")
	}
}

func TestEVMGetTokenBalance_UnknownChain(t *testing.T) {
	pool := evmclient.NewPool(map[string]string{})
	store := vault.NewStore()
	handler := handleEVMGetTokenBalance(store, pool)
	ctx := context.Background()

	req := callToolReq("evm_get_token_balance", map[string]any{
		"chain":            "Ethereum",
		"contract_address": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
		"address":          "0x0000000000000000000000000000000000000001",
	})
	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("expected nil Go error, got: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for chain with no URL, got success")
	}
}

func TestEVMCheckAllowance_UnknownChain(t *testing.T) {
	pool := evmclient.NewPool(map[string]string{})
	store := vault.NewStore()
	handler := handleEVMCheckAllowance(store, pool)
	ctx := context.Background()

	req := callToolReq("evm_check_allowance", map[string]any{
		"chain":            "Ethereum",
		"contract_address": "0xdAC17F958D2ee523a2206206994597C13D831ec7",
		"owner":            "0x0000000000000000000000000000000000000001",
		"spender":          "0x0000000000000000000000000000000000000002",
	})
	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("expected nil Go error, got: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for chain with no URL, got success")
	}
}

func TestEVMTxInfo_UnknownChain(t *testing.T) {
	pool := evmclient.NewPool(map[string]string{})
	store := vault.NewStore()
	handler := handleEVMTxInfo(store, pool)
	ctx := context.Background()

	req := callToolReq("evm_tx_info", map[string]any{
		"chain":   "Ethereum",
		"address": "0x0000000000000000000000000000000000000001",
	})
	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("expected nil Go error, got: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for chain with no URL, got success")
	}
}

func TestEVMCall_UnknownChain(t *testing.T) {
	pool := evmclient.NewPool(map[string]string{})
	handler := handleEVMCall(pool)
	ctx := context.Background()

	req := callToolReq("evm_call", map[string]any{
		"chain": "Ethereum",
		"to":    "0x0000000000000000000000000000000000000001",
		"data":  "0x00",
	})
	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("expected nil Go error, got: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected tool error for chain with no URL, got success")
	}
}
