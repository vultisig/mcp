package tron

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"
)

func TestValidateAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr string
	}{
		{"valid USDT contract", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", ""},
		{"valid address", "TJRabPrwbZy45sbavfcjinPJC18kjpRTv8", ""},
		{"empty", "", "empty address"},
		{"wrong prefix", "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "must start with 'T'"},
		{"too short", "TJRabPr", "must be 34 characters"},
		{"too long", "TJRabPrwbZy45sbavfcjinPJC18kjpRTv8X", "must be 34 characters"},
		{"invalid base58", "T000000000000000000000000000000000", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAddress(tt.addr)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateAddress(%q) = %v, want nil", tt.addr, err)
				}
				return
			}
			if err == nil {
				t.Errorf("ValidateAddress(%q) = nil, want error containing %q", tt.addr, tt.wantErr)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ValidateAddress(%q) = %v, want error containing %q", tt.addr, err, tt.wantErr)
			}
		})
	}
}

func TestAddressToHex(t *testing.T) {
	h, err := AddressToHex("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")
	if err != nil {
		t.Fatalf("AddressToHex() error: %v", err)
	}
	if len(h) != 40 {
		t.Errorf("AddressToHex() hex length = %d, want 40", len(h))
	}

	_, err = AddressToHex("T")
	if err == nil {
		t.Error("AddressToHex(\"T\") should return error")
	}
}

func TestFormatSUN(t *testing.T) {
	tests := []struct {
		name string
		sun  *big.Int
		want string
	}{
		{"zero", big.NewInt(0), "0.000000"},
		{"one TRX", big.NewInt(1_000_000), "1.000000"},
		{"fractional", big.NewInt(1_500_000), "1.500000"},
		{"small amount", big.NewInt(1), "0.000001"},
		{"large amount", big.NewInt(90_000_000_000_000_000), "90000000000.000000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSUN(tt.sun)
			if got != tt.want {
				t.Errorf("FormatSUN(%s) = %q, want %q", tt.sun, got, tt.want)
			}
		})
	}
}

func TestDecodeTRC20Balance(t *testing.T) {
	tests := []struct {
		name    string
		hexData string
		want    int64
		wantErr bool
	}{
		{
			"32-byte zero",
			strings.Repeat("00", 32),
			0,
			false,
		},
		{
			"32-byte value 1000",
			strings.Repeat("00", 30) + "03e8",
			1000,
			false,
		},
		{
			"short data padded",
			"03e8",
			1000,
			false,
		},
		{
			"invalid hex",
			"zzzz",
			0,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeTRC20Balance(tt.hexData)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Int64() != tt.want {
				t.Errorf("DecodeTRC20Balance(%q) = %d, want %d", tt.hexData, got.Int64(), tt.want)
			}
		})
	}
}

func TestDecodeTRC20Decimals(t *testing.T) {
	hexData := strings.Repeat("00", 31) + "06"
	got, err := DecodeTRC20Decimals(hexData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 6 {
		t.Errorf("DecodeTRC20Decimals() = %d, want 6", got)
	}

	hexData = strings.Repeat("00", 31) + "12"
	got, err = DecodeTRC20Decimals(hexData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 18 {
		t.Errorf("DecodeTRC20Decimals() = %d, want 18", got)
	}
}

func TestDecodeTRC20Symbol(t *testing.T) {
	t.Run("short data", func(t *testing.T) {
		data := hex.EncodeToString([]byte("USDT\x00\x00"))
		got, err := DecodeTRC20Symbol(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "USDT" {
			t.Errorf("DecodeTRC20Symbol() = %q, want %q", got, "USDT")
		}
	})

	t.Run("ABI encoded", func(t *testing.T) {
		// offset=32, length=4, data="USDT"
		var buf []byte
		offset := make([]byte, 32)
		offset[31] = 0x20 // offset = 32
		buf = append(buf, offset...)

		length := make([]byte, 32)
		length[31] = 0x04 // length = 4
		buf = append(buf, length...)

		symbolBytes := make([]byte, 32)
		copy(symbolBytes, "USDT")
		buf = append(buf, symbolBytes...)

		got, err := DecodeTRC20Symbol(hex.EncodeToString(buf))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "USDT" {
			t.Errorf("DecodeTRC20Symbol() = %q, want %q", got, "USDT")
		}
	})

	t.Run("invalid hex", func(t *testing.T) {
		_, err := DecodeTRC20Symbol("zzzz")
		if err == nil {
			t.Error("expected error for invalid hex")
		}
	})

	t.Run("overflow offset", func(t *testing.T) {
		// offset set to near max uint64 — should return error, not panic
		buf := make([]byte, 64)
		for i := 0; i < 32; i++ {
			buf[i] = 0xff
		}
		_, err := DecodeTRC20Symbol(hex.EncodeToString(buf))
		if err == nil {
			t.Error("expected error for overflow offset")
		}
	})
}

func TestFormatTokenBalance(t *testing.T) {
	tests := []struct {
		name     string
		balance  *big.Int
		decimals uint8
		want     string
	}{
		{"zero decimals", big.NewInt(1000), 0, "1000"},
		{"6 decimals exact", big.NewInt(1_000_000), 6, "1"},
		{"6 decimals fractional", big.NewInt(1_500_000), 6, "1.5"},
		{"18 decimals", new(big.Int).Mul(big.NewInt(15), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil)), 18, "1.5"},
		{"trailing zeros trimmed", big.NewInt(1_100_000), 6, "1.1"},
		{"leading zeros in frac", big.NewInt(1_000_001), 6, "1.000001"},
		{"zero balance", big.NewInt(0), 18, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTokenBalance(tt.balance, tt.decimals)
			if got != tt.want {
				t.Errorf("FormatTokenBalance(%s, %d) = %q, want %q", tt.balance, tt.decimals, got, tt.want)
			}
		})
	}
}
