// Package hyperliquid provides a REST client for the Hyperliquid Info and Exchange APIs.
package hyperliquid

// infoRequest is the base request body for POST /info.
type infoRequest struct {
	Type string `json:"type"`
}

// userInfoRequest is used for user-specific Info queries.
type userInfoRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
}

// l2BookRequest is the request body for the l2Book Info query.
type l2BookRequest struct {
	Type string `json:"type"`
	Coin string `json:"coin"`
}

// orderByOidRequest is the request body for the orderByOid Info query.
type orderByOidRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
	Oid  int64  `json:"oid"`
}

// Universe holds metadata for a single perp asset.
type Universe struct {
	Name          string  `json:"name"`
	SzDecimals    int     `json:"szDecimals"`
	MaxLeverage   int     `json:"maxLeverage"`
	OnlyIsolated  bool    `json:"onlyIsolated"`
}

// AssetCtx holds live market context for a perp asset.
type AssetCtx struct {
	Funding     string `json:"funding"`
	OpenInterest string `json:"openInterest"`
	PrevDayPx   string `json:"prevDayPx"`
	DayNtlVlm   string `json:"dayNtlVlm"`
	Premium     string `json:"premium"`
	OraclePx    string `json:"oraclePx"`
	MarkPx      string `json:"markPx"`
	MidPx       string `json:"midPx"`
	ImpactPxs   []string `json:"impactPxs"`
}

// MetaResponse is the response for metaAndAssetCtxs.
type MetaResponse struct {
	Universe  []Universe `json:"universe"`
	AssetCtxs []AssetCtx `json:"assetCtxs"`
}

// SpotToken holds metadata for a spot token.
type SpotToken struct {
	Name        string `json:"name"`
	SzDecimals  int    `json:"szDecimals"`
	WeiDecimals int    `json:"weiDecimals"`
	Index       int    `json:"index"`
	TokenID     string `json:"tokenId"`
}

// SpotMarket holds metadata for a spot market.
type SpotMarket struct {
	Name    string `json:"name"`
	Tokens  [2]int `json:"tokens"`
	Index   int    `json:"index"`
	IsCanonical bool `json:"isCanonical"`
}

// SpotAssetCtx holds live context for a spot market.
type SpotAssetCtx struct {
	DayNtlVlm string `json:"dayNtlVlm"`
	MarkPx    string `json:"markPx"`
	MidPx     string `json:"midPx"`
	PrevDayPx string `json:"prevDayPx"`
	CirculatingSupply string `json:"circulatingSupply"`
}

// SpotMeta holds spot universe metadata.
type SpotMeta struct {
	Tokens  []SpotToken  `json:"tokens"`
	Universe []SpotMarket `json:"universe"`
}

// SpotMetaResponse is the response for spotMetaAndAssetCtxs.
type SpotMetaResponse struct {
	Meta      SpotMeta       `json:"meta"`
	AssetCtxs []SpotAssetCtx `json:"assetCtxs"`
}

// Position holds a single perp position.
type Position struct {
	Coin           string `json:"coin"`
	Szi            string `json:"szi"`
	EntryPx        string `json:"entryPx"`
	PositionValue  string `json:"positionValue"`
	UnrealizedPnl  string `json:"unrealizedPnl"`
	ReturnOnEquity string `json:"returnOnEquity"`
	Leverage       Leverage `json:"leverage"`
	LiquidationPx  string `json:"liquidationPx"`
	MarginUsed     string `json:"marginUsed"`
	MaxTradeSzs    []string `json:"maxTradeSzs"`
	CumFunding     CumFunding `json:"cumFunding"`
}

// Leverage describes the leverage setting for a position.
type Leverage struct {
	Type      string `json:"type"`
	Value     int    `json:"value"`
	RawUsd    string `json:"rawUsd,omitempty"`
}

// CumFunding holds cumulative funding info.
type CumFunding struct {
	AllTime   string `json:"allTime"`
	SinceOpen string `json:"sinceOpen"`
	SinceChange string `json:"sinceChange"`
}

// AssetPosition wraps a Position.
type AssetPosition struct {
	Position Position `json:"position"`
	Type     string   `json:"type"`
}

// MarginSummary holds margin account summary.
type MarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalNtlPos     string `json:"totalNtlPos"`
	TotalRawUsd     string `json:"totalRawUsd"`
	TotalMarginUsed string `json:"totalMarginUsed"`
}

// UserState is the response for clearinghouseState.
type UserState struct {
	AssetPositions        []AssetPosition `json:"assetPositions"`
	CrossMaintenanceMarginUsed string     `json:"crossMaintenanceMarginUsed"`
	CrossMarginSummary    MarginSummary   `json:"crossMarginSummary"`
	MarginSummary         MarginSummary   `json:"marginSummary"`
	Withdrawable          string          `json:"withdrawable"`
}

// SpotBalance holds a single spot token balance.
type SpotBalance struct {
	Coin     string `json:"coin"`
	Token    int    `json:"token"`
	Hold     string `json:"hold"`
	Total    string `json:"total"`
	EntryNtl string `json:"entryNtl"`
}

// SpotUserState is the response for spotClearinghouseState.
type SpotUserState struct {
	Balances []SpotBalance `json:"balances"`
}

// Level represents a single price level in an order book.
type Level struct {
	Px string `json:"px"`
	Sz string `json:"sz"`
	N  int    `json:"n"`
}

// L2Book is the response for l2Book.
type L2Book struct {
	Coin   string    `json:"coin"`
	Levels [2][]Level `json:"levels"`
	Time   int64     `json:"time"`
}

// OpenOrder represents an open order.
type OpenOrder struct {
	Coin      string      `json:"coin"`
	Side      string      `json:"side"`
	LimitPx   string      `json:"limitPx"`
	Sz        string      `json:"sz"`
	Oid       int64       `json:"oid"`
	Timestamp int64       `json:"timestamp"`
	OrigSz    string      `json:"origSz"`
	Cloid     interface{} `json:"cloid"`
	OrderType OrderType   `json:"orderType"`
}

// OrderType describes the type of an order.
type OrderType struct {
	Limit  *LimitOrder  `json:"limit,omitempty"`
	Trigger *TriggerOrder `json:"trigger,omitempty"`
}

// LimitOrder describes a limit order's time-in-force.
type LimitOrder struct {
	Tif string `json:"tif"`
}

// TriggerOrder describes a trigger order.
type TriggerOrder struct {
	IsMarket   bool   `json:"isMarket"`
	TriggerPx  string `json:"triggerPx"`
	TpSl       string `json:"tpsl"`
}

// Fill represents a single trade fill.
type Fill struct {
	Coin        string `json:"coin"`
	Px          string `json:"px"`
	Sz          string `json:"sz"`
	Side        string `json:"side"`
	Time        int64  `json:"time"`
	StartPosition string `json:"startPosition"`
	Dir         string `json:"dir"`
	ClosedPnl   string `json:"closedPnl"`
	Hash        string `json:"hash"`
	Oid         int64  `json:"oid"`
	Crossed     bool   `json:"crossed"`
	Fee         string `json:"fee"`
	TID         int64  `json:"tid"`
	Cloid       interface{} `json:"cloid"`
}

// OrderStatus is the response for orderByOid.
type OrderStatus struct {
	Order  OpenOrder `json:"order"`
	Status string    `json:"status"`
	StatusTimestamp int64 `json:"statusTimestamp"`
}

// EIP712Domain is the EIP-712 domain for Hyperliquid exchange actions.
type EIP712Domain struct {
	ChainID           int    `json:"chainId"`
	Name              string `json:"name"`
	VerifyingContract string `json:"verifyingContract"`
	Version           string `json:"version"`
}

// EIP712Types holds the EIP-712 type definitions for an exchange action.
type EIP712Types struct {
	Primary    string                       `json:"primary"`
	Definition map[string][]EIP712TypeField `json:"definition"`
}

// EIP712TypeField is a single field in an EIP-712 type definition.
type EIP712TypeField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ActionPayload is the unsigned exchange action payload returned by builder methods.
type ActionPayload struct {
	Action      interface{}  `json:"action"`
	EIP712Domain EIP712Domain `json:"eip712_domain"`
	EIP712Types  EIP712Types  `json:"eip712_types"`
}

// OrderRequest describes a single order in a place-order action.
type OrderRequest struct {
	// Asset index (coin index from Meta universe).
	A int `json:"a"`
	// IsBuy: true = buy, false = sell.
	B bool `json:"b"`
	// LimitPx is the limit price as a string.
	P string `json:"p"`
	// Size is the order size as a string.
	S string `json:"s"`
	// ReduceOnly.
	R bool `json:"r"`
	// Order type.
	T OrderType `json:"t"`
	// Cloid is an optional client order id.
	C interface{} `json:"c,omitempty"`
}

// placeOrderAction is the inner action for placing orders.
type placeOrderAction struct {
	Type     string         `json:"type"`
	Orders   []OrderRequest `json:"orders"`
	Grouping string         `json:"grouping"`
}

// cancelAction is the inner action for cancelling orders.
type cancelAction struct {
	Type    string        `json:"type"`
	Cancels []CancelOrder `json:"cancels"`
}

// CancelOrder identifies an order to cancel.
type CancelOrder struct {
	A   int   `json:"a"`
	Oid int64 `json:"oid"`
}

// modifyAction is the inner action for modifying orders.
type modifyAction struct {
	Type    string          `json:"type"`
	Modifies []ModifyOrder  `json:"modifies"`
}

// ModifyOrder describes a modification to an existing order.
type ModifyOrder struct {
	Oid   int64        `json:"oid"`
	Order OrderRequest `json:"order"`
}

// updateLeverageAction is the inner action for updating leverage.
type updateLeverageAction struct {
	Type     string `json:"type"`
	Asset    int    `json:"asset"`
	IsCross  bool   `json:"isCross"`
	Leverage int    `json:"leverage"`
}

// usdTransferAction is the inner action for USD transfers.
type usdTransferAction struct {
	Type        string `json:"type"`
	Destination string `json:"destination"`
	Amount      string `json:"amount"`
}

var hyperliquidEIP712Domain = EIP712Domain{
	ChainID:           42161,
	Name:              "HyperliquidSignTransaction",
	VerifyingContract: "0x0000000000000000000000000000000000000000",
	Version:           "1",
}
