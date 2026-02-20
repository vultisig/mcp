package ethereum

import (
	"encoding/binary"
	"math/big"
	"testing"
)

func TestFormatWei(t *testing.T) {
	tests := []struct {
		name     string
		wei      *big.Int
		decimals int
		want     string
	}{
		{
			name:     "zero",
			wei:      big.NewInt(0),
			decimals: 18,
			want:     "0",
		},
		{
			name:     "one ether",
			wei:      new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
			decimals: 18,
			want:     "1",
		},
		{
			name:     "1.5 ether",
			wei:      new(big.Int).Mul(big.NewInt(15), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil)),
			decimals: 18,
			want:     "1.5",
		},
		{
			name:     "small fraction",
			wei:      big.NewInt(1),
			decimals: 18,
			want:     "0.000000000000000001",
		},
		{
			name:     "usdc 6 decimals",
			wei:      big.NewInt(1_500_000),
			decimals: 6,
			want:     "1.5",
		},
		{
			name:     "large amount",
			wei:      new(big.Int).Mul(big.NewInt(123456), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
			decimals: 18,
			want:     "123456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatUnits(tt.wei, tt.decimals)
			if got != tt.want {
				t.Errorf("FormatUnits(%s, %d) = %q, want %q", tt.wei, tt.decimals, got, tt.want)
			}
		})
	}
}

func TestDecodeABIString(t *testing.T) {
	t.Run("standard ABI encoding", func(t *testing.T) {
		// Encode "USDC" as standard ABI string
		data := make([]byte, 96)
		// offset = 32
		data[31] = 0x20
		// length = 4
		data[63] = 0x04
		// "USDC"
		copy(data[64:], "USDC")

		got, err := DecodeABIString(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "USDC" {
			t.Errorf("got %q, want %q", got, "USDC")
		}
	})

	t.Run("bytes32 encoding", func(t *testing.T) {
		// "MKR" left-aligned, right-padded with zeros (non-standard)
		data := make([]byte, 32)
		copy(data, "MKR")

		got, err := DecodeABIString(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "MKR" {
			t.Errorf("got %q, want %q", got, "MKR")
		}
	})

	t.Run("too short", func(t *testing.T) {
		_, err := DecodeABIString([]byte{0x01, 0x02})
		if err == nil {
			t.Fatal("expected error for short data")
		}
	})

	t.Run("longer standard string", func(t *testing.T) {
		name := "Wrapped Ether"
		data := make([]byte, 128)
		// offset = 32
		binary.BigEndian.PutUint64(data[24:32], 32)
		// length
		binary.BigEndian.PutUint64(data[56:64], uint64(len(name)))
		// data
		copy(data[64:], name)

		got, err := DecodeABIString(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != name {
			t.Errorf("got %q, want %q", got, name)
		}
	})
}
