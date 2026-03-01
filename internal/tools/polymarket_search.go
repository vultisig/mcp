package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/polymarket"
)

const (
	maxSearchResults    = 5
	maxMarketsPerEvent  = 5
)

func newPolymarketSearchTool() mcp.Tool {
	return mcp.NewTool("polymarket_search",
		mcp.WithDescription(
			"Search for Polymarket prediction markets by topic or keyword. "+
				"Returns top 5 events with their markets, prices, and volume. "+
				"Use this to discover tradeable prediction markets. "+
				"Load the 'polymarket-trading' skill before placing any trades.",
		),
		mcp.WithString("query",
			mcp.Description("Search query (e.g. 'Trump', 'Bitcoin', 'election')."),
			mcp.Required(),
		),
		mcp.WithBoolean("active_only",
			mcp.Description("Only show active (open) markets. Default true."),
		),
	)
}

// searchMarketSummary is a trimmed market for search results.
type searchMarketSummary struct {
	Question      string   `json:"question"`
	Outcomes      []string `json:"outcomes"`
	OutcomePrices []string `json:"outcome_prices"`
	ClobTokenIDs  []string `json:"clob_token_ids"`
	Active        bool     `json:"active"`
	Volume        string   `json:"volume"`
}

// searchEventSummary is a trimmed event for search results.
type searchEventSummary struct {
	Slug        string                `json:"slug"`
	Title       string                `json:"title"`
	Active      bool                  `json:"active"`
	Volume      float64               `json:"volume"`
	EndDate     string                `json:"end_date,omitempty"`
	Markets     []searchMarketSummary `json:"markets"`
	MoreMarkets int                   `json:"more_markets,omitempty"`
}

func summarizeEvents(events []polymarket.Event) []searchEventSummary {
	if len(events) > maxSearchResults {
		events = events[:maxSearchResults]
	}
	out := make([]searchEventSummary, 0, len(events))
	for _, e := range events {
		se := searchEventSummary{
			Slug:    e.Slug,
			Title:   e.Title,
			Active:  e.Active,
			Volume:  e.Volume,
			EndDate: e.EndDate,
		}
		markets := e.Markets
		if len(markets) > maxMarketsPerEvent {
			markets = markets[:maxMarketsPerEvent]
		}
		for _, m := range markets {
			se.Markets = append(se.Markets, searchMarketSummary{
				Question:      m.Question,
				Outcomes:      m.Outcomes,
				OutcomePrices: m.OutcomePrices,
				ClobTokenIDs:  m.ClobTokenIDs,
				Active:        m.Active,
				Volume:        m.Volume,
			})
		}
		if len(e.Markets) > maxMarketsPerEvent {
			se.MoreMarkets = len(e.Markets) - maxMarketsPerEvent
		}
		out = append(out, se)
	}
	return out
}

func handlePolymarketSearch(pmClient *polymarket.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}

		activeOnly := true
		if v, err := req.RequireBool("active_only"); err == nil {
			activeOnly = v
		}

		events, err := pmClient.SearchEvents(ctx, query, activeOnly)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}
		if len(events) == 0 {
			return mcp.NewToolResultText("No markets found for query: " + query), nil
		}

		summary := summarizeEvents(events)
		data, err := json.Marshal(summary)
		if err != nil {
			return nil, fmt.Errorf("marshal events: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
