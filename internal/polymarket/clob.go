package polymarket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

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
func (c *Client) SubmitOrder(ctx context.Context, creds *ApiCreds, orderPayload map[string]any) (map[string]any, error) {
	return c.authenticatedPost(ctx, creds, "/order", orderPayload)
}

// CancelOrder cancels an open order by ID.
func (c *Client) CancelOrder(ctx context.Context, creds *ApiCreds, orderID string) (map[string]any, error) {
	return c.authenticatedDelete(ctx, creds, "/order/"+url.PathEscape(orderID))
}

// GetOpenOrders fetches open orders for an address, optionally filtered by market.
func (c *Client) GetOpenOrders(ctx context.Context, creds *ApiCreds, market string) ([]OpenOrder, error) {
	path := "/data/orders"
	if market != "" {
		path += "?market=" + url.QueryEscape(market)
	}

	respBody, err := c.authenticatedGet(ctx, creds, path)
	if err != nil {
		return nil, err
	}

	var orders []OpenOrder
	if err := json.Unmarshal(respBody, &orders); err != nil {
		return nil, fmt.Errorf("polymarket: decode open orders: %w", err)
	}
	return orders, nil
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

// HealthCheck pings the CLOB server.
func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.doGet(ctx, c.clobURL, "/ok")
	return err
}

// authenticatedPost sends a POST with L2 HMAC auth headers.
func (c *Client) authenticatedPost(ctx context.Context, creds *ApiCreds, path string, payload map[string]any) (map[string]any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	headers := BuildL2Headers(*creds, "POST", path, string(data))

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

	respBody, _ := io.ReadAll(resp.Body)
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
func (c *Client) authenticatedDelete(ctx context.Context, creds *ApiCreds, path string) (map[string]any, error) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	sig := BuildHmacSignature(creds.Secret, timestamp, "DELETE", path, "")

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.clobURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("POLY_ADDRESS", creds.Key)
	req.Header.Set("POLY_SIGNATURE", sig)
	req.Header.Set("POLY_TIMESTAMP", timestamp)
	req.Header.Set("POLY_NONCE", "0")
	req.Header.Set("POLY_API_KEY", creds.Key)
	req.Header.Set("POLY_PASSPHRASE", creds.Passphrase)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
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
func (c *Client) authenticatedGet(ctx context.Context, creds *ApiCreds, path string) ([]byte, error) {
	headers := BuildL2Headers(*creds, "GET", path, "")

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

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("polymarket: GET %s returned %d: %s", path, resp.StatusCode, string(body))
	}
	return body, nil
}
