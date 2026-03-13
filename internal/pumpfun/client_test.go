package pumpfun

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func newTestRPC(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(rpc.New(srv.URL))
}

func buildBondingCurveData(virtualToken, virtualSol, realToken, realSol, totalSupply uint64, complete bool) []byte {
	data := make([]byte, bondingCurveDataLen)
	copy(data[:8], bondingCurveDiscriminator[:])
	binary.LittleEndian.PutUint64(data[8:16], virtualToken)
	binary.LittleEndian.PutUint64(data[16:24], virtualSol)
	binary.LittleEndian.PutUint64(data[24:32], realToken)
	binary.LittleEndian.PutUint64(data[32:40], realSol)
	binary.LittleEndian.PutUint64(data[40:48], totalSupply)
	if complete {
		data[48] = 1
	}
	return data
}

func rpcResponse(data []byte, owner solana.PublicKey) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"context":{"slot":1},"value":{"data":["%s","base64"],"executable":false,"lamports":1000000,"owner":"%s","rentEpoch":0,"space":%d}}}`,
		encoded, owner.String(), len(data))
}

func rpcNullResponse() string {
	return `{"jsonrpc":"2.0","id":1,"result":{"context":{"slot":1},"value":null}}`
}

func TestDeriveBondingCurvePDA(t *testing.T) {
	mint1 := solana.MustPublicKeyFromBase58("So11111111111111111111111111111111111111112")
	mint2 := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v")

	pda1, _, err := DeriveBondingCurvePDA(mint1)
	if err != nil {
		t.Fatalf("derive PDA for mint1: %v", err)
	}
	if pda1.IsZero() {
		t.Fatal("PDA1 is zero")
	}

	pda1Again, _, err := DeriveBondingCurvePDA(mint1)
	if err != nil {
		t.Fatalf("derive PDA for mint1 again: %v", err)
	}
	if pda1 != pda1Again {
		t.Fatal("PDA derivation is not deterministic")
	}

	pda2, _, err := DeriveBondingCurvePDA(mint2)
	if err != nil {
		t.Fatalf("derive PDA for mint2: %v", err)
	}
	if pda1 == pda2 {
		t.Fatal("different mints should produce different PDAs")
	}
}

func TestGetBondingCurveState_Success(t *testing.T) {
	data := buildBondingCurveData(
		1_000_000_000,  // virtual token reserves
		30_000_000_000, // virtual sol reserves
		800_000_000,    // real token reserves
		5_000_000_000,  // real sol reserves
		1_000_000_000,  // total supply
		false,
	)

	client := newTestRPC(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, rpcResponse(data, ProgramID))
	})

	bc := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	state, err := client.GetBondingCurveState(t.Context(), bc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.VirtualTokenReserves != 1_000_000_000 {
		t.Errorf("VirtualTokenReserves = %d, want 1000000000", state.VirtualTokenReserves)
	}
	if state.VirtualSolReserves != 30_000_000_000 {
		t.Errorf("VirtualSolReserves = %d, want 30000000000", state.VirtualSolReserves)
	}
	if state.RealTokenReserves != 800_000_000 {
		t.Errorf("RealTokenReserves = %d, want 800000000", state.RealTokenReserves)
	}
	if state.RealSolReserves != 5_000_000_000 {
		t.Errorf("RealSolReserves = %d, want 5000000000", state.RealSolReserves)
	}
	if state.TokenTotalSupply != 1_000_000_000 {
		t.Errorf("TokenTotalSupply = %d, want 1000000000", state.TokenTotalSupply)
	}
	if state.Complete {
		t.Error("expected Complete = false")
	}
}

func TestGetBondingCurveState_AccountNotFound(t *testing.T) {
	client := newTestRPC(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, rpcNullResponse())
	})

	bc := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	_, err := client.GetBondingCurveState(t.Context(), bc)
	if err == nil {
		t.Fatal("expected error for account not found")
	}
}

func TestGetBondingCurveState_WrongOwner(t *testing.T) {
	data := buildBondingCurveData(1, 1, 1, 1, 1, false)

	client := newTestRPC(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, rpcResponse(data, solana.SystemProgramID))
	})

	bc := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	_, err := client.GetBondingCurveState(t.Context(), bc)
	if err == nil {
		t.Fatal("expected error for wrong owner")
	}
}

func TestGetBondingCurveState_DataTooShort(t *testing.T) {
	shortData := make([]byte, 20)
	copy(shortData[:8], bondingCurveDiscriminator[:])

	client := newTestRPC(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, rpcResponse(shortData, ProgramID))
	})

	bc := solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	_, err := client.GetBondingCurveState(t.Context(), bc)
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestGetTokenInfo_Success(t *testing.T) {
	data := buildBondingCurveData(
		1_000_000_000,
		30_000_000_000,
		800_000_000,
		5_000_000_000,
		1_000_000_000,
		false,
	)

	client := newTestRPC(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, rpcResponse(data, ProgramID))
	})

	mint := solana.NewWallet().PublicKey().String()
	info, err := client.GetTokenInfo(t.Context(), mint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Mint != mint {
		t.Errorf("Mint = %s, want %s", info.Mint, mint)
	}
	if info.Complete {
		t.Error("expected Complete = false")
	}
	if info.GraduationProgress <= 0 {
		t.Errorf("GraduationProgress = %f, want > 0", info.GraduationProgress)
	}
	if info.GraduationProgress > 100 {
		t.Errorf("GraduationProgress = %f, want <= 100", info.GraduationProgress)
	}
	if info.BondingCurveAddress == "" {
		t.Error("BondingCurveAddress should not be empty")
	}
}

func TestGetTokenInfo_InvalidMint(t *testing.T) {
	client := newTestRPC(t, func(w http.ResponseWriter, r *http.Request) {})

	_, err := client.GetTokenInfo(t.Context(), "not-a-valid-key!!!")
	if err == nil {
		t.Fatal("expected error for invalid mint")
	}
}

func TestGetTokenInfo_GraduationCap(t *testing.T) {
	data := buildBondingCurveData(
		1_000_000_000,
		30_000_000_000,
		800_000_000,
		100_000_000_000, // > 85 SOL threshold
		1_000_000_000,
		true,
	)

	client := newTestRPC(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, rpcResponse(data, ProgramID))
	})

	mint := solana.NewWallet().PublicKey().String()
	info, err := client.GetTokenInfo(t.Context(), mint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.GraduationProgress != 100.0 {
		t.Errorf("GraduationProgress = %f, want 100.0", info.GraduationProgress)
	}
	if !info.Complete {
		t.Error("expected Complete = true")
	}
}

func TestBondingCurveDiscriminator(t *testing.T) {
	h := sha256.Sum256([]byte("account:BondingCurve"))
	var want [8]byte
	copy(want[:], h[:8])
	if want != bondingCurveDiscriminator {
		t.Fatalf("discriminator mismatch: computed %x, hardcoded %x", want, bondingCurveDiscriminator)
	}
}
