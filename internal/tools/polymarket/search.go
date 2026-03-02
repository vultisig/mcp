package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	pm "github.com/vultisig/mcp/internal/polymarket"
)

const (
	maxSearchResults   = 5
	maxMarketsPerEvent = 5
)

func NewSearchTool() mcp.Tool {
	return mcp.NewTool("polymarket_search",
		mcp.WithDescription(
			"Search for Polymarket prediction markets by topic or keyword. "+
				"Returns top 5 events with their tradeable markets, prices, CLOB token IDs, and volume. "+
				"Use question_contains to find a specific outcome within large events (e.g. 'Rubio' in a 100-outcome presidential market). "+
				"Results include everything needed for polymarket_place_bet — no need to call polymarket_market_info separately. "+
				"Load the 'polymarket-trading' skill before placing any trades.",
		),
		mcp.WithString("query",
			mcp.Description("Search query (e.g. 'Trump', 'Bitcoin', 'election')."),
			mcp.Required(),
		),
		mcp.WithBoolean("active_only",
			mcp.Description("Only show active (open) markets. Default true."),
		),
		mcp.WithString("question_contains",
			mcp.Description("Filter sub-markets whose question contains this text (case-insensitive). Use to find specific outcomes in large events. Applied before the per-event market limit."),
		),
	)
}

type searchMarketSummary struct {
	Question      string   `json:"question"`
	Outcomes      []string `json:"outcomes"`
	OutcomePrices []string `json:"outcome_prices"`
	ClobTokenIDs  []string `json:"clob_token_ids"`
	Active        bool     `json:"active"`
	Volume        string   `json:"volume"`
}

type searchEventSummary struct {
	Slug        string                `json:"slug"`
	Title       string                `json:"title"`
	Active      bool                  `json:"active"`
	Volume      float64               `json:"volume"`
	EndDate     string                `json:"end_date,omitempty"`
	Markets     []searchMarketSummary `json:"markets"`
	MoreMarkets int                   `json:"more_markets,omitempty"`
}

func summarizeEvents(events []pm.Event, questionFilter string) []searchEventSummary {
	if len(events) > maxSearchResults {
		events = events[:maxSearchResults]
	}
	qLower := strings.ToLower(questionFilter)

	out := make([]searchEventSummary, 0, len(events))
	for _, e := range events {
		tradable := make([]pm.Market, 0, len(e.Markets))
		for _, m := range e.Markets {
			if m.Closed || !m.Active {
				continue
			}
			if len(m.ClobTokenIDs) == 0 {
				continue
			}
			if liq, _ := strconv.ParseFloat(m.Liquidity, 64); liq <= 0 {
				continue
			}
			tradable = append(tradable, m)
		}

		if qLower != "" {
			filtered := tradable[:0]
			for _, m := range tradable {
				if strings.Contains(strings.ToLower(m.Question), qLower) {
					filtered = append(filtered, m)
				}
			}
			tradable = filtered
		}

		if len(tradable) == 0 {
			continue
		}

		se := searchEventSummary{
			Slug:    e.Slug,
			Title:   e.Title,
			Active:  e.Active,
			Volume:  e.Volume,
			EndDate: e.EndDate,
		}

		showing := tradable
		if len(showing) > maxMarketsPerEvent {
			showing = showing[:maxMarketsPerEvent]
		}
		for _, m := range showing {
			se.Markets = append(se.Markets, searchMarketSummary{
				Question:      m.Question,
				Outcomes:      m.Outcomes,
				OutcomePrices: m.OutcomePrices,
				ClobTokenIDs:  m.ClobTokenIDs,
				Active:        m.Active,
				Volume:        m.Volume,
			})
		}
		if len(tradable) > maxMarketsPerEvent {
			se.MoreMarkets = len(tradable) - maxMarketsPerEvent
		}
		out = append(out, se)
	}
	return out
}

func HandleSearch(pmClient *pm.Client) server.ToolHandlerFunc {
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

		questionFilter := req.GetString("question_contains", "")
		summary := summarizeEvents(events, questionFilter)
		if len(summary) == 0 {
			msg := "No tradeable markets found for query: " + query
			if questionFilter != "" {
				msg += " (with question filter: " + questionFilter + ")"
			}
			return mcp.NewToolResultText(msg), nil
		}
		data, err := json.Marshal(summary)
		if err != nil {
			return nil, fmt.Errorf("marshal events: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
