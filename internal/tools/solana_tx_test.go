package tools

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gagliardetto/solana-go"

	solanaclient "github.com/vultisig/mcp/internal/solana"
	"github.com/vultisig/mcp/internal/types"
	"github.com/vultisig/mcp/internal/vault"
)

func TestBuildSolanaTx_InvalidAmount(t *testing.T) {
	store := vault.NewStore()
	handler := handleBuildSolanaTx(store, solanaclient.NewClient("https://localhost:0"))
	ctx := context.Background()

	tests := []struct {
		name   string
		amount string
	}{
		{"not_a_number", "abc"},
		{"negative", "-100"},
		{"zero", "0"},
		{"overflow", "99999999999999999999999999999999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := callToolReq("build_solana_tx", map[string]any{
				"from":   "11111111111111111111111111111111",
				"to":     "11111111111111111111111111111111",
				"amount": tt.amount,
			})
			res, err := handler(ctx, req)
			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			if !res.IsError {
				t.Fatal("expected tool error for invalid amount")
			}
		})
	}
}

func TestBuildSolanaTx_InvalidAddress(t *testing.T) {
	store := vault.NewStore()
	handler := handleBuildSolanaTx(store, solanaclient.NewClient("https://localhost:0"))
	ctx := context.Background()

	tests := []struct {
		name string
		from string
		to   string
	}{
		{"invalid_from", "not-valid!!!", "11111111111111111111111111111111"},
		{"invalid_to", "11111111111111111111111111111111", "not-valid!!!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := callToolReq("build_solana_tx", map[string]any{
				"from":   tt.from,
				"to":     tt.to,
				"amount": "1000000",
			})
			res, err := handler(ctx, req)
			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			if !res.IsError {
				t.Fatal("expected tool error for invalid address")
			}
		})
	}
}

func TestBuildSolanaTx_MissingParams(t *testing.T) {
	store := vault.NewStore()
	handler := handleBuildSolanaTx(store, solanaclient.NewClient("https://localhost:0"))
	ctx := context.Background()

	tests := []struct {
		name string
		args map[string]any
	}{
		{"missing_to", map[string]any{"from": "11111111111111111111111111111111", "amount": "1000"}},
		{"missing_amount", map[string]any{"from": "11111111111111111111111111111111", "to": "11111111111111111111111111111111"}},
		{"missing_from_no_vault", map[string]any{"to": "11111111111111111111111111111111", "amount": "1000"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := callToolReq("build_solana_tx", tt.args)
			res, err := handler(ctx, req)
			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			if !res.IsError {
				t.Fatal("expected tool error for missing params")
			}
		})
	}
}

func TestBuildSPLTransferTx_InvalidDecimals(t *testing.T) {
	store := vault.NewStore()
	handler := handleBuildSPLTransferTx(store, solanaclient.NewClient("https://localhost:0"))
	ctx := context.Background()

	tests := []struct {
		name     string
		decimals any
	}{
		{"fractional", 6.5},
		{"negative", -1.0},
		{"too_large", 256.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := callToolReq("build_spl_transfer_tx", map[string]any{
				"from":     "11111111111111111111111111111111",
				"to":       "11111111111111111111111111111111",
				"mint":     "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"amount":   "1000000",
				"decimals": tt.decimals,
			})
			res, err := handler(ctx, req)
			if err != nil {
				t.Fatalf("unexpected Go error: %v", err)
			}
			if !res.IsError {
				t.Fatal("expected tool error for invalid decimals")
			}
		})
	}
}

func TestBuildSolanaTx_VaultDerived(t *testing.T) {
	store := vault.NewStore()
	store.Set("default", vault.Info{
		ECDSAPublicKey: testECDSAPubKey,
		EdDSAPublicKey: testEdDSAPubKey,
		ChainCode:      testChainCode,
	})

	handler := handleBuildSolanaTx(store, solanaclient.NewClient("https://localhost:0"))
	ctx := context.Background()

	req := callToolReq("build_solana_tx", map[string]any{
		"to":     "11111111111111111111111111111111",
		"amount": "1000000",
	})

	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	// The RPC call will fail (localhost:0), but address derivation should succeed first.
	// We expect an RPC error, not an address derivation error.
	if !res.IsError {
		// If somehow it didn't error (unexpected), verify the result structure
		text := resultText(t, res)
		var result types.TransactionResult
		err = json.Unmarshal([]byte(text), &result)
		if err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if result.Transactions[0].SigningMode != types.SigningModeEdDSA {
			t.Errorf("signing mode = %q, want %q", result.Transactions[0].SigningMode, types.SigningModeEdDSA)
		}
		return
	}

	// Verify the error is from RPC, not from address derivation
	tc, ok := res.Content[0].(interface{ MarshalJSON() ([]byte, error) })
	if !ok {
		return
	}
	data, _ := json.Marshal(tc)
	errStr := string(data)
	if strings.Contains(errStr, "vault info") || strings.Contains(errStr, "derive") {
		t.Fatalf("expected RPC error, got address error: %s", errStr)
	}
}

func TestBuildSolanaTx_Integration(t *testing.T) {
	skipUnlessSolanaTest(t)

	from := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	to := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")

	client := solanaclient.NewClient("https://api.mainnet-beta.solana.com")

	txBytes, err := client.BuildNativeTransfer(context.Background(), from, to, 1_000_000)
	if err != nil {
		t.Fatalf("build native transfer: %v", err)
	}

	tx, err := solana.TransactionFromBytes(txBytes)
	if err != nil {
		t.Fatalf("parse tx bytes: %v", err)
	}

	if tx.Message.Header.NumRequiredSignatures != 1 {
		t.Errorf("expected 1 required signature, got %d", tx.Message.Header.NumRequiredSignatures)
	}

	hexStr := hex.EncodeToString(txBytes)
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("hex roundtrip failed: %v", err)
	}
	_, err = solana.TransactionFromBytes(decoded)
	if err != nil {
		t.Fatalf("tx roundtrip failed: %v", err)
	}
}

func skipUnlessSolanaTest(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping Solana integration test in short mode")
	}
}
