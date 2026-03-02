---
name: Polymarket Trading
description: Discover, trade, and manage prediction market positions on Polymarket via Polygon
tags: [polymarket, prediction-market, polygon, evm, trading, betting]
---

# Polymarket Trading

Trade prediction markets on Polymarket — a hybrid-decentralized platform on Polygon (Chain ID 137). Markets resolve to $1 (correct) or $0 (incorrect).

**IMPORTANT: Polymarket uses USDC.e (bridged USDC) as collateral — NOT native USDC.**
- USDC.e contract: `0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174`
- When user says "$3", "3 USDC", "3 dollars" — this means 3 USDC.e on Polygon
- To check balance: use `evm_get_token_balance` with chain=Polygon and contract_address=`0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174`
- The `polymarket_place_bet` result includes `usdc_e_balance` — always tell the user their balance

## Contracts (Polygon)

| Contract | Address |
|----------|---------|
| CTF Exchange | `0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E` |
| Neg Risk CTF Exchange | `0xC5d563A36AE78145C45a50134d48A1215220f80a` |
| Neg Risk Adapter | `0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296` |
| Conditional Tokens (ERC1155) | `0x4D97DCd97eC945f40cF65F87097ACe5EA0476045` |
| USDC.e | `0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174` |

## Critical Agent Rules

1. **ONE action per response.** Emit exactly ONE `build_custom_tx` or `polymarket_sign_bet` per response. Never batch multiple actions.
2. **spend vs amount.** When the user says "$X", "X dollars", or "bet X", use the `spend` parameter (dollars). Only use `amount` when the user explicitly says "X shares". The server calculates shares = spend / price.
3. **Don't re-check approvals.** After `polymarket_check_approvals` returns `all_approved: true`, do NOT call it again during this session.
4. **Approval flow.** For each `missing_actions` entry: emit `build_custom_tx` in one response, wait for the `sign_tx` response, then emit the next. One at a time.
5. **No fabricated slugs.** NEVER guess `event_slug`. Always use `polymarket_search` first to get valid slugs.
6. **No "Yes" on multi-outcome.** For multi-outcome events, always specify the exact outcome name (e.g. "Oklahoma City Thunder"), never "Yes".
7. **Do NOT ask unnecessary questions.** If the user's intent is clear ("bet $2 on Thunder"), execute the workflow. Don't ask "are you sure?" when the context is unambiguous.

## MANDATORY WORKFLOW — ALWAYS follow this order

**STEP 1: SEARCH** -> `polymarket_search` (REQUIRED before any bet)
**STEP 2: CHECK BALANCE + APPROVALS** -> `polymarket_check_approvals` (returns USDC.e balance + missing approval actions). **ALWAYS tell the user their USDC.e balance.** If balance < bet amount, tell the user and stop.
**STEP 3: FIX APPROVALS** -> if `missing_count > 0`, emit each `missing_actions` entry as `build_custom_tx` -> `sign_tx`. One at a time.
**STEP 4: PLACE BET** -> `polymarket_place_bet` with `event_slug` + `outcome` + `side` + `price` + `spend` FROM search results
**STEP 5: SHOW SUMMARY** -> display the summary from place_bet result
**STEP 6: SIGN** -> emit `polymarket_sign_bet` action with `order_ref` from place_bet result
**STEP 7: REPORT** -> after signing succeeds, the order is auto-submitted. Report the result from the action result message.

**NEVER skip Steps 1 or 2.** The event_slug MUST come from search results. Balance MUST be checked before placing any bet.

## Placing a Bet

1. Call `polymarket_search` — get event_slug (**MANDATORY first step**)
2. Call `polymarket_check_approvals` — if `missing_count > 0`, emit each `missing_actions` as `build_custom_tx` -> wait for `sign_tx` response. **One per response.**
3. Call `polymarket_place_bet` — pass `event_slug`, `outcome`, `side`, `price`, `spend` (or `amount`)
4. Show the user the `summary` from the result
5. Emit `polymarket_sign_bet` action with `order_ref` from the result
6. After signing succeeds, the order is auto-submitted. Report the result from the action result message.

### Key rules for placing bets

- **NEVER** call `polymarket_build_order` or `polymarket_submit_order` directly — use `polymarket_place_bet` instead
- **NEVER** construct `sign_typed_data` actions for Polymarket — use `polymarket_sign_bet`
- The `order_ref` comes from `polymarket_place_bet` result — copy it verbatim into `polymarket_sign_bet` params
- After signing succeeds, the submission result is included in the action result — just report it
- "Bet $X" means the user wants to SPEND $X total. Use `spend: "X"` — the server calculates shares automatically.

### Betting shorthand

When the user says "bet $3 on Marco Rubio":
1. `polymarket_search` — find the market
2. `polymarket_check_approvals` — fix any missing approvals
3. `polymarket_place_bet` with `spend: "3"` — get summary + order_ref
4. Show summary, emit `polymarket_sign_bet` with order_ref
5. User signs -> auto-submitted -> report result

### Order types

| Type | Behavior | Use when |
|------|----------|----------|
| GTC (Good-Til-Cancelled) | Rests on the book at exact price. No slippage. | Default. "Buy at this price." |
| GTD (Good-Til-Date) | Like GTC but expires at a specific time. | "Buy at this price, but cancel if not filled by Friday." |
| FOK (Fill-Or-Kill) | Must fill entirely at `price` or better, or cancels. | "Buy now at market price." Warn about slippage. |
| FAK (Fill-And-Kill) | Fills what's available at `price` or better. Partial fill OK. | "Buy what you can right now." Warn about partial fills. |

### After order placement

Handle status responses from the auto-submission result:
- `matched` -> "Order filled! You now hold X shares."
- `live` -> "Limit order placed on the book."
- `delayed` -> "Order is being processed."
- `unmatched` -> "Order couldn't be placed. [reason]. Want to try a different price?"

## Discovery Flow

1. `polymarket_search` — **ALWAYS call this first.** Find markets by topic (e.g. "Trump", "Bitcoin", "Fed rate")
   - Returns tradeable markets only (closed/inactive/zero-liquidity filtered out)
   - Returns event slugs, prices, and outcomes — everything needed for `polymarket_place_bet`
   - Use `question_contains` to find specific outcomes in large events
   - **This is the primary discovery tool.**
2. `polymarket_market_info` — ONLY needed for browsing a specific event's full list of sub-markets
   - Supports `question_contains` filter and pagination (`offset`, `limit`)
3. `polymarket_price` — check current probability/price (optional — search already returns prices)
4. `polymarket_orderbook` — check available liquidity at each price level

### CRITICAL: Do NOT re-fetch data you already have

After search returns event slugs, prices, and outcomes:
- Do NOT call market_info again for the same market
- Do NOT call search again with the same query
- Do NOT re-check approvals if `polymarket_check_approvals` already returned `all_approved: true` this conversation (but balance was already reported)
- Go straight to `polymarket_place_bet`

## Token ID Selection (Server-Side Resolution)

**ALWAYS use `event_slug` + `outcome` parameters on `polymarket_place_bet`.**

The `event_slug` MUST come from `polymarket_search` results. NEVER fabricate, guess, or recall slugs from memory.

The server resolves the correct CLOB token ID automatically:
- For multi-outcome events: pass the candidate/option name as `outcome` (e.g., "Marco Rubio")
- For binary markets: pass "Yes" or "No" as `outcome`

**NEVER manually select or pass CLOB token IDs.** Use the event slug + outcome.

## Selling Flow

Same as buying but with `side: SELL`. The user must already hold outcome tokens.

1. Check position: `polymarket_positions`
2. Place sell: `polymarket_place_bet` with `side: SELL`
3. Show summary, emit `polymarket_sign_bet`
4. Auto-submitted after signing

## Managing Orders

- `polymarket_open_orders` — list active orders. Only pass `address` — auth credentials are cached.
- `polymarket_cancel_order` — cancel a specific order by ID. Only pass `order_id` + `address`.

**DO NOT** pass fabricated `auth_signature` or `auth_timestamp` to these tools. Just pass `address` and let the server use cached credentials.

## Positions & History

- `polymarket_positions` — current holdings with entry price, current price, P&L
- `polymarket_trades` — historical trade log

### Position Display Format

When showing Polymarket positions, ALWAYS display:
- Market name / outcome
- Shares held
- Current value (from currentValue field)
- Entry cost (shares x avgPrice)
- P&L in $ (currentValue - entryCost) and % (from pnlPercent field)

Format as a clean summary, not raw JSON.

## Approvals (One-time setup)

Polymarket requires USDC.e approvals for MULTIPLE contracts. Missing any one causes "not enough balance / allowance" errors.

### Required USDC.e approvals (approve on USDC.e contract, spender = each address below)

| # | Spender | Address | When needed |
|---|---------|---------|-------------|
| 1 | CTF Exchange | `0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E` | All markets |
| 2 | Neg Risk CTF Exchange | `0xC5d563A36AE78145C45a50134d48A1215220f80a` | negRisk markets (3+ outcomes) |
| 3 | Neg Risk Adapter | `0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296` | negRisk markets (3+ outcomes) |

**WARNING:** The spender is NEVER the Conditional Tokens contract (`0x4D97...`).

### Required Conditional Tokens approvals (setApprovalForAll on CT contract, operator = each address)

| # | Operator | Address | When needed |
|---|----------|---------|-------------|
| 1 | CTF Exchange | `0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E` | Selling on any market |
| 2 | Neg Risk CTF Exchange | `0xC5d563A36AE78145C45a50134d48A1215220f80a` | Selling on negRisk markets |
| 3 | Neg Risk Adapter | `0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296` | Selling on negRisk markets |

### Checking and fixing approvals

**Use `polymarket_check_approvals`** — ONE call checks all 6 approvals + USDC.e balance.

Response:
- `all_approved: true` -> skip straight to placing bet
- `missing_count: N` + `missing_actions: [...]` -> chain each action as `build_custom_tx` -> `sign_tx`
- `instruction` -> follow it exactly

**IMPORTANT:** Do NOT modify `missing_actions` params. Do NOT ask the user for approval. Just emit each action exactly as returned, one after another. Approvals persist across sessions.

## Redeeming Positions (After Market Resolution)

When a market resolves, winning tokens can be redeemed for USDC.e.

Use `build_custom_tx`:
- tx_type: `evm_contract`
- chain: `Polygon`
- contract_address: `0x4D97DCd97eC945f40cF65F87097ACe5EA0476045` (Conditional Tokens)
- function_name: `redeemPositions`
- params: `[{type: "address", value: "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"}, {type: "bytes32", value: "<parentCollectionId>"}, {type: "bytes32", value: "<conditionId>"}, {type: "uint256[]", value: "<indexSets>"}]`

Then `sign_tx` to execute redemption.

## Fees

Fee formula: `fee = feeRateBps / 10000 * min(price, 1 - price) * size`

- Base rate is ~200 bps (2%) but varies per market — `polymarket_place_bet` fetches it dynamically
- Always show estimated fee + total effective cost before placing order
- Example: "100 shares at $0.65 = $65.00 + $0.70 fee = $65.70 total"
- **Minimum order: $1** — orders with total cost under $1 are rejected.

## NegRisk Markets

Markets with 3+ outcomes use the Neg Risk CTF Exchange. `polymarket_place_bet` auto-detects and uses the correct exchange contract.

Key differences:
- Different exchange contract address
- Different EIP-712 domain (verifyingContract changes)
- **USDC.e approval required for THREE contracts**: NegRisk CTF Exchange, NegRisk Adapter, AND CTF Exchange
- Missing the NegRisk Adapter approval causes "not enough balance / allowance" errors

## Error Recovery

| Error | Resolution |
|-------|-----------|
| Insufficient USDC.e | "You need X more USDC.e on Polygon. Want to swap?" |
| not enough balance / allowance | 1. Verify USDC.e balance. 2. Check allowance for the CORRECT exchange contract. 3. Cancel stale orders with `polymarket_open_orders` then `polymarket_cancel_order`. |
| Market closed | "This market has closed and can no longer be traded." |
| Below minimum / $1 | "Minimum order is $1. Increase spend to meet the minimum." |
| Token resolution failed | Outcome not found. Check available outcomes with `polymarket_search`. |
| "pass the candidate NAME, not Yes" | Multi-outcome event — pass the person/option name, NOT "Yes"/"No". |
| place_bet failed | Check error message. Usually token resolution or balance issue. Re-search and retry. |

## DO NOTs

- **DO NOT** call `polymarket_build_order` or `polymarket_submit_order` directly — use `polymarket_place_bet` instead
- **DO NOT** construct `sign_typed_data` actions for Polymarket — use `polymarket_sign_bet`
- **DO NOT** call `polymarket_place_bet` without calling `polymarket_search` first — event_slug MUST come from search results
- **DO NOT** fabricate, guess, or recall event_slugs from memory — they MUST come from `polymarket_search`
- **DO NOT** fabricate or guess CLOB token IDs — use `event_slug` + `outcome` on place_bet
- **DO NOT** place orders without checking USDC.e balance and allowance first
- **DO NOT** approve the wrong contract — the spender is the EXCHANGE, NEVER the Conditional Tokens contract
- **DO NOT** use `abi_encode`, `evm_check_allowance`, or `evm_tx_info` for approvals — use `polymarket_check_approvals`
- **DO NOT** use FOK/FAK without warning the user about slippage/partial fills
- **DO NOT** place orders on closed markets
- **DO NOT** skip fee disclosure — always show estimated fee + effective cost
- **DO NOT** guess contract addresses — use the addresses in this skill
- **DO NOT** use `search_token` for Polymarket tokens — they are not on CoinGecko
- **DO NOT** pass "Yes" or "No" as the outcome for multi-outcome events — pass the candidate/option name
- **DO NOT** use `amount` when the user says "$X" or "X dollars" — use `spend` instead
- **DO NOT** batch multiple actions in one response — the backend strips all but the first
- **DO NOT** re-check approvals after they return `all_approved: true`
- **DO NOT** stop to ask clarifying questions when the user's intent is unambiguous
- **DO NOT** fabricate `order_ref` values — copy the EXACT `order_ref` from `polymarket_place_bet` response
- **DO NOT** ask "Which team?" or "Ready to sign?" when the user already specified what they want
