package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/vultisig/mcp/internal/jupiter"
	solanaclient "github.com/vultisig/mcp/internal/solana"
	"github.com/vultisig/mcp/internal/vault"
	sdk "github.com/vultisig/recipes/sdk"
	solanasdk "github.com/vultisig/recipes/sdk/solana"
)

func TestBuildSolanaTx_InvalidAmount(t *testing.T) {
	store := vault.NewStore()
	handler := handleBuildSolanaTx(store, solanaclient.NewClient(rpc.New("https://localhost:0")))
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
	handler := handleBuildSolanaTx(store, solanaclient.NewClient(rpc.New("https://localhost:0")))
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
	handler := handleBuildSolanaTx(store, solanaclient.NewClient(rpc.New("https://localhost:0")))
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

func newMockSolanaRPC(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			ID     any    `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "getAccountInfo":
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]any{
					"context": map[string]any{"slot": 1},
					"value": map[string]any{
						"data":       []string{"", "base64"},
						"executable": false,
						"lamports":   1000000,
						"owner":      "11111111111111111111111111111111",
						"rentEpoch":  0,
					},
				},
			})
		default:
			json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  nil,
			})
		}
	}))
}

func TestBuildSolanaTx_VaultDerived(t *testing.T) {
	store := vault.NewStore()
	store.Set("default", vault.Info{
		ECDSAPublicKey: testECDSAPubKey,
		EdDSAPublicKey: testEdDSAPubKey,
		ChainCode:      testChainCode,
	})

	srv := newMockSolanaRPC(t)
	defer srv.Close()
	handler := handleBuildSolanaTx(store, solanaclient.NewClient(rpc.New(srv.URL)))
	ctx := context.Background()

	req := callToolReq("build_solana_tx", map[string]any{
		"to":     "11111111111111111111111111111111",
		"amount": "1000000",
	})

	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}

	if res.IsError {
		t.Fatalf("unexpected tool error: %v", res.Content)
	}

	text := resultText(t, res)
	var result struct {
		Chain       string `json:"chain"`
		Action      string `json:"action"`
		SigningMode string `json:"signing_mode"`
	}
	err = json.Unmarshal([]byte(text), &result)
	if err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Chain != "Solana" {
		t.Errorf("chain = %q, want %q", result.Chain, "Solana")
	}
	if result.SigningMode != "eddsa_ed25519" {
		t.Errorf("signing_mode = %q, want %q", result.SigningMode, "eddsa_ed25519")
	}
}

func TestBuildSolanaTx_Integration(t *testing.T) {
	skipUnlessSolanaTest(t)

	from := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	to := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")

	client := solanaclient.NewClient(rpc.New("https://api.mainnet-beta.solana.com"))

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

func TestBuildSolanaTx_SDKCompatibility(t *testing.T) {
	from := solana.MustPublicKeyFromBase58("7nYhDeFWriouc5PhCH98WCxocNPKfXjJqeFJo59DMKSA")
	to := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")

	transferInst := system.NewTransferInstruction(1_000_000, from, to).Build()

	dummyBlockhash := solana.MustHashFromBase58("11111111111111111111111111111111")
	tx, err := solana.NewTransaction(
		[]solana.Instruction{transferInst},
		dummyBlockhash,
		solana.TransactionPayer(from),
	)
	if err != nil {
		t.Fatalf("create transaction: %v", err)
	}

	txBytes, err := tx.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal transaction: %v", err)
	}

	verifySolanaSDKCompat(t, txBytes, 1)
}

func TestBuildSPLTransferTx_SDKCompatibility(t *testing.T) {
	from := solana.MustPublicKeyFromBase58("7nYhDeFWriouc5PhCH98WCxocNPKfXjJqeFJo59DMKSA")
	to := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	mint := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")
	tokenProgram := solana.TokenProgramID

	sourceATA, _, err := solanaclient.FindAssociatedTokenAddress(from, mint, tokenProgram)
	if err != nil {
		t.Fatalf("find source ATA: %v", err)
	}
	destATA, _, err := solanaclient.FindAssociatedTokenAddress(to, mint, tokenProgram)
	if err != nil {
		t.Fatalf("find dest ATA: %v", err)
	}

	var instructions []solana.Instruction

	createATAInst := solana.NewInstruction(
		solana.SPLAssociatedTokenAccountProgramID,
		[]*solana.AccountMeta{
			{PublicKey: from, IsSigner: true, IsWritable: true},
			{PublicKey: destATA, IsSigner: false, IsWritable: true},
			{PublicKey: to, IsSigner: false, IsWritable: false},
			{PublicKey: mint, IsSigner: false, IsWritable: false},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
			{PublicKey: tokenProgram, IsSigner: false, IsWritable: false},
		},
		[]byte{0},
	)
	instructions = append(instructions, createATAInst)

	transferData := make([]byte, 10)
	transferData[0] = 12
	binary.LittleEndian.PutUint64(transferData[1:9], 1_000_000)
	transferData[9] = 6

	transferInst := solana.NewInstruction(
		tokenProgram,
		[]*solana.AccountMeta{
			{PublicKey: sourceATA, IsSigner: false, IsWritable: true},
			{PublicKey: mint, IsSigner: false, IsWritable: false},
			{PublicKey: destATA, IsSigner: false, IsWritable: true},
			{PublicKey: from, IsSigner: true, IsWritable: false},
		},
		transferData,
	)
	instructions = append(instructions, transferInst)

	dummyBlockhash := solana.MustHashFromBase58("11111111111111111111111111111111")
	tx, err := solana.NewTransaction(
		instructions,
		dummyBlockhash,
		solana.TransactionPayer(from),
	)
	if err != nil {
		t.Fatalf("create transaction: %v", err)
	}

	txBytes, err := tx.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal transaction: %v", err)
	}

	verifySolanaSDKCompat(t, txBytes, 2)
}

func verifySolanaSDKCompat(t *testing.T, txBytes []byte, wantInstructions int) {
	t.Helper()

	solSDK := solanasdk.NewSDK(nil)
	hashes, err := solSDK.DeriveSigningHashes(txBytes, sdk.DeriveOptions{})
	if err != nil {
		t.Fatalf("DeriveSigningHashes: %v", err)
	}

	if len(hashes) != 1 {
		t.Fatalf("expected 1 derived hash, got %d", len(hashes))
	}

	dh := hashes[0]
	if len(dh.Message) == 0 {
		t.Fatal("message is empty")
	}
	if len(dh.Hash) != 32 {
		t.Fatalf("expected 32-byte hash, got %d", len(dh.Hash))
	}

	wantHash := sha256.Sum256(dh.Message)
	if !bytes.Equal(dh.Hash, wantHash[:]) {
		t.Error("Hash != SHA256(Message)")
	}

	parsedTx, err := solana.TransactionFromBytes(txBytes)
	if err != nil {
		t.Fatalf("TransactionFromBytes: %v", err)
	}
	if parsedTx.Message.Header.NumRequiredSignatures != 1 {
		t.Errorf("expected 1 required signature, got %d", parsedTx.Message.Header.NumRequiredSignatures)
	}
	if len(parsedTx.Message.Instructions) != wantInstructions {
		t.Errorf("expected %d instructions, got %d", wantInstructions, len(parsedTx.Message.Instructions))
	}

	hexStr := hex.EncodeToString(txBytes)
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("hex round-trip: %v", err)
	}
	if !bytes.Equal(decoded, txBytes) {
		t.Error("hex round-trip mismatch")
	}
}

func TestBuildSPLTransferTx_AcceptsWSOL(t *testing.T) {
	store := vault.NewStore()
	handler := handleBuildSPLTransferTx(store, solanaclient.NewClient(rpc.New("https://localhost:0")))
	ctx := context.Background()

	req := callToolReq("build_spl_transfer_tx", map[string]any{
		"from":   "7nYhDeFWriouc5PhCH98WCxocNPKfXjJqeFJo59DMKSA",
		"to":     "11111111111111111111111111111111",
		"mint":   "So11111111111111111111111111111111111111112",
		"amount": "1000000",
	})

	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %v", res.Content)
	}

	text := resultText(t, res)
	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result["action"] != "spl_transfer" {
		t.Errorf("expected action spl_transfer, got %v", result["action"])
	}
	if result["mint"] != "So11111111111111111111111111111111111111112" {
		t.Errorf("unexpected mint: %v", result["mint"])
	}
}

func TestBuildSolanaSwap_MissingParams(t *testing.T) {
	store := vault.NewStore()
	jupClient := newMockJupiterClient(t)
	handler := handleBuildSolanaSwap(store, jupClient)
	ctx := context.Background()

	tests := []struct {
		name string
		args map[string]any
	}{
		{"missing_output_mint", map[string]any{
			"from":   "7nYhDeFWriouc5PhCH98WCxocNPKfXjJqeFJo59DMKSA",
			"amount": "1000000",
		}},
		{"missing_amount", map[string]any{
			"from":        "7nYhDeFWriouc5PhCH98WCxocNPKfXjJqeFJo59DMKSA",
			"output_mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		}},
		{"missing_from_no_vault", map[string]any{
			"output_mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			"amount":      "1000000",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := callToolReq("build_solana_swap", tt.args)
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

func TestBuildSolanaSwap_InvalidAmount(t *testing.T) {
	store := vault.NewStore()
	jupClient := newMockJupiterClient(t)
	handler := handleBuildSolanaSwap(store, jupClient)
	ctx := context.Background()

	tests := []struct {
		name   string
		amount string
	}{
		{"not_a_number", "abc"},
		{"negative", "-100"},
		{"zero", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := callToolReq("build_solana_swap", map[string]any{
				"from":        "7nYhDeFWriouc5PhCH98WCxocNPKfXjJqeFJo59DMKSA",
				"output_mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				"amount":      tt.amount,
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

func TestBuildSolanaSwap_DefaultSlippageAndInputMint(t *testing.T) {
	store := vault.NewStore()
	jupSrv, rpcSrv := newMockJupiterServers(t)
	defer jupSrv.Close()
	defer rpcSrv.Close()

	jupClient := jupiter.NewClient(jupSrv.URL, rpc.New(rpcSrv.URL))
	handler := handleBuildSolanaSwap(store, jupClient)
	ctx := context.Background()

	req := callToolReq("build_solana_swap", map[string]any{
		"from":        "7nYhDeFWriouc5PhCH98WCxocNPKfXjJqeFJo59DMKSA",
		"output_mint": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		"amount":      "1000000",
	})

	res, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if res.IsError {
		tc, ok := res.Content[0].(mcp.TextContent)
		if ok {
			t.Fatalf("tool returned error: %s", tc.Text)
		}
		t.Fatal("tool returned error")
	}

	text := resultText(t, res)
	var result struct {
		Chain       string `json:"chain"`
		Action      string `json:"action"`
		SigningMode string `json:"signing_mode"`
		InputMint   string `json:"input_mint"`
	}
	err = json.Unmarshal([]byte(text), &result)
	if err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.Chain != "Solana" {
		t.Errorf("chain = %q, want Solana", result.Chain)
	}
	if result.Action != "swap" {
		t.Errorf("action = %q, want swap", result.Action)
	}
	if result.SigningMode != "eddsa_ed25519" {
		t.Errorf("signing_mode = %q, want %q", result.SigningMode, "eddsa_ed25519")
	}
	if result.InputMint != solana.SolMint.String() {
		t.Errorf("input_mint = %q, want SOL mint (default)", result.InputMint)
	}
}

func newMockJupiterClient(t *testing.T) *jupiter.Client {
	t.Helper()
	jupSrv, rpcSrv := newMockJupiterServers(t)
	t.Cleanup(jupSrv.Close)
	t.Cleanup(rpcSrv.Close)
	return jupiter.NewClient(jupSrv.URL, rpc.New(rpcSrv.URL))
}

func newMockJupiterServers(t *testing.T) (*httptest.Server, *httptest.Server) {
	t.Helper()

	jupSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/swap/v1/quote":
			json.NewEncoder(w).Encode(jupiter.QuoteResponse{
				InputMint:            solana.SolMint.String(),
				InAmount:             "1000000",
				OutputMint:           "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				OutAmount:            "500",
				OtherAmountThreshold: "495",
				SlippageBps:          100,
				PriceImpactPct:       "0.01",
			})
		case "/swap/v1/swap-instructions":
			json.NewEncoder(w).Encode(jupiter.SwapInstructionsResponse{
				ComputeBudgetInstructions: []jupiter.InstructionData{},
				SetupInstructions:         []jupiter.InstructionData{},
				SwapInstruction: jupiter.InstructionData{
					ProgramId: solana.SystemProgramID.String(),
					Accounts: []jupiter.Account{
						{Pubkey: "7nYhDeFWriouc5PhCH98WCxocNPKfXjJqeFJo59DMKSA", IsSigner: true, IsWritable: true},
					},
					Data: "AQAAAA==",
				},
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))

	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"context":{"slot":1},"value":{"blockhash":"11111111111111111111111111111111","lastValidBlockHeight":100}}}`))
	}))

	return jupSrv, rpcSrv
}

func skipUnlessSolanaTest(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping Solana integration test in short mode")
	}
}
