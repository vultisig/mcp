package tools

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"

	"github.com/mark3labs/mcp-go/mcp"
	btcsdk "github.com/vultisig/recipes/sdk/btc"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/thorchain"
	"github.com/vultisig/mcp/internal/vault"
)

func setupBTCVault(t *testing.T) *vault.Store {
	t.Helper()
	store := vault.NewStore()
	handler := handleSetVaultInfo(store)
	req := callToolReq("set_vault_info", map[string]any{
		"ecdsa_public_key": testECDSAPubKey,
		"eddsa_public_key": testEdDSAPubKey,
		"chain_code":       testChainCode,
	})
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("set_vault_info failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("set_vault_info returned error")
	}
	return store
}

func deriveTestBTCAddress(t *testing.T) (string, []byte) {
	t.Helper()
	addr, derivedPubKey, _, err := address.GetAddress(testECDSAPubKey, testChainCode, common.Bitcoin)
	if err != nil {
		t.Fatalf("derive BTC address: %v", err)
	}
	pubBytes, err := hex.DecodeString(derivedPubKey)
	if err != nil {
		t.Fatalf("decode derived pubkey: %v", err)
	}
	return addr, pubBytes
}

// buildPrevTx creates a serialized wire.MsgTx with a P2WPKH output paying to senderAddr.
func buildPrevTx(t *testing.T, senderAddr string, outputValue int64) string {
	t.Helper()

	btcChain := utxoChains["Bitcoin"]
	pkScript, err := btcChain.addressToPkScript(senderAddr)
	if err != nil {
		t.Fatalf("address to pkscript: %v", err)
	}

	dummyHash, _ := chainhash.NewHashFromStr("0000000000000000000000000000000000000000000000000000000000000000")
	msgTx := wire.NewMsgTx(2)
	msgTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: *dummyHash, Index: 0},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	msgTx.AddTxOut(wire.NewTxOut(outputValue, pkScript))

	var buf bytes.Buffer
	err = msgTx.Serialize(&buf)
	if err != nil {
		t.Fatalf("serialize prev tx: %v", err)
	}
	return hex.EncodeToString(buf.Bytes())
}

func mockBlockchairServer(t *testing.T, utxos []blockchair.UTXO, rawTxs map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := splitURLPath(r.URL.Path)

		// /{chain}/dashboards/address/{addr}
		if len(parts) >= 4 && parts[1] == "dashboards" && parts[2] == "address" {
			addr := parts[3]
			var total int64
			for _, u := range utxos {
				total += u.Value
			}
			resp := map[string]any{
				"data": map[string]any{
					addr: map[string]any{
						"address": map[string]any{
							"balance":              total,
							"unspent_output_count": len(utxos),
						},
						"transactions": []string{},
						"utxo":         utxos,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		// /{chain}/raw/transaction/{txhash}
		if len(parts) >= 4 && parts[1] == "raw" && parts[2] == "transaction" {
			txHash := parts[3]
			rawHex, ok := rawTxs[txHash]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			resp := map[string]any{
				"data": map[string]any{
					txHash: map[string]any{
						"raw_transaction": rawHex,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		http.Error(w, "unknown path: "+r.URL.Path, http.StatusNotFound)
	}))
}

func splitURLPath(path string) []string {
	var parts []string
	for _, p := range strings.Split(path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func mockThorchainServer(t *testing.T, gasRate string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addresses := []map[string]any{
			{"chain": "BTC", "address": "bc1qfake", "gas_rate": gasRate, "gas_rate_units": "satsperbyte", "halted": false},
			{"chain": "ETH", "address": "0xfake", "gas_rate": "30", "gas_rate_units": "gwei", "halted": false},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(addresses)
	}))
}

func TestBTCFeeRate(t *testing.T) {
	srv := mockThorchainServer(t, "15")
	defer srv.Close()

	tcClient := thorchain.NewClient(srv.URL)
	handler := handleBTCFeeRate(tcClient)

	req := callToolReq("btc_fee_rate", map[string]any{})
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := resultText(t, res)
	var result feeRateResult
	err = json.Unmarshal([]byte(text), &result)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result.Chain != "Bitcoin" {
		t.Errorf("chain: got %s, want Bitcoin", result.Chain)
	}
	if result.FeeRate != 15 {
		t.Errorf("fee_rate: got %d, want 15", result.FeeRate)
	}
	if result.FeeRateUnit != "sat/vB" {
		t.Errorf("fee_rate_unit: got %s, want sat/vB", result.FeeRateUnit)
	}
}

func mockThorchainServerHalted(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		addresses := []map[string]any{
			{"chain": "BTC", "address": "bc1qfake", "gas_rate": "15", "gas_rate_units": "satsperbyte", "halted": true},
			{"chain": "ETH", "address": "0xfake", "gas_rate": "30", "gas_rate_units": "gwei", "halted": false},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(addresses)
	}))
}

func TestBTCFeeRate_HaltedChain(t *testing.T) {
	srv := mockThorchainServerHalted(t)
	defer srv.Close()

	tcClient := thorchain.NewClient(srv.URL)
	handler := handleBTCFeeRate(tcClient)

	req := callToolReq("btc_fee_rate", map[string]any{})
	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error for halted chain")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if !strings.Contains(tc.Text, "halted") {
		t.Errorf("expected halted error message, got: %s", tc.Text)
	}
}

func TestBuildBTCSend_NegativeUTXOSkipped(t *testing.T) {
	store := setupBTCVault(t)
	senderAddr, _ := deriveTestBTCAddress(t)

	prevTxHash := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	prevTxHex := buildPrevTx(t, senderAddr, 100000)

	utxos := []blockchair.UTXO{
		{TransactionHash: "bad0bad0bad0bad0bad0bad0bad0bad0bad0bad0bad0bad0bad0bad0bad0bad0", Index: 0, Value: -500, BlockID: 800010},
		{TransactionHash: "bad1bad1bad1bad1bad1bad1bad1bad1bad1bad1bad1bad1bad1bad1bad1bad1", Index: -1, Value: 1000, BlockID: 800010},
		{TransactionHash: prevTxHash, Index: 0, Value: 100000, BlockID: 800010},
	}
	rawTxs := map[string]string{prevTxHash: prevTxHex}

	srv := mockBlockchairServer(t, utxos, rawTxs)
	defer srv.Close()

	handler := handleBuildBTCSend(store, btcsdk.Mainnet(), blockchair.NewClient(srv.URL))

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
		t.Fatalf("expected success but got error: %s", resultText(t, res))
	}

	text := resultText(t, res)
	var txResult struct {
		Transactions []struct {
			TxDetails map[string]string `json:"tx_details"`
		} `json:"transactions"`
	}
	err = json.Unmarshal([]byte(text), &txResult)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if txResult.Transactions[0].TxDetails["input_count"] != "1" {
		t.Errorf("expected 1 input (invalid UTXOs skipped), got %s", txResult.Transactions[0].TxDetails["input_count"])
	}
}

func TestBuildBTCSend_Basic(t *testing.T) {
	store := setupBTCVault(t)
	senderAddr, _ := deriveTestBTCAddress(t)

	prevTxHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	prevTxHex := buildPrevTx(t, senderAddr, 100000)

	utxos := []blockchair.UTXO{{TransactionHash: prevTxHash, Index: 0, Value: 100000, BlockID: 800000}}
	rawTxs := map[string]string{prevTxHash: prevTxHex}

	srv := mockBlockchairServer(t, utxos, rawTxs)
	defer srv.Close()

	handler := handleBuildBTCSend(store, btcsdk.Mainnet(), blockchair.NewClient(srv.URL))

	req := callToolReq("build_btc_send", map[string]any{
		"to_address": "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		"amount":     "50000",
		"fee_rate":   float64(10),
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, res)

	var txResult struct {
		Transactions []struct {
			Chain         string            `json:"chain"`
			Action        string            `json:"action"`
			SigningMode   string            `json:"signing_mode"`
			UnsignedTxHex string            `json:"unsigned_tx_hex"`
			TxDetails     map[string]string `json:"tx_details"`
		} `json:"transactions"`
	}
	err = json.Unmarshal([]byte(text), &txResult)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(txResult.Transactions) != 1 {
		t.Fatalf("expected 1 tx, got %d", len(txResult.Transactions))
	}

	tx := txResult.Transactions[0]
	if tx.Chain != "Bitcoin" {
		t.Errorf("chain: got %s, want Bitcoin", tx.Chain)
	}
	if tx.Action != "transfer" {
		t.Errorf("action: got %s, want transfer", tx.Action)
	}
	if tx.SigningMode != "ecdsa_secp256k1" {
		t.Errorf("signing_mode: got %s, want ecdsa_secp256k1", tx.SigningMode)
	}
	if tx.TxDetails["tx_encoding"] != "psbt" {
		t.Errorf("tx_encoding: got %s, want psbt", tx.TxDetails["tx_encoding"])
	}
	if tx.TxDetails["from"] != senderAddr {
		t.Errorf("from: got %s, want %s", tx.TxDetails["from"], senderAddr)
	}

	// Parse PSBT
	psbtBytes, err := hex.DecodeString(tx.UnsignedTxHex)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}
	pkt, err := psbt.NewFromRawBytes(bytes.NewReader(psbtBytes), false)
	if err != nil {
		t.Fatalf("parse PSBT: %v", err)
	}

	if len(pkt.UnsignedTx.TxOut) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(pkt.UnsignedTx.TxOut))
	}
	if pkt.UnsignedTx.TxOut[0].Value != 50000 {
		t.Errorf("recipient value: got %d, want 50000", pkt.UnsignedTx.TxOut[0].Value)
	}
}

func TestBuildBTCSend_WithMemo(t *testing.T) {
	store := setupBTCVault(t)
	senderAddr, _ := deriveTestBTCAddress(t)

	prevTxHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	prevTxHex := buildPrevTx(t, senderAddr, 500000)

	utxos := []blockchair.UTXO{{TransactionHash: prevTxHash, Index: 0, Value: 500000, BlockID: 800001}}
	rawTxs := map[string]string{prevTxHash: prevTxHex}

	srv := mockBlockchairServer(t, utxos, rawTxs)
	defer srv.Close()

	handler := handleBuildBTCSend(store, btcsdk.Mainnet(), blockchair.NewClient(srv.URL))

	memo := "=:ETH.ETH:0x1234567890abcdef1234567890abcdef12345678"
	req := callToolReq("build_btc_send", map[string]any{
		"to_address": "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		"amount":     "200000",
		"fee_rate":   float64(15),
		"memo":       memo,
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, res)

	var txResult struct {
		Transactions []struct {
			Action        string `json:"action"`
			UnsignedTxHex string `json:"unsigned_tx_hex"`
		} `json:"transactions"`
	}
	err = json.Unmarshal([]byte(text), &txResult)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if txResult.Transactions[0].Action != "swap" {
		t.Errorf("action: got %s, want swap", txResult.Transactions[0].Action)
	}

	psbtBytes, err := hex.DecodeString(txResult.Transactions[0].UnsignedTxHex)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}
	pkt, err := psbt.NewFromRawBytes(bytes.NewReader(psbtBytes), false)
	if err != nil {
		t.Fatalf("parse PSBT: %v", err)
	}

	if len(pkt.UnsignedTx.TxOut) != 3 {
		t.Fatalf("expected 3 outputs (vault+change+OP_RETURN), got %d", len(pkt.UnsignedTx.TxOut))
	}

	// Output 2 should be OP_RETURN
	opReturnOut := pkt.UnsignedTx.TxOut[2]
	if opReturnOut.Value != 0 {
		t.Errorf("OP_RETURN value: got %d, want 0", opReturnOut.Value)
	}
	if !txscript.IsUnspendable(opReturnOut.PkScript) {
		t.Error("OP_RETURN output should be unspendable")
	}
}

func TestBuildBTCSend_InsufficientFunds(t *testing.T) {
	store := setupBTCVault(t)

	utxos := []blockchair.UTXO{{
		TransactionHash: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Index:           0,
		Value:           1000,
		BlockID:         800002,
	}}

	srv := mockBlockchairServer(t, utxos, map[string]string{})
	defer srv.Close()

	handler := handleBuildBTCSend(store, btcsdk.Mainnet(), blockchair.NewClient(srv.URL))

	req := callToolReq("build_btc_send", map[string]any{
		"to_address": "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		"amount":     "100000",
		"fee_rate":   float64(10),
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error for insufficient funds")
	}
}

func TestBuildBTCSend_PSBTMetadata(t *testing.T) {
	store := setupBTCVault(t)
	senderAddr, _ := deriveTestBTCAddress(t)

	prevTxHash := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	prevTxHex := buildPrevTx(t, senderAddr, 200000)

	utxos := []blockchair.UTXO{{TransactionHash: prevTxHash, Index: 0, Value: 200000, BlockID: 800003}}
	rawTxs := map[string]string{prevTxHash: prevTxHex}

	srv := mockBlockchairServer(t, utxos, rawTxs)
	defer srv.Close()

	handler := handleBuildBTCSend(store, btcsdk.Mainnet(), blockchair.NewClient(srv.URL))

	req := callToolReq("build_btc_send", map[string]any{
		"to_address": "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		"amount":     "80000",
		"fee_rate":   float64(5),
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := resultText(t, res)

	var txResult struct {
		Transactions []struct {
			UnsignedTxHex string `json:"unsigned_tx_hex"`
		} `json:"transactions"`
	}
	err = json.Unmarshal([]byte(text), &txResult)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	psbtBytes, err := hex.DecodeString(txResult.Transactions[0].UnsignedTxHex)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}
	pkt, err := psbt.NewFromRawBytes(bytes.NewReader(psbtBytes), false)
	if err != nil {
		t.Fatalf("parse PSBT: %v", err)
	}

	for i, input := range pkt.Inputs {
		if input.WitnessUtxo == nil && input.NonWitnessUtxo == nil {
			t.Errorf("input %d: missing UTXO metadata", i)
		}
		if len(input.Bip32Derivation) == 0 {
			t.Errorf("input %d: missing BIP32 derivation", i)
		}
	}
}

func TestBuildBTCSend_SwapWorkflow(t *testing.T) {
	store := setupBTCVault(t)
	senderAddr, _ := deriveTestBTCAddress(t)

	// Step 1: Simulate build_swap_tx output (vault address, amount, memo)
	vaultAddr := "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4"
	swapAmount := "300000"
	swapMemo := "=:ETH.ETH:0xRecipientAddress:0"

	// Step 2: Get fee rate
	thorSrv := mockThorchainServer(t, "20")
	defer thorSrv.Close()
	tcClient := thorchain.NewClient(thorSrv.URL)

	feeHandler := handleBTCFeeRate(tcClient)
	feeRes, err := feeHandler(context.Background(), callToolReq("btc_fee_rate", map[string]any{}))
	if err != nil {
		t.Fatalf("fee rate error: %v", err)
	}
	feeText := resultText(t, feeRes)
	var feeResult feeRateResult
	err = json.Unmarshal([]byte(feeText), &feeResult)
	if err != nil {
		t.Fatalf("unmarshal fee: %v", err)
	}

	// Step 3: Build the BTC transaction
	prevTxHash := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	prevTxHex := buildPrevTx(t, senderAddr, 1000000)

	utxos := []blockchair.UTXO{{TransactionHash: prevTxHash, Index: 0, Value: 1000000, BlockID: 800004}}
	rawTxs := map[string]string{prevTxHash: prevTxHex}

	bcSrv := mockBlockchairServer(t, utxos, rawTxs)
	defer bcSrv.Close()

	sendHandler := handleBuildBTCSend(store, btcsdk.Mainnet(), blockchair.NewClient(bcSrv.URL))

	sendReq := callToolReq("build_btc_send", map[string]any{
		"to_address": vaultAddr,
		"amount":     swapAmount,
		"fee_rate":   float64(feeResult.FeeRate),
		"memo":       swapMemo,
	})

	sendRes, err := sendHandler(context.Background(), sendReq)
	if err != nil {
		t.Fatalf("build_btc_send error: %v", err)
	}
	sendText := resultText(t, sendRes)

	var txResult struct {
		Transactions []struct {
			Action        string            `json:"action"`
			UnsignedTxHex string            `json:"unsigned_tx_hex"`
			TxDetails     map[string]string `json:"tx_details"`
		} `json:"transactions"`
	}
	err = json.Unmarshal([]byte(sendText), &txResult)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tx := txResult.Transactions[0]
	if tx.Action != "swap" {
		t.Errorf("action: got %s, want swap", tx.Action)
	}
	if tx.TxDetails["to"] != vaultAddr {
		t.Errorf("to: got %s, want %s", tx.TxDetails["to"], vaultAddr)
	}
	if tx.TxDetails["amount"] != swapAmount {
		t.Errorf("amount: got %s, want %s", tx.TxDetails["amount"], swapAmount)
	}

	// Parse and verify PSBT structure
	psbtBytes, err := hex.DecodeString(tx.UnsignedTxHex)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}
	pkt, err := psbt.NewFromRawBytes(bytes.NewReader(psbtBytes), false)
	if err != nil {
		t.Fatalf("parse PSBT: %v", err)
	}

	// 3 outputs: vault + change + OP_RETURN
	if len(pkt.UnsignedTx.TxOut) != 3 {
		t.Fatalf("expected 3 outputs, got %d", len(pkt.UnsignedTx.TxOut))
	}

	// Output 0: vault payment
	if pkt.UnsignedTx.TxOut[0].Value != 300000 {
		t.Errorf("vault output value: got %d, want 300000", pkt.UnsignedTx.TxOut[0].Value)
	}

	// Output 1: change (should be positive)
	if pkt.UnsignedTx.TxOut[1].Value <= 0 {
		t.Errorf("change output should be positive, got %d", pkt.UnsignedTx.TxOut[1].Value)
	}

	// Output 2: OP_RETURN
	if !txscript.IsUnspendable(pkt.UnsignedTx.TxOut[2].PkScript) {
		t.Error("output 2 should be OP_RETURN")
	}

	// Fee should be reasonable
	fee := tx.TxDetails["fee"]
	if fee == "" || fee == "0" {
		t.Error("fee should be non-zero")
	}

	t.Logf("Swap workflow: %s sats to %s, fee=%s sats, %d inputs, %d outputs",
		swapAmount, vaultAddr, fee, len(pkt.UnsignedTx.TxIn), len(pkt.UnsignedTx.TxOut))
}

func TestBuildBTCSend_MemoTooLong(t *testing.T) {
	store := setupBTCVault(t)

	utxos := []blockchair.UTXO{{
		TransactionHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Index:           0,
		Value:           500000,
		BlockID:         800000,
	}}

	srv := mockBlockchairServer(t, utxos, map[string]string{})
	defer srv.Close()

	handler := handleBuildBTCSend(store, btcsdk.Mainnet(), blockchair.NewClient(srv.URL))

	longMemo := strings.Repeat("x", 81)
	req := callToolReq("build_btc_send", map[string]any{
		"to_address": "bc1qw508d6qejxtdg4y5r3zarvary0c5xw7kv8f3t4",
		"amount":     "100000",
		"fee_rate":   float64(10),
		"memo":       longMemo,
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error for memo too long")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if !strings.Contains(tc.Text, "memo too long") {
		t.Errorf("expected memo too long error, got: %s", tc.Text)
	}
}
