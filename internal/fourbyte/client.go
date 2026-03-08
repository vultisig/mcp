package fourbyte

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultBaseURL = "https://www.4byte.directory/api/v1"
)

type Signature struct {
	ID            int    `json:"id"`
	TextSignature string `json:"text_signature"`
	BytesSignature string `json:"bytes_signature"`
	HexSignature  string `json:"hex_signature"`
}

type signaturesResponse struct {
	Next     *string     `json:"next"`
	Previous *string     `json:"previous"`
	Count    int         `json:"count"`
	Results  []Signature `json:"results"`
}

type Client struct {
	http    *http.Client
	baseURL string
}

func NewClient() *Client {
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: defaultBaseURL,
	}
}

func (c *Client) doGet(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.http.Do(req)
}

func (c *Client) ResolveSelector(ctx context.Context, selector string) ([]Signature, error) {
	selector = normalizeSelector(selector)

	path := fmt.Sprintf("/signatures/?hex_signature=%s", url.QueryEscape(selector))
	resp, err := c.doGet(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("4byte: request returned %d", resp.StatusCode)
	}

	var sr signaturesResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, err
	}

	return sr.Results, nil
}

func normalizeSelector(selector string) string {
	if len(selector) == 10 && (selector[:2] == "0x" || selector[:2] == "0X") {
		return selector
	}
	if len(selector) == 8 {
		return "0x" + selector
	}
	return selector
}
