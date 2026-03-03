package polymarket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// truncateAddr shortens an address for logging: "0xAbCd...1234".
func truncateAddr(addr string) string {
	if len(addr) <= 10 {
		return addr
	}
	return addr[:6] + "..." + addr[len(addr)-4:]
}

// DeriveApiCreds derives ephemeral L2 API credentials from an L1 auth signature.
// Uses GET /auth/derive-api-key with L1 headers (address, signature, timestamp, nonce).
func (c *Client) DeriveApiCreds(ctx context.Context, address, authSignature string, timestamp int64) (*ApiCreds, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.clobURL+"/auth/derive-api-key", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("POLY_ADDRESS", address)
	req.Header.Set("POLY_SIGNATURE", authSignature)
	req.Header.Set("POLY_TIMESTAMP", fmt.Sprintf("%d", timestamp))
	req.Header.Set("POLY_NONCE", "0")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("polymarket: derive-api-key returned %d: %s", resp.StatusCode, string(respBody))
	}

	var creds ApiCreds
	if err := json.NewDecoder(resp.Body).Decode(&creds); err != nil {
		return nil, fmt.Errorf("polymarket: decode api creds: %w", err)
	}
	return &creds, nil
}

// SubmitOrder posts a signed order to the CLOB using ephemeral L2 credentials.
// address is the maker's Polygon wallet address (used for POLY_ADDRESS L2 header).
func (c *Client) SubmitOrder(ctx context.Context, address string, creds *ApiCreds, orderPayload map[string]any) (map[string]any, error) {
	return c.authenticatedPost(ctx, address, creds, "/order", orderPayload)
}

// CancelOrder cancels an open order by ID.
// address is the maker's Polygon wallet address.
func (c *Client) CancelOrder(ctx context.Context, address string, creds *ApiCreds, orderID string) (map[string]any, error) {
	return c.authenticatedDelete(ctx, address, creds, "/order/"+url.PathEscape(orderID))
}

// GetOpenOrders fetches open orders for an address, optionally filtered by market.
// address is the maker's Polygon wallet address.
func (c *Client) GetOpenOrders(ctx context.Context, address string, creds *ApiCreds, market string) ([]OpenOrder, error) {
	path := "/data/orders"
	if market != "" {
		path += "?market=" + url.QueryEscape(market)
	}

	respBody, err := c.authenticatedGet(ctx, address, creds, path)
	if err != nil {
		return nil, err
	}

	// CLOB returns paginated wrapper: {"data": [...], "next_cursor": "...", "limit": N, "count": N}
	var page struct {
		Data       []OpenOrder `json:"data"`
		NextCursor string      `json:"next_cursor"`
	}
	if err := json.Unmarshal(respBody, &page); err != nil {
		return nil, fmt.Errorf("polymarket: decode open orders: %w", err)
	}
	return page.Data, nil
}

// GetTickSize fetches the minimum tick size for a market token.
func (c *Client) GetTickSize(ctx context.Context, tokenID string) (string, error) {
	body, err := c.doGet(ctx, c.clobURL, "/tick-size?token_id="+url.QueryEscape(tokenID))
	if err != nil {
		return "", err
	}

	var resp struct {
		MinimumTickSize float64 `json:"minimum_tick_size"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("polymarket: decode tick size: %w", err)
	}
	return fmt.Sprintf("%g", resp.MinimumTickSize), nil
}

// GetNegRisk checks if a market uses the negRisk exchange.
func (c *Client) GetNegRisk(ctx context.Context, tokenID string) (bool, error) {
	body, err := c.doGet(ctx, c.clobURL, "/neg-risk?token_id="+url.QueryEscape(tokenID))
	if err != nil {
		return false, err
	}

	var resp struct {
		NegRisk bool `json:"neg_risk"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("polymarket: decode neg risk: %w", err)
	}
	return resp.NegRisk, nil
}

// GetFeeRate fetches the fee rate (in basis points) for a market token.
func (c *Client) GetFeeRate(ctx context.Context, tokenID string) (string, error) {
	body, err := c.doGet(ctx, c.clobURL, "/fee-rate?token_id="+url.QueryEscape(tokenID))
	if err != nil {
		log.Printf("[polymarket] GetFeeRate failed for %s: %v (defaulting to 0)", tokenID, err)
		return "0", nil
	}

	var resp struct {
		BaseFee json.Number `json:"base_fee"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		log.Printf("[polymarket] GetFeeRate unmarshal failed for %s: %v body=%s (defaulting to 0)", tokenID, err, string(body))
		return "0", nil
	}
	feeStr := resp.BaseFee.String()
	if feeStr == "" {
		feeStr = "0"
	}
	log.Printf("[polymarket] GetFeeRate for %s: %s bps", tokenID, feeStr)
	return feeStr, nil
}

// UpdateBalanceAllowance notifies the CLOB server to re-check on-chain balance/allowance
// for the given address. The CLOB caches this state and may not reflect recent approvals
// until this endpoint is called.
// assetType: "COLLATERAL" for USDC.e, "CONDITIONAL" for outcome tokens.
// tokenID: required when assetType is "CONDITIONAL", empty otherwise.
func (c *Client) UpdateBalanceAllowance(ctx context.Context, address string, creds *ApiCreds, sigType int, assetType string, tokenID string) error {
	path := fmt.Sprintf("/balance-allowance/update?asset_type=%s&signature_type=%d", url.QueryEscape(assetType), sigType)
	if tokenID != "" {
		path += "&token_id=" + url.QueryEscape(tokenID)
	}
	_, err := c.authenticatedGet(ctx, address, creds, path)
	if err != nil {
		log.Printf("[polymarket] UpdateBalanceAllowance failed for %s (%s): %v", truncateAddr(address), assetType, err)
		return err
	}
	// Success — no log needed (called on every order)
	return nil
}

// HealthCheck pings the CLOB server.
func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.doGet(ctx, c.clobURL, "/ok")
	return err
}

// authenticatedPost sends a POST with L2 HMAC auth headers.
func (c *Client) authenticatedPost(ctx context.Context, address string, creds *ApiCreds, path string, payload map[string]any) (map[string]any, error) {
	if creds == nil {
		return nil, fmt.Errorf("polymarket: API credentials required for POST %s", path)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	log.Printf("[polymarket] POST %s address=%s", path, truncateAddr(address))

	headers, err := BuildL2Headers(address, *creds, "POST", path, string(data))
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.clobURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("polymarket: reading response body for POST %s: %w", path, readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("polymarket: POST %s returned %d: %s", path, resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("polymarket: decode response: %w", err)
	}
	return result, nil
}

// authenticatedDelete sends a DELETE with L2 HMAC auth headers.
func (c *Client) authenticatedDelete(ctx context.Context, address string, creds *ApiCreds, path string) (map[string]any, error) {
	if creds == nil {
		return nil, fmt.Errorf("polymarket: API credentials required for DELETE %s", path)
	}
	headers, err := BuildL2Headers(address, *creds, "DELETE", path, "")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.clobURL+path, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("polymarket: reading response body for DELETE %s: %w", path, readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("polymarket: DELETE %s returned %d: %s", path, resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("polymarket: decode response: %w", err)
	}
	return result, nil
}

// authenticatedGet sends a GET with L2 HMAC auth headers.
// IMPORTANT: The HMAC signature must be computed on the base path WITHOUT query params.
// Query params are only included in the actual HTTP request URL.
func (c *Client) authenticatedGet(ctx context.Context, address string, creds *ApiCreds, path string) ([]byte, error) {
	if creds == nil {
		return nil, fmt.Errorf("polymarket: API credentials required for GET %s", path)
	}
	// Split path from query params for HMAC signing (Polymarket signs base path only)
	signPath := path
	if idx := strings.IndexByte(path, '?'); idx != -1 {
		signPath = path[:idx]
	}
	headers, err := BuildL2Headers(address, *creds, "GET", signPath, "")
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.clobURL+path, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("polymarket: reading response body for GET %s: %w", path, readErr)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("polymarket: GET %s returned %d: %s", path, resp.StatusCode, string(body))
	}
	return body, nil
}
