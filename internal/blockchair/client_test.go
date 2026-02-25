package blockchair

import (
	"context"
	"os"
	"testing"
)

func skipUnlessIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("BLOCKCHAIR_TEST") != "1" {
		t.Skip("set BLOCKCHAIR_TEST=1 to run Blockchair integration tests")
	}
}

func TestGetAddressDashboard(t *testing.T) {
	skipUnlessIntegration(t)

	c := NewClient("https://api.vultisig.com/blockchair")

	// Satoshi's address — always has a balance and UTXOs.
	dashboard, err := c.GetAddressDashboard(context.Background(), "Bitcoin", "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")
	if err != nil {
		t.Fatalf("GetAddressDashboard: %v", err)
	}

	if dashboard.Address.Balance <= 0 {
		t.Errorf("expected positive balance, got %d", dashboard.Address.Balance)
	}
	if dashboard.Address.TransactionCount <= 0 {
		t.Errorf("expected positive transaction count, got %d", dashboard.Address.TransactionCount)
	}
	if len(dashboard.UTXOs) == 0 {
		t.Error("expected at least one UTXO")
	}
	if len(dashboard.Transactions) == 0 {
		t.Error("expected at least one transaction hash")
	}
}

func TestCacheHit(t *testing.T) {
	skipUnlessIntegration(t)

	c := NewClient("https://api.vultisig.com/blockchair")

	addr := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	d1, err := c.GetAddressDashboard(context.Background(), "Bitcoin", addr)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	d2, err := c.GetAddressDashboard(context.Background(), "Bitcoin", addr)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if d1 != d2 {
		t.Error("expected cache hit to return same pointer")
	}
}

func TestGetRawTransaction(t *testing.T) {
	skipUnlessIntegration(t)

	c := NewClient("https://api.vultisig.com/blockchair")

	txHash := "f4184fc596403b9d638783cf57adfe4c75c605f6356fbc91338530e9831e9e16"
	rawBytes, err := c.GetRawTransaction(context.Background(), "Bitcoin", txHash)
	if err != nil {
		t.Fatalf("GetRawTransaction: %v", err)
	}
	if len(rawBytes) == 0 {
		t.Error("expected non-empty raw transaction bytes")
	}
}

func TestChainFetcher(t *testing.T) {
	skipUnlessIntegration(t)

	c := NewClient("https://api.vultisig.com/blockchair")
	fetcher := c.ChainFetcherWithCtx(context.Background(), "Bitcoin")

	txHash := "f4184fc596403b9d638783cf57adfe4c75c605f6356fbc91338530e9831e9e16"
	rawBytes, err := fetcher.GetRawTransaction(txHash)
	if err != nil {
		t.Fatalf("ChainFetcher.GetRawTransaction: %v", err)
	}
	if len(rawBytes) == 0 {
		t.Error("expected non-empty raw transaction bytes")
	}
}
