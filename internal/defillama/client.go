package defillama

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultBaseURL   = "https://api.llama.fi"
	yieldBaseURL     = "https://yields.llama.fi"
	protocolCacheTTL = 5 * time.Minute
	poolsCacheTTL    = 10 * time.Minute
	chainsCacheTTL   = 5 * time.Minute
)

// Client wraps the DeFiLlama REST API with an in-memory TTL cache.
type Client struct {
	http     *http.Client
	baseURL  string
	yieldURL string

	protocolCache *ttlCache[*Protocol]
	poolsCache    *ttlCache[[]Pool]
	chainsCache   *ttlCache[[]ChainTVL]
}

// NewClient creates a DeFiLlama API client.
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		http:          &http.Client{Timeout: 30 * time.Second},
		baseURL:       baseURL,
		yieldURL:      yieldBaseURL,
		protocolCache: newTTLCache[*Protocol](protocolCacheTTL),
		poolsCache:    newTTLCache[[]Pool](poolsCacheTTL),
		chainsCache:   newTTLCache[[]ChainTVL](chainsCacheTTL),
	}
}

func (c *Client) doGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.http.Do(req)
}

// GetProtocol fetches detailed info for a protocol by slug.
func (c *Client) GetProtocol(ctx context.Context, slug string) (*Protocol, error) {
	if cached, ok := c.protocolCache.get(slug); ok {
		return cached, nil
	}

	resp, err := c.doGet(ctx, c.baseURL+"/protocol/"+url.PathEscape(slug))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return nil, nil // not found
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("defillama: protocol %q returned %d", slug, resp.StatusCode)
	}

	var p Protocol
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}

	c.protocolCache.set(slug, &p)
	return &p, nil
}

// GetYieldPools fetches all yield pools. Results are cached for 10 minutes.
func (c *Client) GetYieldPools(ctx context.Context) ([]Pool, error) {
	if cached, ok := c.poolsCache.get("all"); ok {
		return cached, nil
	}

	resp, err := c.doGet(ctx, c.yieldURL+"/pools")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("defillama: yield pools returned %d", resp.StatusCode)
	}

	var pr poolsResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, err
	}

	c.poolsCache.set("all", pr.Data)
	return pr.Data, nil
}

// GetChainsTVL fetches TVL for all chains. Results are cached for 5 minutes.
func (c *Client) GetChainsTVL(ctx context.Context) ([]ChainTVL, error) {
	if cached, ok := c.chainsCache.get("all"); ok {
		return cached, nil
	}

	resp, err := c.doGet(ctx, c.baseURL+"/v2/chains")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("defillama: chains TVL returned %d", resp.StatusCode)
	}

	var chains []ChainTVL
	if err := json.NewDecoder(resp.Body).Decode(&chains); err != nil {
		return nil, err
	}

	c.chainsCache.set("all", chains)
	return chains, nil
}
