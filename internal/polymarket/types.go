package polymarket

import (
	"encoding/json"
	"strings"
)

// Event represents a Polymarket prediction market event (may contain multiple outcomes).
type Event struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Active      bool     `json:"active"`
	Closed      bool     `json:"closed"`
	Volume      float64  `json:"volume"`
	Liquidity   float64  `json:"liquidity"`
	NegRisk     bool     `json:"negRisk"`
	StartDate   string   `json:"startDate"`
	EndDate     string   `json:"endDate"`
	Markets     []Market `json:"markets"`
}

// Market represents a single binary market within an event.
type Market struct {
	ID            string          `json:"id"`
	ConditionID   string          `json:"conditionId"`
	Question      string          `json:"question"`
	Slug          string          `json:"slug"`
	Outcomes      StringifiedJSON `json:"outcomes"`
	OutcomePrices StringifiedJSON `json:"outcomePrices"`
	ClobTokenIDs  StringifiedJSON `json:"clobTokenIds"`
	Active        bool            `json:"active"`
	Closed        bool            `json:"closed"`
	NegRisk       bool            `json:"negRisk"`
	Volume        string          `json:"volume"`
	Liquidity     string          `json:"liquidity"`
}

// StringifiedJSON handles fields that are JSON-encoded strings (e.g. "[\"Yes\",\"No\"]").
type StringifiedJSON []string

func (s *StringifiedJSON) UnmarshalJSON(data []byte) error {
	// Try direct array first
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*s = arr
		return nil
	}
	// Try stringified JSON (e.g. "[\"Yes\",\"No\"]")
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		*s = nil
		return nil
	}
	str = strings.TrimSpace(str)
	if str == "" || str == "null" {
		*s = nil
		return nil
	}
	return json.Unmarshal([]byte(str), (*[]string)(s))
}

// Position represents a user's position in a market.
// Note: The Polymarket data-api returns numeric fields as JSON numbers, not strings.
type Position struct {
	Asset        string      `json:"asset"`
	ConditionID  string      `json:"conditionId"`
	Market       string      `json:"market"`
	Outcome      string      `json:"outcome"`
	Size         json.Number `json:"size"`
	AvgPrice     json.Number `json:"avgPrice"`
	CurPrice     json.Number `json:"curPrice"`
	RealizedPnl  json.Number `json:"realizedPnl"`
	CurrentValue json.Number `json:"currentValue"`
	PnlPercent   json.Number `json:"pnlPercent"`
	// Additional fields from data-api
	Title     string `json:"title"`
	EventSlug string `json:"eventSlug"`
}

// OrderBookEntry represents a single entry in the order book.
type OrderBookEntry struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

// OrderBook represents the full order book for a token.
type OrderBook struct {
	Bids []OrderBookEntry `json:"bids"`
	Asks []OrderBookEntry `json:"asks"`
}

// PriceInfo holds buy/sell prices and midpoint for a token.
type PriceInfo struct {
	BuyPrice  string `json:"buy_price"`
	SellPrice string `json:"sell_price"`
	Midpoint  string `json:"midpoint"`
}

// Trade represents a historical trade.
type Trade struct {
	ID        string `json:"id"`
	Market    string `json:"market"`
	Asset     string `json:"asset"`
	Side      string `json:"side"`
	Price     string `json:"price"`
	Size      string `json:"size"`
	Outcome   string `json:"outcome"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
}

// OpenOrder represents a live order on the CLOB.
// Note: CLOB returns string fields for sizes/prices but numeric for created_at.
type OpenOrder struct {
	ID             string      `json:"id"`
	Market         string      `json:"market"`
	Asset          string      `json:"asset_id"`
	Side           string      `json:"side"`
	Price          string      `json:"price"`
	OriginalSize   string      `json:"original_size"`
	SizeMatched    string      `json:"size_matched"`
	Status         string      `json:"status"`
	OrderType      string      `json:"type"`
	Outcome        string      `json:"outcome"`
	Owner          string      `json:"owner"`
	MakerAddress   string      `json:"maker_address"`
	CreatedAt      json.Number `json:"created_at"`
	ExpirationTime string      `json:"expiration,omitempty"`
}

// MarketInfo holds metadata used for order construction.
type MarketInfo struct {
	ConditionID string `json:"condition_id"`
	TokenID     string `json:"token_id"`
	MinTickSize string `json:"minimum_tick_size"`
	NegRisk     bool   `json:"neg_risk"`
	FeeRateBps  string `json:"fee_rate_bps"`
}
