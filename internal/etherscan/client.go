package etherscan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/vultisig/mcp/internal/cache"
)

const (
	defaultBaseURL = "https://api.etherscan.io/v2/api"
	abiCacheTTL    = 24 * time.Hour
	txCacheTTL     = 2 * time.Minute
)

// ChainIDs maps Vultisig chain names to Etherscan V2 chain IDs.
var ChainIDs = map[string]string{
	"Ethereum":  "1",
	"BSC":       "56",
	"Polygon":   "137",
	"Arbitrum":  "42161",
	"Optimism":  "10",
	"Base":      "8453",
	"Avalanche": "43114",
	"Blast":     "81457",
	"Mantle":    "5000",
	"Zksync":    "324",
}

// Client wraps the Etherscan V2 unified API with caching and rate limit handling.
type Client struct {
	http     *http.Client
	baseURL  string
	apiKey   string
	abiCache *cache.TTL[string]
	srcCache *cache.TTL[*SourceInfo]
	txCache  *cache.TTL[[]Transaction]
}

// NewClient creates an Etherscan API client. apiKey is optional (empty = free tier).
func NewClient(apiKey string) *Client {
	return &Client{
		http:     &http.Client{Timeout: 30 * time.Second},
		baseURL:  defaultBaseURL,
		apiKey:   apiKey,
		abiCache: cache.NewTTL[string](abiCacheTTL),
		srcCache: cache.NewTTL[*SourceInfo](abiCacheTTL),
		txCache:  cache.NewTTL[[]Transaction](txCacheTTL),
	}
}

// doGet calls the Etherscan V2 API and extracts the result field.
func (c *Client) doGet(ctx context.Context, chain string, params map[string]string) (json.RawMessage, error) {
	chainID, ok := ChainIDs[chain]
	if !ok {
		return nil, fmt.Errorf("unsupported chain: %s", chain)
	}

	u, _ := url.Parse(c.baseURL)
	q := u.Query()
	q.Set("chainid", chainID)
	for k, v := range params {
		q.Set(k, v)
	}
	if c.apiKey != "" {
		q.Set("apikey", c.apiKey)
	}
	u.RawQuery = q.Encode()

	return c.fetch(ctx, u.String())
}

func (c *Client) fetch(ctx context.Context, rawURL string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("etherscan: HTTP %d", resp.StatusCode)
	}

	var ar apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, err
	}

	if ar.Status == "0" {
		msg := string(ar.Result)
		// Remove surrounding quotes from result string
		msg = strings.Trim(msg, "\"")
		if strings.Contains(strings.ToLower(msg), "rate limit") {
			return nil, fmt.Errorf("etherscan rate limit exceeded, please wait a moment and try again")
		}
		if strings.Contains(msg, "Missing/Invalid API Key") {
			return nil, fmt.Errorf("etherscan API key required: set ETHERSCAN_API_KEY environment variable (free at etherscan.io)")
		}
		return nil, fmt.Errorf("%s", msg)
	}

	return ar.Result, nil
}

// GetContractABI returns the ABI JSON string for a verified contract.
func (c *Client) GetContractABI(ctx context.Context, chain, address string) (string, error) {
	cacheKey := chain + ":" + strings.ToLower(address)
	if cached, ok := c.abiCache.Get(cacheKey); ok {
		return cached, nil
	}

	result, err := c.doGet(ctx, chain, map[string]string{
		"module":  "contract",
		"action":  "getabi",
		"address": address,
	})
	if err != nil {
		return "", err
	}

	var abi string
	if err := json.Unmarshal(result, &abi); err != nil {
		return "", err
	}

	c.abiCache.Set(cacheKey, abi)
	return abi, nil
}

// GetSourceCode returns contract source/verification info including proxy detection.
func (c *Client) GetSourceCode(ctx context.Context, chain, address string) (*SourceInfo, error) {
	cacheKey := chain + ":" + strings.ToLower(address)
	if cached, ok := c.srcCache.Get(cacheKey); ok {
		cp := *cached
		return &cp, nil
	}

	result, err := c.doGet(ctx, chain, map[string]string{
		"module":  "contract",
		"action":  "getsourcecode",
		"address": address,
	})
	if err != nil {
		return nil, err
	}

	var infos []SourceInfo
	if err := json.Unmarshal(result, &infos); err != nil {
		return nil, err
	}
	if len(infos) == 0 {
		return nil, fmt.Errorf("no source info returned")
	}

	info := infos[0]
	c.srcCache.Set(cacheKey, &info)
	cp := info
	return &cp, nil
}

func cloneTransactions(in []Transaction) []Transaction {
	out := make([]Transaction, len(in))
	copy(out, in)
	return out
}

// GetTxList returns recent transactions for an address.
func (c *Client) GetTxList(ctx context.Context, chain, address string, page, pageSize int) ([]Transaction, error) {
	cacheKey := fmt.Sprintf("%s:%s:%d:%d", chain, strings.ToLower(address), page, pageSize)
	if cached, ok := c.txCache.Get(cacheKey); ok {
		return cloneTransactions(cached), nil
	}

	result, err := c.doGet(ctx, chain, map[string]string{
		"module":  "account",
		"action":  "txlist",
		"address": address,
		"page":    fmt.Sprintf("%d", page),
		"offset":  fmt.Sprintf("%d", pageSize),
		"sort":    "desc",
	})
	if err != nil {
		// Etherscan returns status "0" for addresses with no transactions.
		// The error message varies: "No transactions found" or just "[]".
		errMsg := err.Error()
		if strings.Contains(errMsg, "No transactions found") || errMsg == "[]" {
			c.txCache.Set(cacheKey, []Transaction{})
			return []Transaction{}, nil
		}
		return nil, err
	}

	var txs []Transaction
	if err := json.Unmarshal(result, &txs); err != nil {
		return nil, err
	}

	c.txCache.Set(cacheKey, txs)
	return cloneTransactions(txs), nil
}
