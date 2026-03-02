package polymarket

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
)

const (
	// Exchange contract addresses on Polygon (Chain ID 137)
	CTFExchangeAddress        = "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E"
	NegRiskCTFExchangeAddress = "0xC5d563A36AE78145C45a50134d48A1215220f80a"
	NegRiskAdapterAddress     = "0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296"
	ConditionalTokensAddress  = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"
	USDCeAddress              = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"
	PolygonChainID            = 137
)

// Side represents BUY or SELL.
type Side int

const (
	Buy  Side = 0
	Sell Side = 1
)

// OrderType represents the order execution strategy.
type OrderType string

const (
	GTC OrderType = "GTC"
	GTD OrderType = "GTD"
	FOK OrderType = "FOK"
	FAK OrderType = "FAK"
)

// OrderParams holds the user-facing parameters for building an order.
type OrderParams struct {
	TokenID    string
	Side       Side
	Price      string
	Size       string    // Number of shares (always required)
	Spend      string    // Dollar amount — for FOK/FAK BUY, used as makerAmount base instead of price×size
	OrderType  OrderType
	Expiry     int64 // Unix timestamp, only for GTD
	NegRisk    bool
	TickSize   string
	FeeRateBps string
}

// EIP712Payload represents a typed data payload ready for signing.
type EIP712Payload struct {
	PrimaryType string         `json:"primaryType"`
	Domain      map[string]any `json:"domain"`
	Types       map[string]any `json:"types"`
	Message     map[string]any `json:"message"`
}

// BuildOrderResult holds the complete output of order construction.
type BuildOrderResult struct {
	OrderRef      string         `json:"order_ref"`
	OrderEIP712   EIP712Payload  `json:"order_eip712"`
	AuthEIP712    EIP712Payload  `json:"auth_eip712"`
	OrderSummary  string         `json:"order_summary"`
	ClobParams    map[string]any `json:"clob_params"`
	FeeEstimate   string         `json:"fee_estimate"`
	EffectiveCost string         `json:"effective_cost"`
}

// OrderStore persists build results server-side (10-min TTL).
// Keyed by both order_ref and maker address (lowercase) so submit_order
// can retrieve by address alone — the LLM doesn't need to thread the ref.
type OrderStore struct {
	cache *ttlCache[*BuildOrderResult]
}

func NewOrderStore() *OrderStore {
	return &OrderStore{cache: newTTLCache[*BuildOrderResult](10 * time.Minute)}
}

func (s *OrderStore) Put(ref string, makerAddr string, result *BuildOrderResult) {
	s.cache.set(ref, result)
	s.cache.set("addr:"+strings.ToLower(makerAddr), result)
}

func (s *OrderStore) Get(ref string) (*BuildOrderResult, bool) {
	return s.cache.get(ref)
}

func (s *OrderStore) GetByAddress(addr string) (*BuildOrderResult, bool) {
	return s.cache.get("addr:" + strings.ToLower(addr))
}

// AuthCache caches derived API credentials by wallet address.
// Avoids re-signing the auth payload for subsequent orders.
type AuthCache struct {
	cache *ttlCache[*ApiCreds]
}

func NewAuthCache() *AuthCache {
	return &AuthCache{cache: newTTLCache[*ApiCreds](30 * time.Minute)}
}

func (c *AuthCache) Put(address string, creds *ApiCreds) {
	c.cache.set(strings.ToLower(address), creds)
}

func (c *AuthCache) Get(address string) (*ApiCreds, bool) {
	return c.cache.get(strings.ToLower(address))
}

// roundConfig mirrors py-clob-client's ROUNDING_CONFIG.
// price: decimal places for price, size: for base qty (always 2), amount: for derived qty.
type roundConfig struct {
	price, size, amount int
}

var roundingConfig = map[string]roundConfig{
	"0.1":    {price: 1, size: 2, amount: 3},
	"0.01":   {price: 2, size: 2, amount: 4},
	"0.001":  {price: 3, size: 2, amount: 5},
	"0.0001": {price: 4, size: 2, amount: 6},
}

func getRoundConfig(tickSize string) roundConfig {
	if rc, ok := roundingConfig[tickSize]; ok {
		return rc
	}
	// Fallback: derive from tick size string
	dec := tickSizeDecimals(tickSize)
	if dec == 0 {
		dec = 2
	}
	return roundConfig{price: dec, size: 2, amount: dec + 2}
}

func tickSizeDecimals(tickSize string) int {
	if idx := strings.IndexByte(tickSize, '.'); idx >= 0 {
		return len(strings.TrimRight(tickSize[idx+1:], "0"))
	}
	return 0
}

func roundNormal(f float64, n int) float64 {
	pow := math.Pow10(n)
	return math.Round(f*pow) / pow
}

func roundDown(f float64, n int) float64 {
	pow := math.Pow10(n)
	return math.Floor(f*pow) / pow
}

func roundUp(f float64, n int) float64 {
	pow := math.Pow10(n)
	return math.Ceil(f*pow) / pow
}

// decimalPlaces returns the number of decimal places in f's string representation.
func decimalPlaces(f float64) int {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return len(s) - i - 1
	}
	return 0
}

// conditionalRound applies py-clob-client's rounding strategy:
// try round_up with extra precision first, fall back to round_down.
func conditionalRound(val float64, maxDec int) float64 {
	if decimalPlaces(val) <= maxDec {
		return val
	}
	val = roundUp(val, maxDec+4)
	if decimalPlaces(val) > maxDec {
		val = roundDown(val, maxDec)
	}
	return val
}

// toTokenDecimals converts a human-readable amount to atomic units (×1e6).
func toTokenDecimals(f float64) *big.Int {
	v := f * 1e6
	if decimalPlaces(v) > 0 {
		v = roundNormal(v, 0)
	}
	return new(big.Int).SetInt64(int64(v))
}

// BuildOrder constructs EIP-712 payloads for both the order and L1 auth.
func BuildOrder(maker string, params OrderParams) (*BuildOrderResult, error) {
	priceF, err := strconv.ParseFloat(params.Price, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid price: %s", params.Price)
	}
	if priceF <= 0 {
		return nil, fmt.Errorf("price must be positive, got: %s", params.Price)
	}
	sizeF, err := strconv.ParseFloat(params.Size, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid size: %s", params.Size)
	}
	if sizeF <= 0 {
		return nil, fmt.Errorf("size must be positive, got: %s", params.Size)
	}

	// Validate order type
	switch params.OrderType {
	case GTC, GTD, FOK, FAK:
		// valid
	default:
		return nil, fmt.Errorf("unknown order type: %q (must be GTC, GTD, FOK, or FAK)", params.OrderType)
	}
	if params.OrderType == GTD && params.Expiry <= 0 {
		return nil, fmt.Errorf("GTD orders require a positive expiry timestamp")
	}

	rc := getRoundConfig(params.TickSize)

	// Round price to tick precision
	priceF = roundNormal(priceF, rc.price)
	if priceF > 1.0 {
		return nil, fmt.Errorf("price must be <= 1.0 (probability), got: %.4f", priceF)
	}

	// Compute maker/taker amounts following py-clob-client's exact rounding rules:
	// - "base" quantity → round_down to rc.size (always 2) decimal places
	// - "derived" quantity → conditional round to rc.amount decimal places
	//
	// Limit BUY:  base=shares(taker), derived=price×shares(maker)
	// Limit SELL: base=shares(maker), derived=price×shares(taker)
	// Market BUY (FOK/FAK with spend): base=spend(maker), derived=spend/price(taker)
	// Market SELL (FOK/FAK): base=shares(maker), derived=price×shares(taker)
	isMarketOrder := params.OrderType == FOK || params.OrderType == FAK

	var rawMaker, rawTaker float64
	if isMarketOrder && params.Side == Buy && params.Spend != "" {
		// Market BUY: USDC spend is the base, shares are derived
		spendF, err := strconv.ParseFloat(params.Spend, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid spend: %s", params.Spend)
		}
		rawMaker = roundDown(spendF, rc.size)
		rawTaker = rawMaker / priceF
		rawTaker = conditionalRound(rawTaker, rc.amount)
	} else if params.Side == Buy {
		// Limit BUY: shares are the base, USDC is derived
		rawTaker = roundDown(sizeF, rc.size)
		rawMaker = rawTaker * priceF
		rawMaker = conditionalRound(rawMaker, rc.amount)
	} else {
		// SELL (both limit and market): shares are the base, USDC is derived
		rawMaker = roundDown(sizeF, rc.size)
		rawTaker = rawMaker * priceF
		rawTaker = conditionalRound(rawTaker, rc.amount)
	}

	makerAmount := toTokenDecimals(rawMaker)
	takerAmount := toTokenDecimals(rawTaker)

	// Determine exchange contract
	exchange := CTFExchangeAddress
	if params.NegRisk {
		exchange = NegRiskCTFExchangeAddress
	}

	// Salt (random-ish from timestamp)
	salt := fmt.Sprintf("%d", time.Now().UnixNano())

	// Expiry for GTD, else 0
	expiration := "0"
	if params.OrderType == GTD && params.Expiry > 0 {
		expiration = fmt.Sprintf("%d", params.Expiry)
	}

	// Nonce: 0 for new orders
	nonce := "0"

	// Fee rate
	feeRateBps := params.FeeRateBps
	if feeRateBps == "" {
		feeRateBps = "0"
	}

	// Build order EIP-712
	orderEIP712 := EIP712Payload{
		PrimaryType: "Order",
		Domain: map[string]any{
			"name":              "Polymarket CTF Exchange",
			"version":           "1",
			"chainId":           PolygonChainID,
			"verifyingContract": exchange,
		},
		Types: map[string]any{
			"EIP712Domain": []map[string]string{
				{"name": "name", "type": "string"},
				{"name": "version", "type": "string"},
				{"name": "chainId", "type": "uint256"},
				{"name": "verifyingContract", "type": "address"},
			},
			"Order": []map[string]string{
				{"name": "salt", "type": "uint256"},
				{"name": "maker", "type": "address"},
				{"name": "signer", "type": "address"},
				{"name": "taker", "type": "address"},
				{"name": "tokenId", "type": "uint256"},
				{"name": "makerAmount", "type": "uint256"},
				{"name": "takerAmount", "type": "uint256"},
				{"name": "expiration", "type": "uint256"},
				{"name": "nonce", "type": "uint256"},
				{"name": "feeRateBps", "type": "uint256"},
				{"name": "side", "type": "uint8"},
				{"name": "signatureType", "type": "uint8"},
			},
		},
		Message: map[string]any{
			"salt":          salt,
			"maker":         maker,
			"signer":        maker,
			"taker":         "0x0000000000000000000000000000000000000000",
			"tokenId":       params.TokenID,
			"makerAmount":   makerAmount.String(),
			"takerAmount":   takerAmount.String(),
			"expiration":    expiration,
			"nonce":         nonce,
			"feeRateBps":    feeRateBps,
			"side":          int(params.Side),
			"signatureType": 0, // EOA
		},
	}

	// Build auth EIP-712 (L1 authentication for CLOB API key derivation)
	authTimestamp := fmt.Sprintf("%d", time.Now().Unix())
	authEIP712 := EIP712Payload{
		PrimaryType: "ClobAuth",
		Domain: map[string]any{
			"name":    "ClobAuthDomain",
			"version": "1",
			"chainId": PolygonChainID,
		},
		Types: map[string]any{
			"EIP712Domain": []map[string]string{
				{"name": "name", "type": "string"},
				{"name": "version", "type": "string"},
				{"name": "chainId", "type": "uint256"},
			},
			"ClobAuth": []map[string]string{
				{"name": "address", "type": "address"},
				{"name": "timestamp", "type": "string"},
				{"name": "nonce", "type": "uint256"},
				{"name": "message", "type": "string"},
			},
		},
		Message: map[string]any{
			"address":   maker,
			"timestamp": authTimestamp,
			"nonce":     0,
			"message":   "This message attests that I control the given wallet",
		},
	}

	// Fee estimate (priceF and sizeF already set above)
	feeRate, _ := strconv.ParseFloat(feeRateBps, 64)
	minPrice := priceF
	if 1-priceF < minPrice {
		minPrice = 1 - priceF
	}
	fee := feeRate / 10000 * minPrice * sizeF
	effectiveCost := priceF*sizeF + fee

	sideStr := "BUY"
	if params.Side == Sell {
		sideStr = "SELL"
	}

	return &BuildOrderResult{
		OrderEIP712: orderEIP712,
		AuthEIP712:  authEIP712,
		OrderSummary: fmt.Sprintf("%s %.0f shares at $%.4f = $%.2f USDC.e",
			sideStr, sizeF, priceF, priceF*sizeF),
		ClobParams: map[string]any{
			"token_id":       params.TokenID,
			"neg_risk":       params.NegRisk,
			"tick_size":      params.TickSize,
			"order_type":     string(params.OrderType),
			"auth_timestamp": authTimestamp,
		},
		FeeEstimate:   fmt.Sprintf("$%.4f", fee),
		EffectiveCost: fmt.Sprintf("$%.2f", effectiveCost),
	}, nil
}
