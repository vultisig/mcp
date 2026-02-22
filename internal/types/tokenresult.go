package types

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// TokenSearchResult is the top-level JSON envelope returned by find_token.
type TokenSearchResult struct {
	Tokens []TokenInfo `json:"tokens"`
}

// TokenInfo describes a single token with all known chain deployments.
type TokenInfo struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Symbol        string            `json:"symbol"`
	MarketCapRank int               `json:"market_cap_rank"`
	Logo          string            `json:"logo"`
	Deployments   []TokenDeployment `json:"deployments"`
}

// TokenDeployment represents one on-chain deployment of a token.
type TokenDeployment struct {
	Chain           string `json:"chain"`
	ContractAddress string `json:"contract_address"`
	Decimals        int    `json:"decimals"`
}

// ToToolResult serialises the TokenSearchResult as JSON and wraps it in an
// MCP tool result.
func (r *TokenSearchResult) ToToolResult() (*mcp.CallToolResult, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("marshal token search result: %w", err)
	}
	return mcp.NewToolResultText(string(data)), nil
}
