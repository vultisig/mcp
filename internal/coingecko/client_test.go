package coingecko

import (
	"context"
	"testing"
)

func TestSearch(t *testing.T) {
	c := NewClient()
	coins, err := c.Search(context.Background(), "USDC")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(coins) == 0 {
		t.Fatal("expected at least one result")
	}

	first := coins[0]
	if first.ID != "usd-coin" {
		t.Errorf("first result ID = %q, want usd-coin", first.ID)
	}
	if first.Symbol != "USDC" {
		t.Errorf("first result symbol = %q, want USDC", first.Symbol)
	}
	if first.MarketCapRank == 0 {
		t.Error("expected non-zero market_cap_rank for USDC")
	}
	if first.Large == "" {
		t.Error("expected non-empty large image URL")
	}
}

func TestCoinDetail_Token(t *testing.T) {
	c := NewClient()
	detail, err := c.CoinDetail(context.Background(), "usd-coin")
	if err != nil {
		t.Fatalf("CoinDetail: %v", err)
	}

	if detail.ID != "usd-coin" {
		t.Errorf("ID = %q, want usd-coin", detail.ID)
	}
	if detail.Image.Large == "" {
		t.Error("expected non-empty large image")
	}

	eth, ok := detail.DetailPlatforms["ethereum"]
	if !ok {
		t.Fatal("expected ethereum in detail_platforms")
	}
	if eth.ContractAddress == "" {
		t.Error("expected non-empty contract address for ethereum")
	}
	if eth.DecimalPlace == nil || *eth.DecimalPlace != 6 {
		t.Errorf("expected decimals=6 for USDC on ethereum, got %v", eth.DecimalPlace)
	}
}

func TestCoinDetail_NativeAsset(t *testing.T) {
	c := NewClient()
	detail, err := c.CoinDetail(context.Background(), "bitcoin")
	if err != nil {
		t.Fatalf("CoinDetail: %v", err)
	}

	if detail.ID != "bitcoin" {
		t.Errorf("ID = %q, want bitcoin", detail.ID)
	}

	// Bitcoin has a single "" platform with empty contract.
	for platform, pd := range detail.DetailPlatforms {
		if platform != "" {
			continue
		}
		if pd.ContractAddress != "" {
			t.Errorf("expected empty contract for native asset, got %q", pd.ContractAddress)
		}
	}
}

func TestSearch_Cache(t *testing.T) {
	c := NewClient()
	ctx := context.Background()

	// First call populates cache.
	coins1, err := c.Search(ctx, "ETH")
	if err != nil {
		t.Fatalf("first Search: %v", err)
	}

	// Second call should return cached data.
	coins2, err := c.Search(ctx, "ETH")
	if err != nil {
		t.Fatalf("second Search: %v", err)
	}

	if len(coins1) != len(coins2) {
		t.Errorf("cache returned different length: %d vs %d", len(coins1), len(coins2))
	}
}

func TestCoinDetail_Cache(t *testing.T) {
	c := NewClient()
	ctx := context.Background()

	d1, err := c.CoinDetail(ctx, "ethereum")
	if err != nil {
		t.Fatalf("first CoinDetail: %v", err)
	}

	d2, err := c.CoinDetail(ctx, "ethereum")
	if err != nil {
		t.Fatalf("second CoinDetail: %v", err)
	}

	// Same pointer means cache hit.
	if d1 != d2 {
		t.Error("expected same pointer from cache")
	}
}
