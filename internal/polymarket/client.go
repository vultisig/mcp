package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultGammaURL = "https://gamma-api.polymarket.com"
	defaultClobURL  = "https://clob.polymarket.com"
	defaultDataURL  = "https://data-api.polymarket.com"

	eventCacheTTL = 5 * time.Minute
	httpTimeout   = 30 * time.Second
)

// Client provides access to Polymarket Gamma, CLOB, and Data APIs.
type Client struct {
	http     *http.Client
	gammaURL string
	clobURL  string
	dataURL  string

	eventCache *ttlCache[[]Event]
}

// NewClient creates a new Polymarket API client.
func NewClient() *Client {
	return &Client{
		http:       &http.Client{Timeout: httpTimeout},
		gammaURL:   defaultGammaURL,
		clobURL:    defaultClobURL,
		dataURL:    defaultDataURL,
		eventCache: newTTLCache[[]Event](eventCacheTTL),
	}
}

// doGet performs a GET request and returns the response body.
func (c *Client) doGet(ctx context.Context, baseURL, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("polymarket: %s returned %d: %s", path, resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// SearchEvents searches for prediction market events via Gamma API.
func (c *Client) SearchEvents(ctx context.Context, query string, activeOnly bool) ([]Event, error) {
	cacheKey := query + fmt.Sprintf(":%v", activeOnly)
	if cached, ok := c.eventCache.get(cacheKey); ok {
		return cached, nil
	}

	params := url.Values{}
	params.Set("title_like", query)
	params.Set("limit", "10")
	params.Set("order", "volume")
	params.Set("ascending", "false")
	if activeOnly {
		params.Set("active", "true")
		params.Set("closed", "false")
	}

	body, err := c.doGet(ctx, c.gammaURL, "/events?"+params.Encode())
	if err != nil {
		return nil, err
	}

	var events []Event
	if err := json.Unmarshal(body, &events); err != nil {
		return nil, fmt.Errorf("polymarket: decode events: %w", err)
	}

	c.eventCache.set(cacheKey, events)
	return events, nil
}

// GetEvent fetches a single event by slug via query param.
func (c *Client) GetEvent(ctx context.Context, slug string) (*Event, error) {
	params := url.Values{}
	params.Set("slug", slug)
	body, err := c.doGet(ctx, c.gammaURL, "/events?"+params.Encode())
	if err != nil {
		return nil, err
	}

	var events []Event
	if err := json.Unmarshal(body, &events); err != nil {
		return nil, fmt.Errorf("polymarket: decode event: %w", err)
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("polymarket: event %q not found", slug)
	}
	return &events[0], nil
}

// GetMarket fetches a single market by numeric ID or slug.
// Tries path lookup first (/markets/{id} — works for numeric IDs),
// then falls back to slug query (/markets?slug={slug}).
func (c *Client) GetMarket(ctx context.Context, id string) (*Market, error) {
	// Try path lookup (returns single object for numeric IDs)
	body, err := c.doGet(ctx, c.gammaURL, "/markets/"+url.PathEscape(id))
	if err == nil {
		var market Market
		if err := json.Unmarshal(body, &market); err == nil && market.ID != "" {
			return &market, nil
		}
	}

	// Fall back to slug query (returns array)
	params := url.Values{}
	params.Set("slug", id)
	params.Set("limit", "1")
	body, err = c.doGet(ctx, c.gammaURL, "/markets?"+params.Encode())
	if err != nil {
		return nil, fmt.Errorf("polymarket: market lookup failed: %w", err)
	}

	var markets []Market
	if err := json.Unmarshal(body, &markets); err != nil {
		return nil, fmt.Errorf("polymarket: decode market: %w", err)
	}
	if len(markets) == 0 {
		return nil, fmt.Errorf("polymarket: market %q not found", id)
	}
	return &markets[0], nil
}

// GetOrderBook fetches the order book for a CLOB token.
func (c *Client) GetOrderBook(ctx context.Context, tokenID string) (*OrderBook, error) {
	body, err := c.doGet(ctx, c.clobURL, "/book?token_id="+url.QueryEscape(tokenID))
	if err != nil {
		return nil, err
	}

	var ob OrderBook
	if err := json.Unmarshal(body, &ob); err != nil {
		return nil, fmt.Errorf("polymarket: decode orderbook: %w", err)
	}
	return &ob, nil
}

// GetPrice fetches the buy price, sell price, and midpoint for a CLOB token.
func (c *Client) GetPrice(ctx context.Context, tokenID string) (*PriceInfo, error) {
	escapedID := url.QueryEscape(tokenID)

	midBody, err := c.doGet(ctx, c.clobURL, "/midpoint?token_id="+escapedID)
	if err != nil {
		return nil, err
	}
	buyBody, err := c.doGet(ctx, c.clobURL, "/price?token_id="+escapedID+"&side=BUY")
	if err != nil {
		return nil, err
	}
	sellBody, err := c.doGet(ctx, c.clobURL, "/price?token_id="+escapedID+"&side=SELL")
	if err != nil {
		return nil, err
	}

	var midResp struct {
		Mid string `json:"mid"`
	}
	var buyResp, sellResp struct {
		Price string `json:"price"`
	}

	if err := json.Unmarshal(midBody, &midResp); err != nil {
		return nil, fmt.Errorf("polymarket: decode midpoint: %w", err)
	}
	if err := json.Unmarshal(buyBody, &buyResp); err != nil {
		return nil, fmt.Errorf("polymarket: decode buy price: %w", err)
	}
	if err := json.Unmarshal(sellBody, &sellResp); err != nil {
		return nil, fmt.Errorf("polymarket: decode sell price: %w", err)
	}

	return &PriceInfo{
		BuyPrice:  buyResp.Price,
		SellPrice: sellResp.Price,
		Midpoint:  midResp.Mid,
	}, nil
}

// GetPositions fetches positions for an address via the Data API.
func (c *Client) GetPositions(ctx context.Context, address string) ([]Position, error) {
	body, err := c.doGet(ctx, c.dataURL, "/positions?user="+url.QueryEscape(address))
	if err != nil {
		return nil, err
	}

	var positions []Position
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, fmt.Errorf("polymarket: decode positions: %w", err)
	}
	return positions, nil
}

// GetTrades fetches trade history for an address via the Data API.
func (c *Client) GetTrades(ctx context.Context, address string, limit int) ([]Trade, error) {
	params := url.Values{}
	params.Set("user", address)
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}

	body, err := c.doGet(ctx, c.dataURL, "/trades?"+params.Encode())
	if err != nil {
		return nil, err
	}

	var trades []Trade
	if err := json.Unmarshal(body, &trades); err != nil {
		return nil, fmt.Errorf("polymarket: decode trades: %w", err)
	}
	return trades, nil
}
