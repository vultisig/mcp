// Package hyperliquid provides a REST client for the Hyperliquid Info and Exchange APIs.
package hyperliquid

// infoRequest is the common wrapper for all POST /info requests.
type infoRequest struct {
	Type    string `json:"type"`
	User    string `json:"user,omitempty"`
	Coin    string `json:"coin,omitempty"`
	OID     uint64 `json:"oid,omitempty"`
	NCandle int    `json:"nCandle,omitempty"`
}

// orderByOidRequest is used for QueryOrderByOid which needs both user and oid.
type orderByOidRequest struct {
	Type string `json:"type"`
	User string `json:"user"`
	OID  uint64 `json:"oid"`
}

// --- Info response types ---

// AssetCtx holds perpetual market context for one asset.
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

// Universe describes a single perpetual market.
type Universe struct {
	Name          string   `json:"name"`
	SzDecimals    int      `json:"szDecimals"`
	MaxLeverage   int      `json:"maxLeverage"`
	Fractionality bool     `json:"onlyIsolated"`
}

// Meta holds the perpetual universe metadata.
type Meta struct {
	Universe []Universe `json:"universe"`
}

// MetaAndAssetCtxsResponse is returned by GetMetaAndAssetCtxs.
// Index 0 is Meta, index 1 is []AssetCtx (raw JSON arrays).
type MetaAndAssetCtxsResponse struct {
	Meta      Meta       `json:"-"`
	AssetCtxs []AssetCtx `json:"-"`
	Raw       []any      `json:"-"`
}

// SpotToken describes a spot token.
type SpotToken struct {
	Name        string `json:"name"`
	SzDecimals  int    `json:"szDecimals"`
	WeiDecimals int    `json:"weiDecimals"`
	Index       int    `json:"index"`
	TokenID     string `json:"tokenId"`
	IsCanonical bool   `json:"isCanonical"`
}

// SpotMarket describes a spot trading pair.
type SpotMarket struct {
	Name    string `json:"name"`
	Tokens  [2]int `json:"tokens"`
	Index   int    `json:"index"`
	IsCanonical bool `json:"isCanonical"`
}

// SpotMeta holds spot universe metadata.
type SpotMeta struct {
	Tokens   []SpotToken  `json:"tokens"`
	Universe []SpotMarket `json:"universe"`
}

// SpotAssetCtx holds spot market context.
type SpotAssetCtx struct {
	PrevDayPx string `json:"prevDayPx"`
	DayNtlVlm string `json:"dayNtlVlm"`
	MarkPx    string `json:"markPx"`
	MidPx     string `json:"midPx"`
	CirculatingSupply string `json:"circulatingSupply"`
}

// SpotMetaAndAssetCtxsResponse is returned by GetSpotMetaAndAssetCtxs.
type SpotMetaAndAssetCtxsResponse struct {
	Meta      SpotMeta       `json:"-"`
	AssetCtxs []SpotAssetCtx `json:"-"`
}

// PerpPosition holds a user's perpetual position.
type PerpPosition struct {
	Coin           string `json:"coin"`
	Szi            string `json:"szi"`
	LeverageType   string `json:"leverageType"`
	Leverage       int    `json:"leverage"`
	EntryPx        string `json:"entryPx"`
	PositionValue  string `json:"positionValue"`
	UnrealizedPnl  string `json:"unrealizedPnl"`
	ReturnOnEquity string `json:"returnOnEquity"`
	LiquidationPx  string `json:"liquidationPx"`
	MarginUsed     string `json:"marginUsed"`
	MaxLeverage    int    `json:"maxLeverage"`
}

// AssetPosition wraps a PerpPosition.
type AssetPosition struct {
	Position PerpPosition `json:"position"`
	Type     string       `json:"type"`
}

// MarginSummary summarizes a user's margin state.
type MarginSummary struct {
	AccountValue    string `json:"accountValue"`
	TotalNtlPos     string `json:"totalNtlPos"`
	TotalRawUsd     string `json:"totalRawUsd"`
	TotalMarginUsed string `json:"totalMarginUsed"`
}

// UserState is returned by GetUserState.
type UserState struct {
	AssetPositions        []AssetPosition `json:"assetPositions"`
	MarginSummary         MarginSummary   `json:"marginSummary"`
	CrossMarginSummary    MarginSummary   `json:"crossMarginSummary"`
	CrossMaintenanceMarginUsed string     `json:"crossMaintenanceMarginUsed"`
	Withdrawable          string          `json:"withdrawable"`
	Time                  int64           `json:"time"`
}

// SpotBalance holds a user's spot token balance.
type SpotBalance struct {
	Coin     string `json:"coin"`
	Token    int    `json:"token"`
	Hold     string `json:"hold"`
	Total    string `json:"total"`
	EntryNtl string `json:"entryNtl"`
}

// SpotUserState is returned by GetSpotUserState.
type SpotUserState struct {
	Balances []SpotBalance `json:"balances"`
}

// Level represents one price level in the order book.
type Level struct {
	Px string `json:"px"`
	Sz string `json:"sz"`
	N  int    `json:"n"`
}

// L2Book is returned by GetL2Book.
type L2Book struct {
	Coin   string    `json:"coin"`
	Time   int64     `json:"time"`
	Levels [][]Level `json:"levels"`
}

// OpenOrder represents an open resting order.
type OpenOrder struct {
	Coin      string `json:"coin"`
	Side      string `json:"side"`
	LimitPx   string `json:"limitPx"`
	Sz        string `json:"sz"`
	OID       uint64 `json:"oid"`
	Timestamp int64  `json:"timestamp"`
	OrigSz    string `json:"origSz"`
}

// UserFill represents a completed trade fill.
type UserFill struct {
	Coin        string `json:"coin"`
	Px          string `json:"px"`
	Sz          string `json:"sz"`
	Side        string `json:"side"`
	Time        int64  `json:"time"`
	StartPosition string `json:"startPosition"`
	Dir         string `json:"dir"`
	ClosedPnl   string `json:"closedPnl"`
	Hash        string `json:"hash"`
	OID         uint64 `json:"oid"`
	Crossed     bool   `json:"crossed"`
	Fee         string `json:"fee"`
	TID         uint64 `json:"tid"`
	FeeToken    string `json:"feeToken"`
}

// OrderStatus represents the status of an order queried by OID.
type OrderStatus struct {
	Order  OpenOrder `json:"order"`
	Status string    `json:"status"`
}

// --- Exchange action types ---

// EIP712Domain is the EIP-712 domain for Hyperliquid on Arbitrum.
type EIP712Domain struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	ChainID           int    `json:"chainId"`
	VerifyingContract string `json:"verifyingContract"`
}

// EIP712Types holds the EIP-712 type definitions for a Hyperliquid action.
type EIP712Types map[string][]EIP712TypeField

// EIP712TypeField is one field in an EIP-712 type definition.
type EIP712TypeField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ActionPayload is the unsigned payload returned by exchange action builders.
// The backend (agent-backend) is responsible for EIP-712 signing.
type ActionPayload struct {
	Action       any          `json:"action"`
	EIP712Domain EIP712Domain `json:"eip712_domain"`
	EIP712Types  EIP712Types  `json:"eip712_types"`
}

// hyperliquidDomain returns the canonical EIP-712 domain for Hyperliquid on Arbitrum mainnet.
func hyperliquidDomain() EIP712Domain {
	return EIP712Domain{
		Name:              "HyperliquidSignTransaction",
		Version:           "1",
		ChainID:           42161,
		VerifyingContract: "0x0000000000000000000000000000000000000000",
	}
}

// --- Order action structs ---

// OrderLimit holds limit order parameters.
type OrderLimit struct {
	Tif string `json:"tif"` // "Gtc", "Ioc", "Alo"
}

// OrderTrigger holds stop/tp order parameters.
type OrderTrigger struct {
	IsMarket  bool   `json:"isMarket"`
	TriggerPx string `json:"triggerPx"`
	Tpsl      string `json:"tpsl"` // "tp" or "sl"
}

// OrderType holds either a limit or trigger order type.
type OrderType struct {
	Limit   *OrderLimit   `json:"limit,omitempty"`
	Trigger *OrderTrigger `json:"trigger,omitempty"`
}

// OrderRequest is one order within a place-order action.
type OrderRequest struct {
	A int       `json:"a"`  // asset index
	B bool      `json:"b"`  // isBuy
	P string    `json:"p"`  // price
	S string    `json:"s"`  // size
	R bool      `json:"r"`  // reduceOnly
	T OrderType `json:"t"`  // order type
	C string    `json:"c,omitempty"` // cloid (optional)
}

// PlaceOrderAction is sent to the exchange to place one or more orders.
type PlaceOrderAction struct {
	Type     string         `json:"type"`
	Orders   []OrderRequest `json:"orders"`
	Grouping string         `json:"grouping"` // "na", "normalTpsl", "positionTpsl"
}

// CancelRequest identifies an order to cancel.
type CancelRequest struct {
	A   int    `json:"a"`   // asset index
	OID uint64 `json:"oid"` // order ID
}

// CancelAction cancels one or more orders.
type CancelAction struct {
	Type    string          `json:"type"`
	Cancels []CancelRequest `json:"cancels"`
}

// ModifyRequest holds the new parameters for an existing order.
type ModifyRequest struct {
	OID   uint64       `json:"oid"`
	Order OrderRequest `json:"order"`
}

// ModifyAction modifies one or more existing orders.
type ModifyAction struct {
	Type    string          `json:"type"`
	Modifies []ModifyRequest `json:"modifies"`
}

// UpdateLeverageAction sets leverage for an asset.
type UpdateLeverageAction struct {
	Type      string `json:"type"`
	Asset     int    `json:"asset"`
	IsCross   bool   `json:"isCross"`
	Leverage  int    `json:"leverage"`
}

// UsdTransferAction transfers USD to another address.
type UsdTransferAction struct {
	Type        string `json:"type"`
	Hyperliquid bool   `json:"hyperliquid"`
	Amount      string `json:"amount"`
	Destination string `json:"destination"`
}

// --- Action builder functions ---

// BuildPlaceOrderAction returns an unsigned ActionPayload for placing orders.
func BuildPlaceOrderAction(orders []OrderRequest, grouping string) ActionPayload {
	if grouping == "" {
		grouping = "na"
	}
	action := PlaceOrderAction{
		Type:     "order",
		Orders:   orders,
		Grouping: grouping,
	}
	return ActionPayload{
		Action:       action,
		EIP712Domain: hyperliquidDomain(),
		EIP712Types: EIP712Types{
			"Agent": {
				{Name: "source", Type: "string"},
				{Name: "connectionId", Type: "bytes32"},
			},
		},
	}
}

// BuildCancelAction returns an unsigned ActionPayload for cancelling orders.
func BuildCancelAction(cancels []CancelRequest) ActionPayload {
	action := CancelAction{
		Type:    "cancel",
		Cancels: cancels,
	}
	return ActionPayload{
		Action:       action,
		EIP712Domain: hyperliquidDomain(),
		EIP712Types: EIP712Types{
			"Agent": {
				{Name: "source", Type: "string"},
				{Name: "connectionId", Type: "bytes32"},
			},
		},
	}
}

// BuildModifyAction returns an unsigned ActionPayload for modifying orders.
func BuildModifyAction(modifies []ModifyRequest) ActionPayload {
	action := ModifyAction{
		Type:     "batchModify",
		Modifies: modifies,
	}
	return ActionPayload{
		Action:       action,
		EIP712Domain: hyperliquidDomain(),
		EIP712Types: EIP712Types{
			"Agent": {
				{Name: "source", Type: "string"},
				{Name: "connectionId", Type: "bytes32"},
			},
		},
	}
}

// BuildUpdateLeverageAction returns an unsigned ActionPayload for setting leverage.
func BuildUpdateLeverageAction(asset int, isCross bool, leverage int) ActionPayload {
	action := UpdateLeverageAction{
		Type:     "updateLeverage",
		Asset:    asset,
		IsCross:  isCross,
		Leverage: leverage,
	}
	return ActionPayload{
		Action:       action,
		EIP712Domain: hyperliquidDomain(),
		EIP712Types: EIP712Types{
			"Agent": {
				{Name: "source", Type: "string"},
				{Name: "connectionId", Type: "bytes32"},
			},
		},
	}
}

// BuildUsdTransferAction returns an unsigned ActionPayload for a USD transfer.
func BuildUsdTransferAction(amount, destination string) ActionPayload {
	action := UsdTransferAction{
		Type:        "usdSend",
		Hyperliquid: true,
		Amount:      amount,
		Destination: destination,
	}
	return ActionPayload{
		Action:       action,
		EIP712Domain: hyperliquidDomain(),
		EIP712Types: EIP712Types{
			"UsdSend": {
				{Name: "hyperliquidChain", Type: "string"},
				{Name: "destination", Type: "string"},
				{Name: "amount", Type: "string"},
				{Name: "time", Type: "uint64"},
			},
		},
	}
}
