package thorchain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const defaultBaseURL = "https://thornode.ninerealms.com"

const feeCacheTTL = 2 * time.Minute

type Client struct {
	http     *http.Client
	baseURL  string
	mu       sync.RWMutex
	feeCache map[string]feeCacheEntry
}

type feeCacheEntry struct {
	rate      uint64
	expiresAt time.Time
}

func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		http:     &http.Client{Timeout: 15 * time.Second},
		baseURL:  baseURL,
		feeCache: make(map[string]feeCacheEntry),
	}
}

// SatsPerByte returns the recommended fee rate for a chain (e.g. "BTC").
// Uses THORChain inbound_addresses GasRate, matching app-recurring's feeProvider.
func (c *Client) SatsPerByte(ctx context.Context, chain string) (uint64, error) {
	c.mu.RLock()
	entry, ok := c.feeCache[chain]
	c.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.rate, nil
	}

	addresses, err := c.getInboundAddresses(ctx)
	if err != nil {
		return 0, err
	}

	for _, addr := range addresses {
		if addr.Chain == chain {
			if addr.Halted {
				return 0, fmt.Errorf("chain %s is currently halted on THORChain", chain)
			}
			rate, err := strconv.ParseUint(addr.GasRate, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("parse gas rate for %s: %w", chain, err)
			}
			c.mu.Lock()
			c.feeCache[chain] = feeCacheEntry{rate: rate, expiresAt: time.Now().Add(feeCacheTTL)}
			c.mu.Unlock()
			return rate, nil
		}
	}

	return 0, fmt.Errorf("no inbound address found for chain %s", chain)
}

type inboundAddress struct {
	Chain        string `json:"chain"`
	Address      string `json:"address"`
	GasRate      string `json:"gas_rate"`
	GasRateUnits string `json:"gas_rate_units"`
	Halted       bool   `json:"halted"`
}

func (c *Client) getInboundAddresses(ctx context.Context) ([]inboundAddress, error) {
	url := c.baseURL + "/thorchain/inbound_addresses"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("thorchain inbound_addresses: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("thorchain inbound_addresses returned %d", resp.StatusCode)
	}

	var addresses []inboundAddress
	err = json.NewDecoder(resp.Body).Decode(&addresses)
	if err != nil {
		return nil, fmt.Errorf("thorchain: decode inbound_addresses: %w", err)
	}

	return addresses, nil
}
