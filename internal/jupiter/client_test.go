package jupiter

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func TestInstructionDataToInstruction(t *testing.T) {
	inst := InstructionData{
		ProgramId: solana.SystemProgramID.String(),
		Accounts: []Account{
			{Pubkey: solana.SystemProgramID.String(), IsSigner: false, IsWritable: false},
			{Pubkey: solana.TokenProgramID.String(), IsSigner: true, IsWritable: true},
		},
		Data: "AQAAAA==", // base64 of [1,0,0,0]
	}

	result, err := inst.ToInstruction()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ProgramID() != solana.SystemProgramID {
		t.Errorf("program id = %s, want %s", result.ProgramID(), solana.SystemProgramID)
	}

	accounts := result.Accounts()
	if len(accounts) != 2 {
		t.Fatalf("accounts len = %d, want 2", len(accounts))
	}

	if accounts[0].PublicKey != solana.SystemProgramID {
		t.Errorf("account[0] = %s, want %s", accounts[0].PublicKey, solana.SystemProgramID)
	}
	if accounts[0].IsSigner {
		t.Error("account[0] should not be signer")
	}
	if accounts[0].IsWritable {
		t.Error("account[0] should not be writable")
	}

	if accounts[1].PublicKey != solana.TokenProgramID {
		t.Errorf("account[1] = %s, want %s", accounts[1].PublicKey, solana.TokenProgramID)
	}
	if !accounts[1].IsSigner {
		t.Error("account[1] should be signer")
	}
	if !accounts[1].IsWritable {
		t.Error("account[1] should be writable")
	}

	data, err := result.Data()
	if err != nil {
		t.Fatalf("unexpected error getting data: %v", err)
	}
	if len(data) != 4 || data[0] != 1 {
		t.Errorf("data = %v, want [1 0 0 0]", data)
	}
}

func TestInstructionDataToInstruction_InvalidProgramID(t *testing.T) {
	inst := InstructionData{
		ProgramId: "not-a-valid-key",
		Accounts:  []Account{},
		Data:      "AQAAAA==",
	}

	_, err := inst.ToInstruction()
	if err == nil {
		t.Fatal("expected error for invalid program id")
	}
}

func TestInstructionDataToInstruction_InvalidBase64(t *testing.T) {
	inst := InstructionData{
		ProgramId: solana.SystemProgramID.String(),
		Accounts:  []Account{},
		Data:      "!!!not-base64!!!",
	}

	_, err := inst.ToInstruction()
	if err == nil {
		t.Fatal("expected error for invalid base64 data")
	}
}

func TestGetQuote_MockServer(t *testing.T) {
	quote := QuoteResponse{
		InputMint:            solana.SolMint.String(),
		InAmount:             "1000000000",
		OutputMint:           "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		OutAmount:            "5000000",
		OtherAmountThreshold: "4950000",
		SwapMode:             "ExactIn",
		SlippageBps:          100,
		PriceImpactPct:       "0.01",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/swap/v1/quote" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		q := r.URL.Query()
		if q.Get("inputMint") != solana.SolMint.String() {
			t.Errorf("unexpected inputMint: %s", q.Get("inputMint"))
		}
		if q.Get("amount") != "1000000000" {
			t.Errorf("unexpected amount: %s", q.Get("amount"))
		}
		if q.Get("restrictIntermediateTokens") != "true" {
			t.Errorf("expected restrictIntermediateTokens=true")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(quote)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, rpc.New("https://localhost:0"))
	ctx := context.Background()

	result, err := client.GetQuote(ctx, solana.SolMint.String(), "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", big.NewInt(1_000_000_000), 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OutAmount != "5000000" {
		t.Errorf("out amount = %s, want 5000000", result.OutAmount)
	}
	if result.PriceImpactPct != "0.01" {
		t.Errorf("price impact = %s, want 0.01", result.PriceImpactPct)
	}
}

func TestGetSwapInstructions_MockServer(t *testing.T) {
	swapResp := SwapInstructionsResponse{
		ComputeBudgetInstructions: []InstructionData{
			{
				ProgramId: solana.SystemProgramID.String(),
				Accounts:  []Account{},
				Data:      "AQAAAA==",
			},
		},
		SetupInstructions: []InstructionData{},
		SwapInstruction: InstructionData{
			ProgramId: solana.SystemProgramID.String(),
			Accounts: []Account{
				{Pubkey: solana.SystemProgramID.String(), IsSigner: false, IsWritable: true},
			},
			Data: "AgAAAA==",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/swap/v1/swap-instructions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected json content type, got %s", r.Header.Get("Content-Type"))
		}

		var body swapRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		if !body.WrapAndUnwrapSol {
			t.Error("expected wrapAndUnwrapSol=true")
		}
		if !body.DynamicComputeUnitLimit {
			t.Error("expected dynamicComputeUnitLimit=true")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(swapResp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, rpc.New("https://localhost:0"))
	ctx := context.Background()

	quote := QuoteResponse{
		InputMint:  solana.SolMint.String(),
		OutputMint: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
		OutAmount:  "5000000",
	}

	result, err := client.GetSwapInstructions(ctx, quote, "7nYhDeFWriouc5PhCH98WCxocNPKfXjJqeFJo59DMKSA")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ComputeBudgetInstructions) != 1 {
		t.Errorf("compute budget instructions = %d, want 1", len(result.ComputeBudgetInstructions))
	}

	swapInst, err := result.SwapInstruction.ToInstruction()
	if err != nil {
		t.Fatalf("failed to parse swap instruction: %v", err)
	}
	if swapInst.ProgramID() != solana.SystemProgramID {
		t.Errorf("swap program = %s, want %s", swapInst.ProgramID(), solana.SystemProgramID)
	}
}

func TestInstructionDataToInstruction_InvalidAccountPubkey(t *testing.T) {
	inst := InstructionData{
		ProgramId: solana.SystemProgramID.String(),
		Accounts: []Account{
			{Pubkey: "not-a-valid-pubkey", IsSigner: false, IsWritable: false},
		},
		Data: "AQAAAA==",
	}

	_, err := inst.ToInstruction()
	if err == nil {
		t.Fatal("expected error for invalid account pubkey")
	}
}

func TestBuildSwapTransaction_RejectsAddressLookupTables(t *testing.T) {
	reqCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/swap/v1/quote":
			json.NewEncoder(w).Encode(QuoteResponse{
				InputMint:            solana.SolMint.String(),
				InAmount:             "1000000",
				OutputMint:           "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
				OutAmount:            "500",
				OtherAmountThreshold: "495",
				SlippageBps:          100,
			})
		case "/swap/v1/swap-instructions":
			json.NewEncoder(w).Encode(SwapInstructionsResponse{
				ComputeBudgetInstructions: []InstructionData{},
				SetupInstructions:         []InstructionData{},
				SwapInstruction: InstructionData{
					ProgramId: solana.SystemProgramID.String(),
					Accounts:  []Account{},
					Data:      "AQAAAA==",
				},
				AddressLookupTableAddresses: []string{"SomeLookupTable111111111111111111111111111111"},
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
		reqCount++
	}))
	defer srv.Close()

	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"context":{"slot":1},"value":{"blockhash":"11111111111111111111111111111111","lastValidBlockHeight":100}}}`))
	}))
	defer rpcSrv.Close()

	client := NewClient(srv.URL, rpc.New(rpcSrv.URL))
	ctx := context.Background()

	_, err := client.BuildSwapTransaction(ctx, "7nYhDeFWriouc5PhCH98WCxocNPKfXjJqeFJo59DMKSA", "", "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", big.NewInt(1_000_000), 100)
	if err == nil {
		t.Fatal("expected error for address lookup tables")
	}
	if reqCount == 0 {
		t.Fatal("expected at least one request to mock server")
	}
}

func TestGetQuote_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"no routes found"}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, rpc.New("https://localhost:0"))
	ctx := context.Background()

	_, err := client.GetQuote(ctx, solana.SolMint.String(), "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v", big.NewInt(1000), 100)
	if err == nil {
		t.Fatal("expected error for bad status")
	}
	t.Log("got expected error:", err)
}
