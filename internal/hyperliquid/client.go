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

// Client is a Hyperliquid REST API client.
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

// postInfo sends a POST /info request with the given body and decodes the response into dst.
func (c *Client) postInfo(ctx context.Context, body interface{}, dst interface{}) error {
	data, err := json.Marshal(body)
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if err := json.Unmarshal(respBody, dst); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	return nil
}

// GetMetaAndAssetCtxs returns perp universe metadata and live asset contexts.
func (c *Client) GetMetaAndAssetCtxs(ctx context.Context) (*MetaResponse, error) {
	var result [2]json.RawMessage
	if err := c.postInfo(ctx, infoRequest{Type: "metaAndAssetCtxs"}, &result); err != nil {
		return nil, err
	}
	var meta struct {
		Universe []Universe `json:"universe"`
	}
	if err := json.Unmarshal(result[0], &meta); err != nil {
		return nil, fmt.Errorf("parse meta: %w", err)
	}
	var ctxs []AssetCtx
	if err := json.Unmarshal(result[1], &ctxs); err != nil {
		return nil, fmt.Errorf("parse asset ctxs: %w", err)
	}
	return &MetaResponse{Universe: meta.Universe, AssetCtxs: ctxs}, nil
}

// GetSpotMetaAndAssetCtxs returns spot universe metadata and live asset contexts.
func (c *Client) GetSpotMetaAndAssetCtxs(ctx context.Context) (*SpotMetaResponse, error) {
	var result [2]json.RawMessage
	if err := c.postInfo(ctx, infoRequest{Type: "spotMetaAndAssetCtxs"}, &result); err != nil {
		return nil, err
	}
	var meta SpotMeta
	if err := json.Unmarshal(result[0], &meta); err != nil {
		return nil, fmt.Errorf("parse spot meta: %w", err)
	}
	var ctxs []SpotAssetCtx
	if err := json.Unmarshal(result[1], &ctxs); err != nil {
		return nil, fmt.Errorf("parse spot asset ctxs: %w", err)
	}
	return &SpotMetaResponse{Meta: meta, AssetCtxs: ctxs}, nil
}

// GetUserState returns the perp clearinghouse state for a given address.
func (c *Client) GetUserState(ctx context.Context, address string) (*UserState, error) {
	var result UserState
	if err := c.postInfo(ctx, userInfoRequest{Type: "clearinghouseState", User: address}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSpotUserState returns the spot clearinghouse state for a given address.
func (c *Client) GetSpotUserState(ctx context.Context, address string) (*SpotUserState, error) {
	var result SpotUserState
	if err := c.postInfo(ctx, userInfoRequest{Type: "spotClearinghouseState", User: address}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetL2Book returns the L2 order book for a given coin.
func (c *Client) GetL2Book(ctx context.Context, coin string) (*L2Book, error) {
	var result L2Book
	if err := c.postInfo(ctx, l2BookRequest{Type: "l2Book", Coin: coin}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetOpenOrders returns all open orders for a given address.
func (c *Client) GetOpenOrders(ctx context.Context, address string) ([]OpenOrder, error) {
	var result []OpenOrder
	if err := c.postInfo(ctx, userInfoRequest{Type: "openOrders", User: address}, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetUserFills returns recent trade fills for a given address.
func (c *Client) GetUserFills(ctx context.Context, address string) ([]Fill, error) {
	var result []Fill
	if err := c.postInfo(ctx, userInfoRequest{Type: "userFills", User: address}, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// QueryOrderByOid returns the status of a specific order by its OID.
func (c *Client) QueryOrderByOid(ctx context.Context, address string, oid int64) (*OrderStatus, error) {
	var result OrderStatus
	if err := c.postInfo(ctx, orderByOidRequest{Type: "orderByOid", User: address, Oid: oid}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BuildPlaceOrderAction returns an unsigned place-order action payload.
func BuildPlaceOrderAction(orders []OrderRequest, grouping string) ActionPayload {
	if grouping == "" {
		grouping = "na"
	}
	action := placeOrderAction{
		Type:     "order",
		Orders:   orders,
		Grouping: grouping,
	}
	return ActionPayload{
		Action:       action,
		EIP712Domain: hyperliquidEIP712Domain,
		EIP712Types: EIP712Types{
			Primary: "HyperliquidTransaction:Order",
			Definition: map[string][]EIP712TypeField{
				"HyperliquidTransaction:Order": {
					{Name: "hyperliquidChain", Type: "string"},
					{Name: "orders", Type: "Order[]"},
					{Name: "grouping", Type: "string"},
					{Name: "nonce", Type: "uint64"},
				},
				"Order": {
					{Name: "asset", Type: "uint32"},
					{Name: "isBuy", Type: "bool"},
					{Name: "limitPx", Type: "string"},
					{Name: "sz", Type: "string"},
					{Name: "reduceOnly", Type: "bool"},
					{Name: "orderType", Type: "string"},
					{Name: "cloid", Type: "string"},
				},
			},
		},
	}
}

// BuildCancelAction returns an unsigned cancel-order action payload.
func BuildCancelAction(cancels []CancelOrder) ActionPayload {
	action := cancelAction{
		Type:    "cancel",
		Cancels: cancels,
	}
	return ActionPayload{
		Action:       action,
		EIP712Domain: hyperliquidEIP712Domain,
		EIP712Types: EIP712Types{
			Primary: "HyperliquidTransaction:Cancel",
			Definition: map[string][]EIP712TypeField{
				"HyperliquidTransaction:Cancel": {
					{Name: "hyperliquidChain", Type: "string"},
					{Name: "cancels", Type: "CancelRequest[]"},
					{Name: "nonce", Type: "uint64"},
				},
				"CancelRequest": {
					{Name: "asset", Type: "uint32"},
					{Name: "cloid", Type: "string"},
				},
			},
		},
	}
}

// BuildModifyAction returns an unsigned modify-order action payload.
func BuildModifyAction(modifies []ModifyOrder) ActionPayload {
	action := modifyAction{
		Type:     "batchModify",
		Modifies: modifies,
	}
	return ActionPayload{
		Action:       action,
		EIP712Domain: hyperliquidEIP712Domain,
		EIP712Types: EIP712Types{
			Primary: "HyperliquidTransaction:BatchModifyOrders",
			Definition: map[string][]EIP712TypeField{
				"HyperliquidTransaction:BatchModifyOrders": {
					{Name: "hyperliquidChain", Type: "string"},
					{Name: "modifies", Type: "ModifyRequest[]"},
					{Name: "nonce", Type: "uint64"},
				},
				"ModifyRequest": {
					{Name: "oid", Type: "uint64"},
					{Name: "order", Type: "Order"},
				},
				"Order": {
					{Name: "asset", Type: "uint32"},
					{Name: "isBuy", Type: "bool"},
					{Name: "limitPx", Type: "string"},
					{Name: "sz", Type: "string"},
					{Name: "reduceOnly", Type: "bool"},
					{Name: "orderType", Type: "string"},
					{Name: "cloid", Type: "string"},
				},
			},
		},
	}
}

// BuildUpdateLeverageAction returns an unsigned update-leverage action payload.
func BuildUpdateLeverageAction(asset int, isCross bool, leverage int) ActionPayload {
	action := updateLeverageAction{
		Type:     "updateLeverage",
		Asset:    asset,
		IsCross:  isCross,
		Leverage: leverage,
	}
	return ActionPayload{
		Action:       action,
		EIP712Domain: hyperliquidEIP712Domain,
		EIP712Types: EIP712Types{
			Primary: "HyperliquidTransaction:UpdateLeverage",
			Definition: map[string][]EIP712TypeField{
				"HyperliquidTransaction:UpdateLeverage": {
					{Name: "hyperliquidChain", Type: "string"},
					{Name: "asset", Type: "uint32"},
					{Name: "isCross", Type: "bool"},
					{Name: "leverage", Type: "uint32"},
					{Name: "nonce", Type: "uint64"},
				},
			},
		},
	}
}

// BuildUsdTransferAction returns an unsigned USD transfer action payload.
func BuildUsdTransferAction(destination, amount string) ActionPayload {
	action := usdTransferAction{
		Type:        "usdSend",
		Destination: destination,
		Amount:      amount,
	}
	return ActionPayload{
		Action:       action,
		EIP712Domain: hyperliquidEIP712Domain,
		EIP712Types: EIP712Types{
			Primary: "HyperliquidTransaction:UsdSend",
			Definition: map[string][]EIP712TypeField{
				"HyperliquidTransaction:UsdSend": {
					{Name: "hyperliquidChain", Type: "string"},
					{Name: "destination", Type: "string"},
					{Name: "amount", Type: "string"},
					{Name: "time", Type: "uint64"},
				},
			},
		},
	}
}
