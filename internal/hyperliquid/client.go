// Package hyperliquid provides a REST client for the Hyperliquid Info and Exchange APIs.
package hyperliquid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps the Hyperliquid REST API.
type Client struct {
	apiURL     string
	httpClient *http.Client
}

// NewClient creates a new Hyperliquid client with the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		apiURL: baseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// postInfo sends a POST /info request with the given body and decodes the response into result.
func (c *Client) postInfo(ctx context.Context, reqBody any, result any) error {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+"/info", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return nil
}

// GetMetaAndAssetCtxs returns perpetual universe metadata and per-asset market context.
// The response is a 2-element JSON array: [Meta, []AssetCtx].
func (c *Client) GetMetaAndAssetCtxs(ctx context.Context) (Meta, []AssetCtx, error) {
	req := infoRequest{Type: "metaAndAssetCtxs"}
	var raw []json.RawMessage
	if err := c.postInfo(ctx, req, &raw); err != nil {
		return Meta{}, nil, err
	}
	if len(raw) != 2 {
		return Meta{}, nil, fmt.Errorf("unexpected response length: %d", len(raw))
	}
	var meta Meta
	if err := json.Unmarshal(raw[0], &meta); err != nil {
		return Meta{}, nil, fmt.Errorf("parse meta: %w", err)
	}
	var ctxs []AssetCtx
	if err := json.Unmarshal(raw[1], &ctxs); err != nil {
		return Meta{}, nil, fmt.Errorf("parse asset ctxs: %w", err)
	}
	return meta, ctxs, nil
}

// GetSpotMetaAndAssetCtxs returns spot universe metadata and per-market context.
// The response is a 2-element JSON array: [SpotMeta, []SpotAssetCtx].
func (c *Client) GetSpotMetaAndAssetCtxs(ctx context.Context) (SpotMeta, []SpotAssetCtx, error) {
	req := infoRequest{Type: "spotMetaAndAssetCtxs"}
	var raw []json.RawMessage
	if err := c.postInfo(ctx, req, &raw); err != nil {
		return SpotMeta{}, nil, err
	}
	if len(raw) != 2 {
		return SpotMeta{}, nil, fmt.Errorf("unexpected response length: %d", len(raw))
	}
	var meta SpotMeta
	if err := json.Unmarshal(raw[0], &meta); err != nil {
		return SpotMeta{}, nil, fmt.Errorf("parse spot meta: %w", err)
	}
	var ctxs []SpotAssetCtx
	if err := json.Unmarshal(raw[1], &ctxs); err != nil {
		return SpotMeta{}, nil, fmt.Errorf("parse spot asset ctxs: %w", err)
	}
	return meta, ctxs, nil
}

// GetUserState returns the perpetual clearinghouse state for the given address.
func (c *Client) GetUserState(ctx context.Context, address string) (UserState, error) {
	req := infoRequest{Type: "clearinghouseState", User: address}
	var result UserState
	if err := c.postInfo(ctx, req, &result); err != nil {
		return UserState{}, err
	}
	return result, nil
}

// GetSpotUserState returns the spot clearinghouse state for the given address.
func (c *Client) GetSpotUserState(ctx context.Context, address string) (SpotUserState, error) {
	req := infoRequest{Type: "spotClearinghouseState", User: address}
	var result SpotUserState
	if err := c.postInfo(ctx, req, &result); err != nil {
		return SpotUserState{}, err
	}
	return result, nil
}

// GetL2Book returns the level-2 order book for the given coin.
func (c *Client) GetL2Book(ctx context.Context, coin string) (L2Book, error) {
	req := infoRequest{Type: "l2Book", Coin: coin}
	var result L2Book
	if err := c.postInfo(ctx, req, &result); err != nil {
		return L2Book{}, err
	}
	return result, nil
}

// GetOpenOrders returns all open resting orders for the given address.
func (c *Client) GetOpenOrders(ctx context.Context, address string) ([]OpenOrder, error) {
	req := infoRequest{Type: "openOrders", User: address}
	var result []OpenOrder
	if err := c.postInfo(ctx, req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetUserFills returns recent trade fills for the given address.
func (c *Client) GetUserFills(ctx context.Context, address string) ([]UserFill, error) {
	req := infoRequest{Type: "userFills", User: address}
	var result []UserFill
	if err := c.postInfo(ctx, req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// QueryOrderByOid returns the status of a specific order by its OID.
func (c *Client) QueryOrderByOid(ctx context.Context, address string, oid uint64) (OrderStatus, error) {
	req := orderByOidRequest{Type: "orderByOid", User: address, OID: oid}
	var result OrderStatus
	if err := c.postInfo(ctx, req, &result); err != nil {
		return OrderStatus{}, err
	}
	return result, nil
}
