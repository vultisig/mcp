package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/polymarket"
)

const (
	maxDescriptionLen      = 200
	defaultMarketsPageSize = 10
)

func newPolymarketMarketInfoTool() mcp.Tool {
	return mcp.NewTool("polymarket_market_info",
		mcp.WithDescription(
			"Get detailed information about a specific Polymarket event or market. "+
				"Returns market question, outcomes, prices, CLOB token IDs, volume, and liquidity. "+
				"Use event slug to get full event with all markets, or market_id for a specific market. "+
				"For events with many sub-markets, use offset to paginate through them. "+
				"For multi-outcome events, load 'polymarket-trading' skill for token ID selection rules.",
		),
		mcp.WithString("slug",
			mcp.Description("Event slug (e.g. 'democratic-presidential-nominee-2028'). Returns the event with its markets."),
		),
		mcp.WithString("market_id",
			mcp.Description("Numeric market ID (e.g. '559657') or market slug. Returns a single market."),
		),
		mcp.WithNumber("offset",
			mcp.Description("Skip this many sub-markets when returning event markets. Default 0. Use to paginate large events."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of sub-markets to return per event. Default 10."),
		),
		mcp.WithString("question_contains",
			mcp.Description("Filter sub-markets whose question contains this text (case-insensitive). Applied before pagination."),
		),
	)
}

func truncateDescription(s string) string {
	if len(s) <= maxDescriptionLen {
		return s
	}
	return s[:maxDescriptionLen] + "..."
}

type eventInfoResponse struct {
	polymarket.Event
	TotalMarkets int `json:"total_markets"`
	Offset       int `json:"offset"`
	Showing      int `json:"showing"`
}

func handlePolymarketMarketInfo(pmClient *polymarket.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug := req.GetString("slug", "")
		marketID := req.GetString("market_id", "")

		if slug == "" && marketID == "" {
			return mcp.NewToolResultError("provide either slug or market_id"), nil
		}

		if slug != "" {
			event, err := pmClient.GetEvent(ctx, slug)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("event lookup failed: %v", err)), nil
			}
			event.Description = truncateDescription(event.Description)

			// Filter non-tradable markets (before pagination)
			tradable := event.Markets[:0]
			for _, m := range event.Markets {
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
			event.Markets = tradable

			// Filter by question text (before pagination)
			if qFilter := req.GetString("question_contains", ""); qFilter != "" {
				qLower := strings.ToLower(qFilter)
				filtered := event.Markets[:0]
				for _, m := range event.Markets {
					if strings.Contains(strings.ToLower(m.Question), qLower) {
						filtered = append(filtered, m)
					}
				}
				event.Markets = filtered
			}

			offset := int(req.GetFloat("offset", 0))
			limit := int(req.GetFloat("limit", defaultMarketsPageSize))
			if limit <= 0 {
				limit = defaultMarketsPageSize
			}

			total := len(event.Markets)
			if offset >= total {
				event.Markets = nil
			} else {
				end := offset + limit
				if end > total {
					end = total
				}
				event.Markets = event.Markets[offset:end]
			}

			resp := eventInfoResponse{
				Event:        *event,
				TotalMarkets: total,
				Offset:       offset,
				Showing:      len(event.Markets),
			}

			data, err := json.Marshal(resp)
			if err != nil {
				return nil, fmt.Errorf("marshal event: %w", err)
			}
			return mcp.NewToolResultText(string(data)), nil
		}

		market, err := pmClient.GetMarket(ctx, marketID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("market lookup failed: %v", err)), nil
		}
		data, err := json.Marshal(market)
		if err != nil {
			return nil, fmt.Errorf("marshal market: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
