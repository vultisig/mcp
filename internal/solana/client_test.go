package solana

import (
	"context"
	"math"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

func TestFormatLamports(t *testing.T) {
	tests := []struct {
		name     string
		lamports uint64
		want     string
	}{
		{"zero", 0, "0"},
		{"one_lamport", 1, "0.000000001"},
		{"one_sol", 1_000_000_000, "1"},
		{"one_and_half_sol", 1_500_000_000, "1.5"},
		{"fractional", 123_456_789, "0.123456789"},
		{"large", 18_446_744_073, "18.446744073"},
		{"trailing_zeros_trimmed", 1_100_000_000, "1.1"},
		{"max_uint64", math.MaxUint64, "18446744073.709551615"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatLamports(tt.lamports)
			if got != tt.want {
				t.Errorf("FormatLamports(%d) = %q, want %q", tt.lamports, got, tt.want)
			}
		})
	}
}

func TestParsePublicKey(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		pk, err := ParsePublicKey("11111111111111111111111111111111")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pk != solana.SystemProgramID {
			t.Errorf("got %s, want %s", pk, solana.SystemProgramID)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := ParsePublicKey("not-a-valid-key!!!")
		if err == nil {
			t.Fatal("expected error for invalid key")
		}
	})
}

func TestGetTokenProgram_NativeMint(t *testing.T) {
	client := NewClient(rpc.New("https://localhost:0"))
	ctx := context.Background()

	pubkey, decimals, err := client.GetTokenProgram(ctx, solana.SolMint)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pubkey != solana.TokenProgramID {
		t.Errorf("expected TokenProgramID, got %s", pubkey)
	}
	if decimals != 9 {
		t.Errorf("expected 9 decimals, got %d", decimals)
	}
}

func TestFindAssociatedTokenAddress(t *testing.T) {
	wallet := solana.MustPublicKeyFromBase58("7nYhDeFWriouc5PhCH98WCxocNPKfXjJqeFJo59DMKSA")
	mint := solana.MustPublicKeyFromBase58("EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v") // USDC
	tokenProgram := solana.TokenProgramID

	ata1, _, err := FindAssociatedTokenAddress(wallet, mint, tokenProgram)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ata2, _, err := FindAssociatedTokenAddress(wallet, mint, tokenProgram)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ata1 != ata2 {
		t.Errorf("non-deterministic: %s != %s", ata1, ata2)
	}

	if ata1.IsZero() {
		t.Error("ATA should not be zero")
	}
}
