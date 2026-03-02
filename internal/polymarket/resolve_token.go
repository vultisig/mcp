package polymarket

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ResolvedToken holds the result of token resolution from event_slug + outcome.
type ResolvedToken struct {
	TokenID   string `json:"token_id"`
	Question  string `json:"question"`
	Outcome   string `json:"outcome"`
	Price     string `json:"price"`
	EventSlug string `json:"event_slug"`
	NegRisk   bool   `json:"neg_risk"`
}

// ResolveToken finds the correct CLOB token ID from an event slug and outcome text.
//
// For negRisk events (3+ outcomes like "Who will win the election?"):
//   - Finds the sub-market whose question contains the outcome text (e.g., "Marco Rubio")
//   - Returns the "Yes" token (index 0) for that sub-market
//
// For binary events ("Will X happen?"):
//   - outcome should be "Yes" or "No" (case-insensitive)
//   - Returns the corresponding token
func (c *Client) ResolveToken(ctx context.Context, eventSlug, outcome string) (*ResolvedToken, error) {
	event, err := c.GetEvent(ctx, eventSlug)
	if err != nil {
		return nil, fmt.Errorf("event %q not found — use polymarket_search to get the correct event_slug, never guess: %w", eventSlug, err)
	}

	outLower := strings.ToLower(strings.TrimSpace(outcome))
	if outLower == "" {
		return nil, fmt.Errorf("outcome is required — specify the outcome to bet on (e.g. 'Yes', 'No', or a candidate name)")
	}

	// Filter to tradable markets
	var tradable []Market
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

	if len(tradable) == 0 {
		return nil, fmt.Errorf("no tradable markets in event %q", eventSlug)
	}

	// For negRisk events: find market whose question contains the outcome text
	// For binary events: match "yes"/"no" against outcomes list
	if len(tradable) > 1 || event.NegRisk {
		return c.resolveNegRisk(tradable, outLower, eventSlug)
	}
	return c.resolveBinary(tradable[0], outLower, eventSlug)
}

func (c *Client) resolveNegRisk(markets []Market, outLower, eventSlug string) (*ResolvedToken, error) {
	// Find market whose question contains the outcome text
	var match *Market
	for i, m := range markets {
		if strings.Contains(strings.ToLower(m.Question), outLower) {
			match = &markets[i]
			break
		}
	}
	// Fuzzy fallback: word-overlap matching
	if match == nil {
		bestScore := 0.0
		var bestIdx int
		for i, m := range markets {
			score := wordOverlapScore(outLower, strings.ToLower(m.Question))
			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}
		if bestScore >= 0.5 {
			match = &markets[bestIdx]
		}
	}

	if match == nil {
		// Special case: LLM passes "yes"/"no" on multi-outcome events
		if outLower == "yes" || outLower == "no" {
			return nil, fmt.Errorf(
				"This is a multi-outcome event with %d options — 'Yes' and 'No' don't exist as outcomes here. "+
					"Each option is its own market (e.g. \"%s\"). "+
					"Pass the specific option name as the outcome parameter. Available options: %s",
				len(markets), markets[0].Question, marketNames(markets, 10))
		}
		return nil, fmt.Errorf("outcome %q not found in event. Available markets: %s",
			outLower, marketNames(markets, 10))
	}

	if len(match.ClobTokenIDs) == 0 {
		return nil, fmt.Errorf("market %q has no CLOB token IDs", match.Question)
	}

	// For negRisk markets, "Yes" token is always index 0
	tokenID := match.ClobTokenIDs[0]
	price := ""
	if len(match.OutcomePrices) > 0 {
		price = match.OutcomePrices[0]
	}

	return &ResolvedToken{
		TokenID:   tokenID,
		Question:  match.Question,
		Outcome:   "Yes",
		Price:     price,
		EventSlug: eventSlug,
		NegRisk:   true,
	}, nil
}

// marketNames returns up to max market questions, with "... and N more" if truncated.
func marketNames(markets []Market, max int) string {
	names := make([]string, 0, max)
	for i, m := range markets {
		if i >= max {
			names = append(names, fmt.Sprintf("... and %d more", len(markets)-max))
			break
		}
		names = append(names, m.Question)
	}
	return strings.Join(names, "; ")
}

// wordOverlapScore returns the fraction of query words (len >= 3) found in text.
func wordOverlapScore(query, text string) float64 {
	queryWords := strings.Fields(query)
	hits := 0
	total := 0
	for _, w := range queryWords {
		if len(w) < 3 {
			continue
		}
		total++
		if strings.Contains(text, w) {
			hits++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func (c *Client) resolveBinary(market Market, outLower, eventSlug string) (*ResolvedToken, error) {
	// Match "yes"/"no" against the outcomes list
	idx := -1
	for i, o := range market.Outcomes {
		if strings.ToLower(o) == outLower {
			idx = i
			break
		}
	}
	if idx == -1 {
		return nil, fmt.Errorf("outcome %q not found. Available: %s",
			outLower, strings.Join([]string(market.Outcomes), ", "))
	}

	if idx >= len(market.ClobTokenIDs) {
		return nil, fmt.Errorf("no CLOB token for outcome index %d", idx)
	}

	tokenID := market.ClobTokenIDs[idx]
	price := ""
	if idx < len(market.OutcomePrices) {
		price = market.OutcomePrices[idx]
	}

	return &ResolvedToken{
		TokenID:   tokenID,
		Question:  market.Question,
		Outcome:   market.Outcomes[idx],
		Price:     price,
		EventSlug: eventSlug,
		NegRisk:   market.NegRisk,
	}, nil
}
