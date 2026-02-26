package evm

import (
	"testing"
)

func TestEVMChains_AllHaveDefaults(t *testing.T) {
	defaults := DefaultRPCURLs()
	for _, name := range EVMChains {
		url, ok := defaults[name]
		if !ok {
			t.Errorf("chain %q has no default RPC URL", name)
		}
		if url == "" {
			t.Errorf("chain %q has empty default RPC URL", name)
		}
	}
}

func TestEVMChains_AllHaveChainID(t *testing.T) {
	for _, name := range EVMChains {
		id, ok := ChainIDByName(name)
		if !ok {
			t.Errorf("ChainIDByName(%q) not found", name)
			continue
		}
		if id.Sign() <= 0 {
			t.Errorf("ChainIDByName(%q) = %s, want positive", name, id)
		}
	}
}

func TestEVMChains_AllHaveTicker(t *testing.T) {
	for _, name := range EVMChains {
		ticker := NativeTicker(name)
		if ticker == "" {
			t.Errorf("NativeTicker(%q) is empty", name)
		}
	}
}

func TestChainIDByName_KnownChains(t *testing.T) {
	tests := []struct {
		chain  string
		wantID int64
	}{
		{"Ethereum", 1},
		{"BSC", 56},
		{"Polygon", 137},
		{"Avalanche", 43114},
		{"Arbitrum", 42161},
		{"Optimism", 10},
		{"Base", 8453},
		{"Blast", 81457},
		{"Mantle", 5000},
		{"Zksync", 324},
	}

	for _, tt := range tests {
		t.Run(tt.chain, func(t *testing.T) {
			id, ok := ChainIDByName(tt.chain)
			if !ok {
				t.Fatalf("ChainIDByName(%q) not found", tt.chain)
			}
			if id.Int64() != tt.wantID {
				t.Errorf("got chain ID %d, want %d", id.Int64(), tt.wantID)
			}
		})
	}
}

func TestChainIDByName_UnknownChain(t *testing.T) {
	_, ok := ChainIDByName("Unknown")
	if ok {
		t.Error("expected false for unknown chain name")
	}
}

func TestNativeTicker_KnownChains(t *testing.T) {
	tests := []struct {
		chain  string
		ticker string
	}{
		{"Ethereum", "ETH"},
		{"BSC", "BNB"},
		{"Polygon", "POL"},
		{"Avalanche", "AVAX"},
		{"Arbitrum", "ETH"},
		{"Optimism", "ETH"},
		{"Base", "ETH"},
		{"Mantle", "MNT"},
	}

	for _, tt := range tests {
		t.Run(tt.chain, func(t *testing.T) {
			got := NativeTicker(tt.chain)
			if got != tt.ticker {
				t.Errorf("NativeTicker(%q) = %q, want %q", tt.chain, got, tt.ticker)
			}
		})
	}
}

func TestNativeTicker_UnknownChain(t *testing.T) {
	got := NativeTicker("Unknown")
	if got != "ETH" {
		t.Errorf("NativeTicker unknown chain: got %q, want %q (fallback)", got, "ETH")
	}
}

func TestPool_UnknownChain(t *testing.T) {
	pool := NewPool(map[string]string{})
	_, _, err := pool.Get(nil, "Ethereum") //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for chain with no URL, got nil")
	}
}

func TestPool_EmptyURL(t *testing.T) {
	pool := NewPool(map[string]string{"Ethereum": ""})
	_, _, err := pool.Get(nil, "Ethereum") //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
}
