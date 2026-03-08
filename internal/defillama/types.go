package defillama

import "strings"

// ProtocolSummary is a single entry from the /protocols list endpoint.
type ProtocolSummary struct {
	Name     string   `json:"name"`
	Slug     string   `json:"slug"`
	TVL      float64  `json:"tvl"`
	Category string   `json:"category"`
	Chains   []string `json:"chains"`
	Change1d float64  `json:"change_1d"`
	Change7d float64  `json:"change_7d"`
}

// Protocol is the detailed response from /protocol/{slug}.
// Note: the API returns `tvl` as a time-series array, so we skip it
// and compute total TVL from `currentChainTvls`.
type Protocol struct {
	Name             string             `json:"name"`
	Slug             string             `json:"slug"`
	Category         string             `json:"category"`
	Chains           []string           `json:"chains"`
	Change1d         *float64           `json:"change_1d"`
	Change7d         *float64           `json:"change_7d"`
	CurrentChainTvls map[string]float64 `json:"currentChainTvls"`
	Description      string             `json:"description"`
	URL              string             `json:"url"`
}

// IsAggregateTVLKey returns true for keys in currentChainTvls that represent
// aggregated sub-categories rather than actual chains.
func IsAggregateTVLKey(name string) bool {
	if strings.Contains(name, "-") {
		return true
	}
	switch name {
	case "borrowed", "staking", "pool2", "vesting":
		return true
	default:
		return false
	}
}

// TotalTVL computes the total TVL by summing currentChainTvls,
// excluding borrowed/staking/pool2/vesting aggregations.
func (p *Protocol) TotalTVL() float64 {
	var total float64
	for k, v := range p.CurrentChainTvls {
		if IsAggregateTVLKey(k) {
			continue
		}
		total += v
	}
	return total
}

// Pool is a single yield pool from the yields.llama.fi/pools endpoint.
type Pool struct {
	Pool      string  `json:"pool"`
	Chain     string  `json:"chain"`
	Project   string  `json:"project"`
	Symbol    string  `json:"symbol"`
	TVLUsd    float64 `json:"tvlUsd"`
	APY       float64 `json:"apy"`
	APYBase   float64 `json:"apyBase"`
	APYReward float64 `json:"apyReward"`
	StableCoin bool   `json:"stablecoin"`
	ILRisk    string  `json:"ilRisk"`
}

// poolsResponse wraps the yield pools API response.
type poolsResponse struct {
	Data []Pool `json:"data"`
}

// ChainTVL is a single chain from the /v2/chains endpoint.
type ChainTVL struct {
	Name string  `json:"name"`
	TVL  float64 `json:"tvl"`
}
