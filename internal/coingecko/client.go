package coingecko

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultBaseURL = "https://api.vultisig.com/coingeicko/api/v3"

	// searchCacheTTL controls how long search results are reused.
	searchCacheTTL = 5 * time.Minute

	// detailCacheTTL controls how long coin detail (contract addresses,
	// decimals, images) is reused. This data changes very rarely.
	detailCacheTTL = 10 * time.Minute
)

// Client wraps the CoinGecko REST API (via Vultisig proxy) with an in-memory TTL cache.
type Client struct {
	http    *http.Client
	baseURL string

	searchCache *ttlCache[[]SearchCoin]
	detailCache *ttlCache[*CoinDetail]
}

// NewClient creates a CoinGecko API client that routes through the Vultisig proxy.
func NewClient() *Client {
	return &Client{
		http:        &http.Client{Timeout: 30 * time.Second},
		baseURL:     defaultBaseURL,
		searchCache: newTTLCache[[]SearchCoin](searchCacheTTL),
		detailCache: newTTLCache[*CoinDetail](detailCacheTTL),
	}
}

func (c *Client) doGet(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.http.Do(req)
}

// --- Search ----------------------------------------------------------------

// SearchCoin is a single coin from the /search endpoint.
type SearchCoin struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	MarketCapRank int    `json:"market_cap_rank"`
	Thumb         string `json:"thumb"`
	Large         string `json:"large"`
}

type searchResponse struct {
	Coins []SearchCoin `json:"coins"`
}

// Search queries the CoinGecko /search endpoint. Results are cached for 5 minutes.
func (c *Client) Search(ctx context.Context, query string) ([]SearchCoin, error) {
	if cached, ok := c.searchCache.get(query); ok {
		return cached, nil
	}

	resp, err := c.doGet(ctx, "/search?query="+url.QueryEscape(query))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coingecko: search returned %d", resp.StatusCode)
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	c.searchCache.set(query, sr.Coins)
	return sr.Coins, nil
}

// --- Coin detail -----------------------------------------------------------

// CoinDetail is the subset of /coins/{id} we need.
type CoinDetail struct {
	ID              string                    `json:"id"`
	Symbol          string                    `json:"symbol"`
	Name            string                    `json:"name"`
	Image           CoinImage                 `json:"image"`
	MarketCapRank   int                       `json:"market_cap_rank"`
	DetailPlatforms map[string]PlatformDetail `json:"detail_platforms"`
}

// CoinImage holds the three sizes CoinGecko returns.
type CoinImage struct {
	Thumb string `json:"thumb"`
	Small string `json:"small"`
	Large string `json:"large"`
}

// PlatformDetail contains the contract address and decimals for one chain.
type PlatformDetail struct {
	DecimalPlace    *int   `json:"decimal_place"`
	ContractAddress string `json:"contract_address"`
}

// CoinDetail fetches /coins/{id} with only the fields we need (no market data,
// tickers, community or developer data). Results are cached for 10 minutes.
func (c *Client) CoinDetail(ctx context.Context, id string) (*CoinDetail, error) {
	if cached, ok := c.detailCache.get(id); ok {
		return cached, nil
	}

	path := fmt.Sprintf("/coins/%s?localization=false&tickers=false&market_data=false&community_data=false&developer_data=false",
		url.PathEscape(id))

	resp, err := c.doGet(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coingecko: coin detail for %q returned %d", id, resp.StatusCode)
	}

	var cd CoinDetail
	if err := json.NewDecoder(resp.Body).Decode(&cd); err != nil {
		return nil, err
	}

	c.detailCache.set(id, &cd)
	return &cd, nil
}
