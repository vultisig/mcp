package polymarket

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

const (
	// Exchange contract addresses on Polygon (Chain ID 137)
	CTFExchangeAddress        = "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E"
	NegRiskCTFExchangeAddress = "0xC5d563A36AE78145C45a50134d48A1215220f80a"
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
	TokenID   string
	Side      Side
	Price     string
	Size      string
	OrderType OrderType
	Expiry    int64 // Unix timestamp, only for GTD
	NegRisk   bool
	TickSize  string
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

// BuildOrder constructs EIP-712 payloads for both the order and L1 auth.
func BuildOrder(maker string, params OrderParams) (*BuildOrderResult, error) {
	price, ok := new(big.Float).SetString(params.Price)
	if !ok {
		return nil, fmt.Errorf("invalid price: %s", params.Price)
	}
	size, ok := new(big.Float).SetString(params.Size)
	if !ok {
		return nil, fmt.Errorf("invalid size: %s", params.Size)
	}

	// Compute maker/taker amounts (USDC.e has 6 decimals)
	// makerAmount = price * size * 1e6 (for BUY) or size * 1e6 (for SELL)
	// takerAmount = size * 1e6 (for BUY) or price * size * 1e6 (for SELL)
	scale := new(big.Float).SetInt64(1_000_000)
	cost := new(big.Float).Mul(price, size)

	var makerAmount, takerAmount *big.Int
	if params.Side == Buy {
		ma := new(big.Float).Mul(cost, scale)
		makerAmount, _ = ma.Int(nil)
		ta := new(big.Float).Mul(size, scale)
		takerAmount, _ = ta.Int(nil)
	} else {
		ma := new(big.Float).Mul(size, scale)
		makerAmount, _ = ma.Int(nil)
		ta := new(big.Float).Mul(cost, scale)
		takerAmount, _ = ta.Int(nil)
	}

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

	// Fee estimate
	priceF, _ := price.Float64()
	sizeF, _ := size.Float64()
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
