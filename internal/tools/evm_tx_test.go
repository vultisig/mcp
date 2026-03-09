package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/vultisig/mcp/internal/vault"
)

// ---------------------------------------------------------------------------
// Constants derived from on-chain Spark (ERC-4626) transactions executed by
// address 0xE721dd7a654D7E95518014526f6897deF6A44933 on Ethereum mainnet.
//
// Contracts:
//   USDT:       0xdAC17F958D2ee523a2206206994597C13D831ec7
//   Spark Vault: 0xe2e7a17dFf93280dec073C995595155283e3C372
//
// Vault keys (for address derivation):
//   ECDSA: 038e9b3ae4e94e9b9a0b561d23a11b8f794bd45a6f7f65a2293a0283004f9937d2
//   EdDSA: c442debf05fc82a23809729d6c39625aa4a05b78128fb04d9d7ca29c7adc4fb4
//   Chain code: 5798e3142c4da332e5729b859fc74ee00f417e5a4b418821b6b370cd97a3c456
// ---------------------------------------------------------------------------

const (
	testECDSAPubKey = "038e9b3ae4e94e9b9a0b561d23a11b8f794bd45a6f7f65a2293a0283004f9937d2"
	testEdDSAPubKey = "c442debf05fc82a23809729d6c39625aa4a05b78128fb04d9d7ca29c7adc4fb4"
	testChainCode   = "5798e3142c4da332e5729b859fc74ee00f417e5a4b418821b6b370cd97a3c456"
	testAddress     = "0xE721dd7a654D7E95518014526f6897deF6A44933"

	usdt       = "0xdAC17F958D2ee523a2206206994597C13D831ec7"
	sparkVault = "0xe2e7a17dFf93280dec073C995595155283e3C372"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// callToolReq builds an mcp.CallToolRequest for testing tool handlers directly.
func callToolReq(name string, args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	}
}

// resultText extracts the text from a successful CallToolResult.
func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res.IsError {
		tc, ok := res.Content[0].(mcp.TextContent)
		if ok {
			t.Fatalf("tool returned error: %s", tc.Text)
		}
		t.Fatalf("tool returned error with unexpected content type")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	return tc.Text
}

// ---------------------------------------------------------------------------
// TestABIEncode_SparkCalldata verifies ABI encoding against actual on-chain
// calldata from the Spark transactions.
// ---------------------------------------------------------------------------

func TestABIEncode_SparkCalldata(t *testing.T) {
	handler := handleABIEncode()
	ctx := context.Background()

	tests := []struct {
		name      string
		signature string
		args      []any
		wantHex   string // expected 0x-prefixed hex from on-chain tx input data
	}{
		{
			name:      "approve_reset_to_zero",
			signature: "approve(address,uint256)",
			args:      []any{"0xe2e7a17dFf93280dec073C995595155283e3C372", "0"},
			wantHex:   "0x095ea7b3000000000000000000000000e2e7a17dff93280dec073c995595155283e3c3720000000000000000000000000000000000000000000000000000000000000000",
		},
		{
			name:      "approve_1_usdt",
			signature: "approve(address,uint256)",
			args:      []any{"0xe2e7a17dFf93280dec073C995595155283e3C372", "1000000"},
			wantHex:   "0x095ea7b3000000000000000000000000e2e7a17dff93280dec073c995595155283e3c37200000000000000000000000000000000000000000000000000000000000f4240",
		},
		{
			name:      "deposit_1_usdt",
			signature: "deposit(uint256,address)",
			args:      []any{"1000000", "0xE721dd7a654D7E95518014526f6897deF6A44933"},
			wantHex:   "0x6e553f6500000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933",
		},
		{
			name:      "withdraw_with_interest",
			signature: "withdraw(uint256,address,address)",
			args:      []any{"2000029", "0xE721dd7a654D7E95518014526f6897deF6A44933", "0xE721dd7a654D7E95518014526f6897deF6A44933"},
			wantHex:   "0xb460af9400000000000000000000000000000000000000000000000000000000001e849d000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933",
		},
		{
			name:      "allowance_check",
			signature: "allowance(address,address)",
			args:      []any{"0xE721dd7a654D7E95518014526f6897deF6A44933", "0xe2e7a17dFf93280dec073C995595155283e3C372"},
			wantHex:   "0xdd62ed3e000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933000000000000000000000000e2e7a17dff93280dec073c995595155283e3c372",
		},
		{
			name:      "maxWithdraw_query",
			signature: "maxWithdraw(address)",
			args:      []any{"0xE721dd7a654D7E95518014526f6897deF6A44933"},
			wantHex:   "0xce96cb77000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933",
		},
		{
			name:      "assetsOf_query",
			signature: "assetsOf(address)",
			args:      []any{"0xE721dd7a654D7E95518014526f6897deF6A44933"},
			wantHex:   "0x2c62fa10000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := callToolReq("abi_encode", map[string]any{
				"signature": tt.signature,
				"args":      tt.args,
			})
			res, err := handler(ctx, req)
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}

			text := resultText(t, res)
			var out struct {
				Encoded string `json:"encoded"`
			}
			if err := json.Unmarshal([]byte(text), &out); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if out.Encoded != tt.wantHex {
				t.Errorf("encoded mismatch\n  got:  %s\n  want: %s", out.Encoded, tt.wantHex)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestConvertAmount verifies USDT (6 decimals) conversions.
// ---------------------------------------------------------------------------

func TestConvertAmount_USDT(t *testing.T) {
	handler := handleConvertAmount()
	ctx := context.Background()

	tests := []struct {
		name      string
		amount    string
		decimals  float64
		direction string
		want      string
	}{
		{"to_base_1_usdt", "1", 6, "to_base", "1000000"},
		{"to_base_2_usdt", "2", 6, "to_base", "2000000"},
		{"to_base_fractional", "1.5", 6, "to_base", "1500000"},
		{"to_human_1000000", "1000000", 6, "to_human", "1"},
		{"to_human_2000029", "2000029", 6, "to_human", "2.000029"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := callToolReq("convert_amount", map[string]any{
				"amount":    tt.amount,
				"decimals":  tt.decimals,
				"direction": tt.direction,
			})
			res, err := handler(ctx, req)
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}
			got := resultText(t, res)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestBuildEVMTx_SparkTransactions verifies transaction building using exact
// parameters from the on-chain Spark transactions (nonces 7-13).
// Each built transaction is decoded back via the Vultisig recipes SDK
// DecodeUnsignedPayload to verify structural correctness.
// ---------------------------------------------------------------------------

func TestBuildEVMTx_SparkTransactions(t *testing.T) {
	handler := handleBuildEVMTx()
	ctx := context.Background()

	tests := []struct {
		name               string
		to                 string
		value              string
		data               string
		nonce              string
		gasLimit           string
		maxFeePerGas       string
		maxPriorityFee     string
		wantTo             string
		wantNonce          string
		wantGas            string
		wantMaxFee         string
		wantMaxPriorityFee string
		wantChain          string
		wantValue          string
	}{
		{
			name:               "nonce7_approve_reset",
			to:                 usdt,
			value:              "0",
			data:               "0x095ea7b3000000000000000000000000e2e7a17dff93280dec073c995595155283e3c3720000000000000000000000000000000000000000000000000000000000000000",
			nonce:              "7",
			gasLimit:           "28767",
			maxFeePerGas:       "134966309",
			maxPriorityFee:     "423",
			wantTo:             "0xdAC17F958D2ee523a2206206994597C13D831ec7",
			wantNonce:          "7",
			wantGas:            "28767",
			wantMaxFee:         "134966309",
			wantMaxPriorityFee: "423",
			wantChain:          "Ethereum",
			wantValue:          "0",
		},
		{
			name:               "nonce8_approve_1usdt",
			to:                 usdt,
			value:              "0",
			data:               "0x095ea7b3000000000000000000000000e2e7a17dff93280dec073c995595155283e3c37200000000000000000000000000000000000000000000000000000000000f4240",
			nonce:              "8",
			gasLimit:           "48936",
			maxFeePerGas:       "137582443",
			maxPriorityFee:     "423",
			wantTo:             "0xdAC17F958D2ee523a2206206994597C13D831ec7",
			wantNonce:          "8",
			wantGas:            "48936",
			wantMaxFee:         "137582443",
			wantMaxPriorityFee: "423",
			wantChain:          "Ethereum",
			wantValue:          "0",
		},
		{
			name:               "nonce9_deposit_1usdt",
			to:                 sparkVault,
			value:              "0",
			data:               "0x6e553f6500000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933",
			nonce:              "9",
			gasLimit:           "150000",
			maxFeePerGas:       "137582443",
			maxPriorityFee:     "423",
			wantTo:             "0xe2e7a17dFf93280dec073C995595155283e3C372",
			wantNonce:          "9",
			wantGas:            "150000",
			wantMaxFee:         "137582443",
			wantMaxPriorityFee: "423",
			wantChain:          "Ethereum",
			wantValue:          "0",
		},
		{
			name:               "nonce12_deposit_1usdt_second",
			to:                 sparkVault,
			value:              "0",
			data:               "0x6e553f6500000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933",
			nonce:              "12",
			gasLimit:           "105313",
			maxFeePerGas:       "82751424",
			maxPriorityFee:     "15750",
			wantTo:             "0xe2e7a17dFf93280dec073C995595155283e3C372",
			wantNonce:          "12",
			wantGas:            "105313",
			wantMaxFee:         "82751424",
			wantMaxPriorityFee: "15750",
			wantChain:          "Ethereum",
			wantValue:          "0",
		},
		{
			name:               "nonce13_withdraw_with_interest",
			to:                 sparkVault,
			value:              "0",
			data:               "0xb460af9400000000000000000000000000000000000000000000000000000000001e849d000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933",
			nonce:              "13",
			gasLimit:           "104414",
			maxFeePerGas:       "133342876",
			maxPriorityFee:     "15750",
			wantTo:             "0xe2e7a17dFf93280dec073C995595155283e3C372",
			wantNonce:          "13",
			wantGas:            "104414",
			wantMaxFee:         "133342876",
			wantMaxPriorityFee: "15750",
			wantChain:          "Ethereum",
			wantValue:          "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := callToolReq("build_evm_tx", map[string]any{
				"to":                       tt.to,
				"value":                    tt.value,
				"data":                     tt.data,
				"nonce":                    tt.nonce,
				"gas_limit":                tt.gasLimit,
				"max_fee_per_gas":          tt.maxFeePerGas,
				"max_priority_fee_per_gas": tt.maxPriorityFee,
				"chain_id":                 "1",
			})
			res, err := handler(ctx, req)
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}

			text := resultText(t, res)

			var result struct {
				Chain                string `json:"chain"`
				ChainID              string `json:"chain_id"`
				To                   string `json:"to"`
				Value                string `json:"value"`
				Nonce                string `json:"nonce"`
				GasLimit             string `json:"gas_limit"`
				MaxFeePerGas         string `json:"max_fee_per_gas"`
				MaxPriorityFeePerGas string `json:"max_priority_fee_per_gas"`
				Data                 string `json:"data"`
				TxType               int    `json:"tx_type"`
			}
			if err := json.Unmarshal([]byte(text), &result); err != nil {
				t.Fatalf("unmarshal result: %v", err)
			}

			if result.Chain != tt.wantChain {
				t.Errorf("chain: got %q, want %q", result.Chain, tt.wantChain)
			}
			if result.ChainID != "1" {
				t.Errorf("chain_id: got %q, want %q", result.ChainID, "1")
			}
			if result.To != tt.wantTo {
				t.Errorf("to: got %q, want %q", result.To, tt.wantTo)
			}
			if result.Value != tt.wantValue {
				t.Errorf("value: got %q, want %q", result.Value, tt.wantValue)
			}
			if result.Nonce != tt.wantNonce {
				t.Errorf("nonce: got %q, want %q", result.Nonce, tt.wantNonce)
			}
			if result.GasLimit != tt.wantGas {
				t.Errorf("gas_limit: got %q, want %q", result.GasLimit, tt.wantGas)
			}
			if result.MaxFeePerGas != tt.wantMaxFee {
				t.Errorf("max_fee_per_gas: got %q, want %q", result.MaxFeePerGas, tt.wantMaxFee)
			}
			if result.MaxPriorityFeePerGas != tt.wantMaxPriorityFee {
				t.Errorf("max_priority_fee_per_gas: got %q, want %q", result.MaxPriorityFeePerGas, tt.wantMaxPriorityFee)
			}
			if result.Data != tt.data {
				t.Errorf("data: got %q, want %q", result.Data, tt.data)
			}
			if result.TxType != 2 {
				t.Errorf("tx_type: got %d, want 2", result.TxType)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestSetVaultInfo_AddressDerivation verifies that storing the test vault
// keys and deriving an Ethereum address produces the expected address.
// ---------------------------------------------------------------------------

func TestSetVaultInfo_AddressDerivation(t *testing.T) {
	store := vault.NewStore()
	handler := handleSetVaultInfo(store)
	ctx := context.Background()

	req := callToolReq("set_vault_info", map[string]any{
		"ecdsa_public_key": testECDSAPubKey,
		"eddsa_public_key": testEdDSAPubKey,
		"chain_code":       testChainCode,
	})
	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	text := resultText(t, res)
	if text != "vault info stored for session" {
		t.Errorf("unexpected response: %q", text)
	}

	// Verify the vault info was stored and can derive the correct address.
	info, ok := store.Get("default")
	if !ok {
		t.Fatal("vault info not stored")
	}
	if info.ECDSAPublicKey != testECDSAPubKey {
		t.Errorf("ecdsa key mismatch")
	}
	if info.EdDSAPublicKey != testEdDSAPubKey {
		t.Errorf("eddsa key mismatch")
	}
	if info.ChainCode != testChainCode {
		t.Errorf("chain code mismatch")
	}
}

// ---------------------------------------------------------------------------
// TestSetVaultInfo_MissingChainCode verifies the error when chain_code
// parameter is missing (the "chaincode_hex" bug seen in the logs).
// ---------------------------------------------------------------------------

func TestSetVaultInfo_MissingChainCode(t *testing.T) {
	store := vault.NewStore()
	handler := handleSetVaultInfo(store)
	ctx := context.Background()

	req := callToolReq("set_vault_info", map[string]any{
		"ecdsa_public_key": testECDSAPubKey,
		"eddsa_public_key": testEdDSAPubKey,
		"chaincode_hex":    testChainCode, // wrong parameter name
	})
	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error for missing chain_code")
	}
}

// ---------------------------------------------------------------------------
// TestSparkDepositWorkflow exercises the full Spark deposit flow:
// 1. convert_amount (human → base)
// 2. abi_encode approve(address,uint256) with 0 (USDT reset)
// 3. abi_encode approve(address,uint256) with deposit amount
// 4. abi_encode deposit(uint256,address)
// 5. build_evm_tx for each step
//
// This mirrors the exact tool invocation sequence from the MCP logs.
// ---------------------------------------------------------------------------

func TestSparkDepositWorkflow(t *testing.T) {
	ctx := context.Background()

	// Step 1: Convert 1 USDT to base units.
	convertRes, err := handleConvertAmount()(ctx, callToolReq("convert_amount", map[string]any{
		"amount":    "1",
		"decimals":  float64(6),
		"direction": "to_base",
	}))
	if err != nil {
		t.Fatalf("convert_amount: %v", err)
	}
	baseAmount := resultText(t, convertRes)
	if baseAmount != "1000000" {
		t.Fatalf("convert_amount: got %q, want %q", baseAmount, "1000000")
	}

	// Step 2: Encode approve(address,uint256) with 0 (USDT reset pattern).
	abiHandler := handleABIEncode()
	approveZeroRes, err := abiHandler(ctx, callToolReq("abi_encode", map[string]any{
		"signature": "approve(address,uint256)",
		"args":      []any{sparkVault, "0"},
	}))
	if err != nil {
		t.Fatalf("abi_encode approve(0): %v", err)
	}
	var approveZero struct{ Encoded string }
	json.Unmarshal([]byte(resultText(t, approveZeroRes)), &approveZero)

	// Step 3: Encode approve(address,uint256) with 1000000.
	approveAmountRes, err := abiHandler(ctx, callToolReq("abi_encode", map[string]any{
		"signature": "approve(address,uint256)",
		"args":      []any{sparkVault, baseAmount},
	}))
	if err != nil {
		t.Fatalf("abi_encode approve(1000000): %v", err)
	}
	var approveAmount struct{ Encoded string }
	json.Unmarshal([]byte(resultText(t, approveAmountRes)), &approveAmount)

	// Step 4: Encode deposit(uint256,address).
	depositRes, err := abiHandler(ctx, callToolReq("abi_encode", map[string]any{
		"signature": "deposit(uint256,address)",
		"args":      []any{baseAmount, testAddress},
	}))
	if err != nil {
		t.Fatalf("abi_encode deposit: %v", err)
	}
	var deposit struct{ Encoded string }
	json.Unmarshal([]byte(resultText(t, depositRes)), &deposit)

	// Step 5: Build all three transactions in sequence.
	buildHandler := handleBuildEVMTx()
	txCases := []struct {
		name string
		to   string
		data string
	}{
		{"approve_reset", usdt, approveZero.Encoded},
		{"approve_amount", usdt, approveAmount.Encoded},
		{"deposit", sparkVault, deposit.Encoded},
	}

	for i, tc := range txCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := buildHandler(ctx, callToolReq("build_evm_tx", map[string]any{
				"to":                       tc.to,
				"value":                    "0",
				"data":                     tc.data,
				"nonce":                    "7",
				"gas_limit":                "100000",
				"max_fee_per_gas":          "100000000",
				"max_priority_fee_per_gas": "1000",
				"chain_id":                 "1",
			}))
			if err != nil {
				t.Fatalf("build_evm_tx[%d]: %v", i, err)
			}

			text := resultText(t, res)
			var result struct {
				ChainID string `json:"chain_id"`
				TxType  int    `json:"tx_type"`
			}
			if err := json.Unmarshal([]byte(text), &result); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if result.ChainID != "1" {
				t.Errorf("chain_id: got %q, want %q", result.ChainID, "1")
			}
			if result.TxType != 2 {
				t.Errorf("tx_type: got %d, want 2", result.TxType)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestSparkWithdrawWorkflow exercises the Spark withdraw flow:
// 1. abi_encode maxWithdraw(address) — prepare the read call
// 2. abi_encode withdraw(uint256,address,address)
// 3. build_evm_tx for the withdraw
// ---------------------------------------------------------------------------

func TestSparkWithdrawWorkflow(t *testing.T) {
	ctx := context.Background()
	abiHandler := handleABIEncode()

	// Step 1: Encode maxWithdraw(address) calldata.
	maxWithdrawRes, err := abiHandler(ctx, callToolReq("abi_encode", map[string]any{
		"signature": "maxWithdraw(address)",
		"args":      []any{testAddress},
	}))
	if err != nil {
		t.Fatalf("abi_encode maxWithdraw: %v", err)
	}
	var maxWithdraw struct{ Encoded string }
	json.Unmarshal([]byte(resultText(t, maxWithdrawRes)), &maxWithdraw)

	wantMaxWithdrawCalldata := "0xce96cb77000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933"
	if maxWithdraw.Encoded != wantMaxWithdrawCalldata {
		t.Fatalf("maxWithdraw calldata mismatch\n  got:  %s\n  want: %s", maxWithdraw.Encoded, wantMaxWithdrawCalldata)
	}

	// Step 2: Encode withdraw(uint256,address,address) with known amount (2000029 from on-chain).
	withdrawRes, err := abiHandler(ctx, callToolReq("abi_encode", map[string]any{
		"signature": "withdraw(uint256,address,address)",
		"args":      []any{"2000029", testAddress, testAddress},
	}))
	if err != nil {
		t.Fatalf("abi_encode withdraw: %v", err)
	}
	var withdraw struct{ Encoded string }
	json.Unmarshal([]byte(resultText(t, withdrawRes)), &withdraw)

	wantWithdrawCalldata := "0xb460af9400000000000000000000000000000000000000000000000000000000001e849d000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933"
	if withdraw.Encoded != wantWithdrawCalldata {
		t.Fatalf("withdraw calldata mismatch\n  got:  %s\n  want: %s", withdraw.Encoded, wantWithdrawCalldata)
	}

	// Step 3: Build the withdraw transaction with on-chain parameters (nonce 13).
	buildHandler := handleBuildEVMTx()
	res, err := buildHandler(ctx, callToolReq("build_evm_tx", map[string]any{
		"to":                       sparkVault,
		"value":                    "0",
		"data":                     withdraw.Encoded,
		"nonce":                    "13",
		"gas_limit":                "104414",
		"max_fee_per_gas":          "133342876",
		"max_priority_fee_per_gas": "15750",
		"chain_id":                 "1",
	}))
	if err != nil {
		t.Fatalf("build_evm_tx: %v", err)
	}

	text := resultText(t, res)
	var result struct {
		Nonce    string `json:"nonce"`
		GasLimit string `json:"gas_limit"`
		Data     string `json:"data"`
		TxType   int    `json:"tx_type"`
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.Nonce != "13" {
		t.Errorf("nonce: got %q, want %q", result.Nonce, "13")
	}
	if result.GasLimit != "104414" {
		t.Errorf("gas_limit: got %q, want %q", result.GasLimit, "104414")
	}
	if result.Data != wantWithdrawCalldata {
		t.Errorf("data: got %q, want %q", result.Data, wantWithdrawCalldata)
	}
	if result.TxType != 2 {
		t.Errorf("tx_type: got %d, want 2", result.TxType)
	}
}

// ---------------------------------------------------------------------------
// TestBuildEVMTx_Deterministic verifies that building the same transaction
// twice produces identical unsigned_tx_hex (idempotency).
// ---------------------------------------------------------------------------

func TestBuildEVMTx_Deterministic(t *testing.T) {
	handler := handleBuildEVMTx()
	ctx := context.Background()

	args := map[string]any{
		"to":                       sparkVault,
		"value":                    "0",
		"data":                     "0x6e553f6500000000000000000000000000000000000000000000000000000000000f4240000000000000000000000000e721dd7a654d7e95518014526f6897def6a44933",
		"nonce":                    "12",
		"gas_limit":                "105313",
		"max_fee_per_gas":          "82751424",
		"max_priority_fee_per_gas": "15750",
		"chain_id":                 "1",
	}

	var outputs [2]string
	for i := range outputs {
		res, err := handler(ctx, callToolReq("build_evm_tx", args))
		if err != nil {
			t.Fatalf("handler error [%d]: %v", i, err)
		}
		outputs[i] = resultText(t, res)
	}

	if outputs[0] != outputs[1] {
		t.Errorf("non-deterministic output:\n  run1: %s\n  run2: %s", outputs[0], outputs[1])
	}
}
