---
name: Polymarket Trading
description: Discover, trade, and manage prediction market positions on Polymarket via Polygon
tags: [polymarket, prediction-market, polygon, evm, trading, betting]
---

# Polymarket Trading

Trade prediction markets on Polymarket — a hybrid-decentralized platform on Polygon (Chain ID 137). Markets resolve to $1 (correct) or $0 (incorrect).

**USDC.e (bridged USDC) is the collateral token** — NOT native USDC.
- Contract: `0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174`
- "$3", "3 USDC", "3 dollars" → 3 USDC.e on Polygon
- Check balance: `evm_get_token_balance` with chain=Polygon, contract_address above

## Contracts (Polygon)

| Contract | Address |
|----------|---------|
| CTF Exchange | `0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E` |
| Neg Risk CTF Exchange | `0xC5d563A36AE78145C45a50134d48A1215220f80a` |
| Neg Risk Adapter | `0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296` |
| Conditional Tokens (ERC1155) | `0x4D97DCd97eC945f40cF65F87097ACe5EA0476045` |
| USDC.e | `0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174` |

## Rules

1. **ONE action per response.** Never batch multiple `build_custom_tx` or `polymarket_sign_bet`.
2. **spend vs amount.** "$X" / "X dollars" / "bet X" → `spend`. "X shares" → `amount`. Server calculates shares.
3. **Don't re-emit approval transactions** after `all_approved: true` this session. Still call `polymarket_check_approvals` (step 2) to verify balance, but skip step 3 if already approved.
4. **No fabricated slugs.** `event_slug` MUST come from `polymarket_search`. Never guess/recall.
5. **Multi-outcome events:** pass candidate name (e.g. "Oklahoma City Thunder"), NOT "Yes".
6. **Don't ask when intent is clear.** "Bet $2 on Thunder" → execute, don't ask "are you sure?"
7. **NEVER** call `polymarket_build_order` or `polymarket_submit_order` — use `polymarket_place_bet` + `polymarket_sign_bet`.
8. **NEVER** construct `sign_typed_data` for Polymarket — use `polymarket_sign_bet`.
9. **Minimum order: $1.** Orders under $1 are rejected.

## Bet Workflow (MANDATORY — follow in order)

1. **SEARCH** → `polymarket_search` — get event_slug, prices, outcomes
2. **CHECK** → `polymarket_check_approvals` — returns USDC.e balance + missing approvals. **Tell user their balance.** If balance < bet, stop.
3. **FIX APPROVALS** → if `missing_count > 0`, emit each `missing_actions` entry as `build_custom_tx` → wait for `sign_tx`. One at a time. Do NOT modify params.
4. **PLACE** → `polymarket_place_bet` with `event_slug`, `outcome`, `side`, `price`, `spend`
5. **SHOW** → display summary from result
6. **SIGN** → emit `polymarket_sign_bet` action with `order_ref` from result (copy verbatim)
7. **REPORT** → order auto-submits after signing. Report result from action result message:
   - `matched` → "Order filled!" then call `polymarket_positions` to show new position
   - `matched` (partial, FAK) → "Filled X of Y shares. Rest cancelled (limited liquidity)." then show position
   - `live` → "Limit order placed on the book."
   - `delayed` → "Processing..." then check `polymarket_open_orders`
   - `unmatched` → explain why, suggest adjusted price or retry with GTC

**NEVER skip steps 1-2.** Slug must come from search. Balance must be checked.

## Order Types

| Type | Behavior | Use when |
|------|----------|----------|
| FAK | Fills what's available, cancels unfilled remainder. Immediate. | **Default.** Most user bets. |
| GTC | Rests on book at exact price until filled/cancelled. | User says "limit order" or "at price X". |
| FOK | Must fill 100% or cancel entirely. | "All or nothing." |
| GTD | Like GTC but expires at a time. | "Cancel if not filled by Friday." |

**FAK is the default.** It fills whatever liquidity exists and cancels the rest — no failed orders on thin books. Use GTC only when the user explicitly asks for a limit order. After a FAK fill, tell the user how much was filled (e.g. "Filled 3.2 of 5.6 shares at $0.17").

## Selling

Same workflow as buying but with `side: SELL`. User must hold outcome tokens.

1. Check position: `polymarket_positions`
2. `polymarket_place_bet` with `side: SELL`
3. Show summary → `polymarket_sign_bet` → auto-submitted

## Discovery

- `polymarket_search` — primary discovery. Returns tradeable markets with slugs, prices, outcomes.
- `polymarket_market_info` — browse a specific event's sub-markets (supports `question_contains`, pagination)
- `polymarket_price` — current probability (optional — search already returns prices)
- `polymarket_orderbook` — liquidity at each price level

**Don't re-fetch data you already have.** After search returns results, go straight to `polymarket_place_bet`.

## Token ID Selection

Always use `event_slug` + `outcome` on `polymarket_place_bet`. Server resolves the CLOB token ID. Never manually pass token IDs.

## Managing Orders

- `polymarket_open_orders` — list active orders. Pass `address` only — auth is cached.
- `polymarket_cancel_order` — cancel by order ID. Pass `order_id` + `address`.

Do NOT fabricate `auth_signature` or `auth_timestamp` for these tools.

## Positions & History

- `polymarket_positions` — holdings with entry price, current price, P&L
- `polymarket_trades` — historical trade log

Display positions as: market/outcome, shares, current value, entry cost (shares × avgPrice), P&L in $ and % (from `pnlPercent`). Clean summary, not raw JSON.

## Approvals (One-time)

`polymarket_check_approvals` checks all 6 approvals + balance in one call.

### USDC.e approvals (spender = each address)

| Spender | Address | When |
|---------|---------|------|
| CTF Exchange | `0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E` | All markets |
| Neg Risk CTF Exchange | `0xC5d563A36AE78145C45a50134d48A1215220f80a` | negRisk (3+ outcomes) |
| Neg Risk Adapter | `0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296` | negRisk (3+ outcomes) |

### Conditional Tokens approvals (setApprovalForAll, operator = each address)

| Operator | Address | When |
|----------|---------|------|
| CTF Exchange | `0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E` | Selling |
| Neg Risk CTF Exchange | `0xC5d563A36AE78145C45a50134d48A1215220f80a` | Selling on negRisk |
| Neg Risk Adapter | `0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296` | Selling on negRisk |

**Spender is the EXCHANGE, never the Conditional Tokens contract.**

## Fees

`fee = feeRateBps / 10000 × min(price, 1 - price) × size`

- ~200 bps (2%) base, varies per market — fetched dynamically by `polymarket_place_bet`
- Always show estimated fee + total cost before placing

## NegRisk Markets

Markets with 3+ outcomes use Neg Risk CTF Exchange. `polymarket_place_bet` auto-detects. Requires approvals for all three contracts.

## Redeeming (After Resolution)

Use `build_custom_tx` with `tx_type: evm_contract`, `chain: Polygon`, contract: Conditional Tokens, `function_name: redeemPositions`.

## Error Recovery

| Error | Resolution |
|-------|-----------|
| Insufficient USDC.e | "Need X more USDC.e on Polygon. Want to swap?" |
| not enough balance / allowance | Check USDC.e balance, verify approvals for correct exchange, cancel stale orders |
| Market closed | "This market has closed." |
| Below $1 minimum | "Minimum order is $1." |
| Token resolution failed | Check outcomes with `polymarket_search` |
| "pass the candidate NAME" | Multi-outcome → pass person/option name, not "Yes"/"No" |
| Partial fill (FAK) | Tell user exactly how much filled: "Filled X of Y shares. The rest was cancelled due to limited liquidity at this price." |
| "couldn't be fully filled" (FOK) | Market lacks depth. Retry with FAK for partial fill, or GTC to rest on book. |
