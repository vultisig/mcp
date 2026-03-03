package blockchair

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const dashboardCacheTTL = 5 * time.Minute

// ChainInfo describes a supported UTXO chain.
type ChainInfo struct {
	Slug     string // Blockchair URL slug (e.g. "bitcoin")
	Ticker   string // Currency ticker (e.g. "BTC")
	Decimals int    // Satoshi-like decimals (always 8 for supported chains)
}

// SupportedChains maps Vultisig chain names to Blockchair chain metadata.
var SupportedChains = map[string]ChainInfo{
	"Bitcoin":      {Slug: "bitcoin", Ticker: "BTC", Decimals: 8},
	"Bitcoin-Cash": {Slug: "bitcoin-cash", Ticker: "BCH", Decimals: 8},
	"Litecoin":     {Slug: "litecoin", Ticker: "LTC", Decimals: 8},
	"Dogecoin":     {Slug: "dogecoin", Ticker: "DOGE", Decimals: 8},
	"Dash":         {Slug: "dash", Ticker: "DASH", Decimals: 8},
	"Zcash":        {Slug: "zcash", Ticker: "ZEC", Decimals: 8},
}

// ChainNames returns the sorted list of supported chain names for use in tool enums.
var ChainNames = []string{
	"Bitcoin",
	"Bitcoin-Cash",
	"Dash",
	"Dogecoin",
	"Litecoin",
	"Zcash",
}

// AddressInfo holds the address-level stats from the Blockchair dashboard.
type AddressInfo struct {
	Type               string  `json:"type"`
	Balance            int64   `json:"balance"`
	BalanceUSD         float64 `json:"balance_usd"`
	Received           int64   `json:"received"`
	Spent              int64   `json:"spent"`
	OutputCount        int     `json:"output_count"`
	UnspentOutputCount int     `json:"unspent_output_count"`
	TransactionCount   int     `json:"transaction_count"`
}

// UTXO represents a single unspent transaction output.
type UTXO struct {
	BlockID         int64  `json:"block_id"`
	TransactionHash string `json:"transaction_hash"`
	Index           int    `json:"index"`
	Value           int64  `json:"value"`
}

// AddressDashboard contains the full dashboard data for one address.
type AddressDashboard struct {
	Address      AddressInfo `json:"address"`
	Transactions []string    `json:"transactions"`
	UTXOs        []UTXO      `json:"utxo"`
}

// dashboardResponse is the raw JSON envelope from the Blockchair API.
type dashboardResponse struct {
	Data    map[string]AddressDashboard `json:"data"`
	Context struct {
		Code int `json:"code"`
	} `json:"context"`
}

// Client wraps the Vultisig Blockchair proxy with an in-memory TTL cache.
type Client struct {
	http    *http.Client
	baseURL string

	cache      *ttlCache[*AddressDashboard]
	rawTxCache *ttlCache[[]byte]
}

// NewClient creates a Blockchair API client.
func NewClient(baseURL string) *Client {
	return &Client{
		http:       &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		cache:      newTTLCache[*AddressDashboard](dashboardCacheTTL),
		rawTxCache: newTTLCache[[]byte](dashboardCacheTTL),
	}
}

// GetAddressDashboard fetches the address dashboard for a UTXO chain.
// Results are cached for 5 minutes keyed by chain:address.
func (c *Client) GetAddressDashboard(ctx context.Context, chain, address string) (*AddressDashboard, error) {
	info, ok := SupportedChains[chain]
	if !ok {
		return nil, fmt.Errorf("unsupported UTXO chain: %s", chain)
	}

	cacheKey := chain + ":" + address
	if cached, ok := c.cache.get(cacheKey); ok {
		return cached, nil
	}

	url := fmt.Sprintf("%s/%s/dashboards/address/%s", c.baseURL, info.Slug, address)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("blockchair: %s dashboard returned %d", chain, resp.StatusCode)
	}

	var dr dashboardResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return nil, fmt.Errorf("blockchair: decode response: %w", err)
	}

	dashboard, ok := dr.Data[address]
	if !ok {
		return nil, fmt.Errorf("blockchair: no data for address %s", address)
	}

	c.cache.set(cacheKey, &dashboard)
	return &dashboard, nil
}

// GetRawTransaction fetches raw transaction bytes for a given tx hash.
// Follows the pattern from app-recurring/internal/blockchair/client.go.
func (c *Client) GetRawTransaction(ctx context.Context, chain, txHash string) ([]byte, error) {
	info, ok := SupportedChains[chain]
	if !ok {
		return nil, fmt.Errorf("unsupported UTXO chain: %s", chain)
	}

	cacheKey := chain + ":" + txHash
	if cached, ok := c.rawTxCache.get(cacheKey); ok {
		return cached, nil
	}

	url := fmt.Sprintf("%s/%s/raw/transaction/%s", c.baseURL, info.Slug, txHash)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("blockchair: raw tx %s returned %d", txHash, resp.StatusCode)
	}

	var rr rawTxResponse
	err = json.NewDecoder(resp.Body).Decode(&rr)
	if err != nil {
		return nil, fmt.Errorf("blockchair: decode raw tx: %w", err)
	}

	item, ok := rr.Data[txHash]
	if !ok {
		return nil, fmt.Errorf("blockchair: no data for tx %s", txHash)
	}

	raw, err := hex.DecodeString(item.RawTransaction)
	if err != nil {
		return nil, fmt.Errorf("blockchair: decode raw tx hex: %w", err)
	}

	c.rawTxCache.set(cacheKey, raw)
	return raw, nil
}

// ChainFetcherWithCtx returns a PrevTxFetcher adapter for the given chain,
// implementing the btc.PrevTxFetcher interface used by PopulatePSBTMetadata.
// Captures the request context so cancellation propagates to raw tx fetches.
func (c *Client) ChainFetcherWithCtx(ctx context.Context, chain string) *chainFetcherAdapter {
	return &chainFetcherAdapter{client: c, chain: chain, ctx: ctx}
}

type chainFetcherAdapter struct {
	client *Client
	chain  string
	ctx    context.Context
}

func (a *chainFetcherAdapter) GetRawTransaction(txHash string) ([]byte, error) {
	return a.client.GetRawTransaction(a.ctx, a.chain, txHash)
}

// TxDashboard holds the transaction-level dashboard data from Blockchair.
type TxDashboard struct {
	BlockID       int64  `json:"block_id"`
	Fee           int64  `json:"fee"`
	InputTotal    int64  `json:"input_total"`
	OutputTotal   int64  `json:"output_total"`
	Confirmations int    `json:"confirmations,omitempty"`
}

type txDashboardResponse struct {
	Data    map[string]txDashboardData `json:"data"`
	Context struct {
		Code int `json:"code"`
	} `json:"context"`
}

type txDashboardData struct {
	Transaction TxDashboard `json:"transaction"`
}

// GetTxDashboard fetches the transaction dashboard for a UTXO chain tx hash.
// Returns nil, nil if the transaction is not found (404).
func (c *Client) GetTxDashboard(ctx context.Context, chain, txHash string) (*TxDashboard, error) {
	info, ok := SupportedChains[chain]
	if !ok {
		return nil, fmt.Errorf("unsupported UTXO chain: %s", chain)
	}

	url := fmt.Sprintf("%s/%s/dashboards/transaction/%s", c.baseURL, info.Slug, txHash)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("blockchair: tx dashboard returned %d", resp.StatusCode)
	}

	var dr txDashboardResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return nil, fmt.Errorf("blockchair: decode tx dashboard: %w", err)
	}

	data, ok := dr.Data[txHash]
	if !ok {
		return nil, nil
	}

	return &data.Transaction, nil
}

// FormatSatoshis converts a satoshi-like integer to a decimal string.
func FormatSatoshis(amount int64, decimals int) string {
	if decimals == 0 {
		return fmt.Sprintf("%d", amount)
	}
	divisor := int64(1)
	for range decimals {
		divisor *= 10
	}
	whole := amount / divisor
	frac := amount % divisor
	if frac < 0 {
		frac = -frac
	}
	format := fmt.Sprintf("%%d.%%0%dd", decimals)
	return fmt.Sprintf(format, whole, frac)
}

type rawTxResponse struct {
	Data map[string]struct {
		RawTransaction string `json:"raw_transaction"`
	} `json:"data"`
}
