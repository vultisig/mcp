package tools

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"

	zcashsdk "github.com/vultisig/recipes/sdk/zcash"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/mayachain"
	"github.com/vultisig/mcp/internal/thorchain"
)

// buildPrevTxForChain creates a serialized wire.MsgTx with a P2PKH output paying to
// senderAddr using the given chain's address encoder.
func buildPrevTxForChain(t *testing.T, chainName, senderAddr string, outputValue int64) string {
	t.Helper()
	chain := utxoChains[chainName]
	pkScript, err := chain.addressToPkScript(senderAddr)
	if err != nil {
		t.Fatalf("address to pkscript for %s: %v", chainName, err)
	}

	dummyHash, _ := chainhash.NewHashFromStr("0000000000000000000000000000000000000000000000000000000000000000")
	msgTx := wire.NewMsgTx(chain.txVersion)
	msgTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: *dummyHash, Index: 0},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	msgTx.AddTxOut(wire.NewTxOut(outputValue, pkScript))

	var buf bytes.Buffer
	err = msgTx.Serialize(&buf)
	if err != nil {
		t.Fatalf("serialize prev tx for %s: %v", chainName, err)
	}
	return hex.EncodeToString(buf.Bytes())
}

// mockThorchainMulti creates a mock THORChain server returning gas rates for multiple chains.
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
		json.NewEncoder(w).Encode(addresses)
	}))
}

// mockMayachainServer creates a mock MayaChain server returning gas rates for multiple chains.
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
		json.NewEncoder(w).Encode(addresses)
	}))
}

// deriveChainAddr derives the vault address for a given chain using the test keys.
func deriveChainAddr(t *testing.T, chain common.Chain) string {
	t.Helper()
	addr, _, _, err := address.GetAddress(testECDSAPubKey, testChainCode, chain)
	if err != nil {
		t.Fatalf("derive address for %v: %v", chain, err)
	}
	return addr
}

// Fee rate tests

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

// PSBT chain tests

func TestBuildLTCSend_Basic(t *testing.T) {
	store := setupBTCVault(t)
	senderAddr := deriveChainAddr(t, common.Litecoin)

	prevTxHash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	prevTxHex := buildPrevTxForChain(t, "Litecoin", senderAddr, 200000)

	utxos := []blockchair.UTXO{{TransactionHash: prevTxHash, Index: 0, Value: 200000, BlockID: 800000}}
	rawTxs := map[string]string{prevTxHash: prevTxHex}

	srv := mockBlockchairServer(t, utxos, rawTxs)
	defer srv.Close()

	handler := handleBuildLTCSend(store, blockchair.NewClient(srv.URL))
	req := callToolReq("build_ltc_send", map[string]any{
		"to_address": senderAddr,
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
	if tx.Chain != "Litecoin" {
		t.Errorf("chain: got %s, want Litecoin", tx.Chain)
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
	if tx.TxDetails["ticker"] != "LTC" {
		t.Errorf("ticker: got %s, want LTC", tx.TxDetails["ticker"])
	}
	if tx.TxDetails["from"] != senderAddr {
		t.Errorf("from: got %s, want %s", tx.TxDetails["from"], senderAddr)
	}

	psbtBytes, err := hex.DecodeString(tx.UnsignedTxHex)
	if err != nil {
		t.Fatalf("decode hex: %v", err)
	}
	pkt, err := psbt.NewFromRawBytes(bytes.NewReader(psbtBytes), false)
	if err != nil {
		t.Fatalf("parse PSBT: %v", err)
	}
	if pkt.UnsignedTx.TxOut[0].Value != 50000 {
		t.Errorf("recipient value: got %d, want 50000", pkt.UnsignedTx.TxOut[0].Value)
	}
}

func TestBuildLTCSend_InsufficientFunds(t *testing.T) {
	store := setupBTCVault(t)

	utxos := []blockchair.UTXO{{
		TransactionHash: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Index:           0, Value: 500, BlockID: 800000,
	}}
	srv := mockBlockchairServer(t, utxos, map[string]string{})
	defer srv.Close()

	handler := handleBuildLTCSend(store, blockchair.NewClient(srv.URL))
	req := callToolReq("build_ltc_send", map[string]any{
		"to_address": deriveChainAddr(t, common.Litecoin),
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

func TestBuildDOGESend_Basic(t *testing.T) {
	store := setupBTCVault(t)
	senderAddr := deriveChainAddr(t, common.Dogecoin)

	prevTxHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	prevTxHex := buildPrevTxForChain(t, "Dogecoin", senderAddr, 5_000_000_000)

	utxos := []blockchair.UTXO{{TransactionHash: prevTxHash, Index: 0, Value: 5_000_000_000, BlockID: 800000}}
	rawTxs := map[string]string{prevTxHash: prevTxHex}

	srv := mockBlockchairServer(t, utxos, rawTxs)
	defer srv.Close()

	handler := handleBuildDOGESend(store, blockchair.NewClient(srv.URL))
	req := callToolReq("build_doge_send", map[string]any{
		"to_address": senderAddr,
		"amount":     "2000000000",
		"fee_rate":   float64(1000),
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
			Chain     string            `json:"chain"`
			TxDetails map[string]string `json:"tx_details"`
		} `json:"transactions"`
	}
	err = json.Unmarshal([]byte(text), &txResult)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tx := txResult.Transactions[0]
	if tx.Chain != "Dogecoin" {
		t.Errorf("chain: got %s, want Dogecoin", tx.Chain)
	}
	if tx.TxDetails["ticker"] != "DOGE" {
		t.Errorf("ticker: got %s, want DOGE", tx.TxDetails["ticker"])
	}
	if tx.TxDetails["tx_encoding"] != "psbt" {
		t.Errorf("tx_encoding: got %s, want psbt", tx.TxDetails["tx_encoding"])
	}
}

func TestBuildBCHSend_Basic(t *testing.T) {
	store := setupBTCVault(t)
	senderAddr := deriveChainAddr(t, common.BitcoinCash)

	prevTxHash := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	prevTxHex := buildPrevTxForChain(t, "Bitcoin-Cash", senderAddr, 200000)

	utxos := []blockchair.UTXO{{TransactionHash: prevTxHash, Index: 0, Value: 200000, BlockID: 800000}}
	rawTxs := map[string]string{prevTxHash: prevTxHex}

	srv := mockBlockchairServer(t, utxos, rawTxs)
	defer srv.Close()

	handler := handleBuildBCHSend(store, blockchair.NewClient(srv.URL))
	req := callToolReq("build_bch_send", map[string]any{
		"to_address": senderAddr,
		"amount":     "50000",
		"fee_rate":   float64(5),
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
			Chain     string            `json:"chain"`
			TxDetails map[string]string `json:"tx_details"`
		} `json:"transactions"`
	}
	err = json.Unmarshal([]byte(text), &txResult)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if txResult.Transactions[0].Chain != "Bitcoin-Cash" {
		t.Errorf("chain: got %s, want Bitcoin-Cash", txResult.Transactions[0].Chain)
	}
	if txResult.Transactions[0].TxDetails["ticker"] != "BCH" {
		t.Errorf("ticker: got %s, want BCH", txResult.Transactions[0].TxDetails["ticker"])
	}
}

func TestBuildDASHSend_Basic(t *testing.T) {
	store := setupBTCVault(t)
	senderAddr := deriveChainAddr(t, common.Dash)

	prevTxHash := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	prevTxHex := buildPrevTxForChain(t, "Dash", senderAddr, 500000)

	utxos := []blockchair.UTXO{{TransactionHash: prevTxHash, Index: 0, Value: 500000, BlockID: 800000}}
	rawTxs := map[string]string{prevTxHash: prevTxHex}

	srv := mockBlockchairServer(t, utxos, rawTxs)
	defer srv.Close()

	handler := handleBuildDASHSend(store, blockchair.NewClient(srv.URL))
	req := callToolReq("build_dash_send", map[string]any{
		"to_address": senderAddr,
		"amount":     "100000",
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
			Chain     string            `json:"chain"`
			TxDetails map[string]string `json:"tx_details"`
		} `json:"transactions"`
	}
	err = json.Unmarshal([]byte(text), &txResult)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if txResult.Transactions[0].Chain != "Dash" {
		t.Errorf("chain: got %s, want Dash", txResult.Transactions[0].Chain)
	}
	if txResult.Transactions[0].TxDetails["ticker"] != "DASH" {
		t.Errorf("ticker: got %s, want DASH", txResult.Transactions[0].TxDetails["ticker"])
	}
}

// ZEC tests

func TestBuildZECSend_Basic(t *testing.T) {
	store := setupBTCVault(t)
	senderAddr := deriveChainAddr(t, common.Zcash)

	utxos := []blockchair.UTXO{{
		TransactionHash: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		Index:           0, Value: 500_000_000, BlockID: 800000,
	}}
	srv := mockBlockchairServer(t, utxos, map[string]string{})
	defer srv.Close()

	handler := handleBuildZECSend(store, blockchair.NewClient(srv.URL))
	req := callToolReq("build_zec_send", map[string]any{
		"to_address": senderAddr,
		"amount":     "100000000",
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
	if tx.Chain != "Zcash" {
		t.Errorf("chain: got %s, want Zcash", tx.Chain)
	}
	if tx.Action != "transfer" {
		t.Errorf("action: got %s, want transfer", tx.Action)
	}
	if tx.SigningMode != "ecdsa_secp256k1" {
		t.Errorf("signing_mode: got %s, want ecdsa_secp256k1", tx.SigningMode)
	}
	if tx.TxDetails["tx_encoding"] != "zcash_v4" {
		t.Errorf("tx_encoding: got %s, want zcash_v4", tx.TxDetails["tx_encoding"])
	}
	if tx.TxDetails["ticker"] != "ZEC" {
		t.Errorf("ticker: got %s, want ZEC", tx.TxDetails["ticker"])
	}
	if tx.TxDetails["fee"] == "" || tx.TxDetails["fee"] == "0" {
		t.Error("fee should be non-zero")
	}
	rawBytes, err := hex.DecodeString(tx.UnsignedTxHex)
	if err != nil {
		t.Fatalf("invalid hex: %v", err)
	}
	if len(rawBytes) == 0 {
		t.Error("transaction bytes should not be empty")
	}
}

func TestBuildZECSend_WithMemo(t *testing.T) {
	store := setupBTCVault(t)
	senderAddr := deriveChainAddr(t, common.Zcash)

	utxos := []blockchair.UTXO{{
		TransactionHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Index:           0, Value: 1_000_000_000, BlockID: 800000,
	}}
	srv := mockBlockchairServer(t, utxos, map[string]string{})
	defer srv.Close()

	handler := handleBuildZECSend(store, blockchair.NewClient(srv.URL))
	req := callToolReq("build_zec_send", map[string]any{
		"to_address": senderAddr,
		"amount":     "200000000",
		"memo":       "=:DASH.DASH:XfakeAddress",
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected success: %s", resultText(t, res))
	}

	text := resultText(t, res)
	var txResult struct {
		Transactions []struct {
			Action string `json:"action"`
		} `json:"transactions"`
	}
	err = json.Unmarshal([]byte(text), &txResult)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if txResult.Transactions[0].Action != "swap" {
		t.Errorf("action: got %s, want swap", txResult.Transactions[0].Action)
	}
}

func TestBuildZECSend_InsufficientFunds(t *testing.T) {
	store := setupBTCVault(t)

	utxos := []blockchair.UTXO{{
		TransactionHash: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		Index:           0, Value: 1000, BlockID: 800000,
	}}
	srv := mockBlockchairServer(t, utxos, map[string]string{})
	defer srv.Close()

	handler := handleBuildZECSend(store, blockchair.NewClient(srv.URL))
	req := callToolReq("build_zec_send", map[string]any{
		"to_address": deriveChainAddr(t, common.Zcash),
		"amount":     "100000000",
	})

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error for insufficient funds")
	}
}

func TestZecEstimateFee(t *testing.T) {
	p2pkhScript := p2pkhScript(make([]byte, 20))

	twoOutputs := []*zcashsdk.TxOutput{
		{Value: 100000, Script: p2pkhScript},
		{Value: 0, Script: p2pkhScript},
	}

	// 1 input, 2 outputs → grace_actions=2 → fee = 5000 * 2 = 10000
	fee := zecEstimateFee(1, twoOutputs)
	if fee != 10000 {
		t.Errorf("1 input 2 outputs: got %d, want 10000", fee)
	}

	// 3 inputs, 2 outputs → input_actions=3 > grace_actions=2 → fee = 5000 * 3 = 15000
	fee = zecEstimateFee(3, twoOutputs)
	if fee != 15000 {
		t.Errorf("3 inputs 2 outputs: got %d, want 15000", fee)
	}

	// 1 input, 2 outputs → still within grace, same as 1 input
	fee = zecEstimateFee(2, twoOutputs)
	if fee != 10000 {
		t.Errorf("2 inputs 2 outputs: got %d, want 10000", fee)
	}
}
