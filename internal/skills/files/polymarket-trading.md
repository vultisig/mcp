---
name: Polymarket Trading
description: Discover, trade, and manage prediction market positions on Polymarket via Polygon
tags: [polymarket, prediction-market, polygon, evm, trading, betting]
---

# Polymarket Trading

Trade prediction markets on Polymarket — a hybrid-decentralized platform on Polygon (Chain ID 137). Markets resolve to $1 (correct) or $0 (incorrect). Collateral is USDC.e.

## Contracts (Polygon)

| Contract | Address |
|----------|---------|
| CTF Exchange | `0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E` |
| Neg Risk CTF Exchange | `0xC5d563A36AE78145C45a50134d48A1215220f80a` |
| Conditional Tokens (ERC1155) | `0x4D97DCd97eC945f40cF65F87097ACe5EA0476045` |
| USDC.e | `0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174` |

## Authentication

Polymarket requires an EIP-712 "ClobAuth" signature to authenticate with the CLOB API. Derived API credentials do NOT expire (only the per-request HMAC timestamp has a 30-second window).

When the user first mentions Polymarket in a conversation (any intent — betting, positions, orders, markets):
1. Generate the ClobAuth EIP-712 payload:
   - primaryType: "ClobAuth"
   - domain: { name: "ClobAuthDomain", version: "1", chainId: 137 }
   - types: { ClobAuth: [{ name: "address", type: "address" }, { name: "timestamp", type: "string" }, { name: "nonce", type: "uint256" }, { name: "message", type: "string" }] }
   - message: { address: <user's Polygon address>, timestamp: <current unix seconds as string>, nonce: 0, message: "This message attests that I control the given wallet" }
2. Emit sign_typed_data with this single payload (id: "polymarket_auth", chain: "Polygon")
3. After the user signs, CACHE the auth_signature and auth_timestamp — reuse them for ALL subsequent Polymarket calls in this conversation (open_orders, cancel_order, submit_order)
4. Do NOT generate a new auth payload for every order — reuse the cached one

Before the user's first order in a conversation:
1. Check USDC.e allowance via evm_check_allowance (token: 0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174, spender: exchange contract)
   - Regular markets: spender = 0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E (CTF Exchange)
   - negRisk markets (3+ outcomes): spender = 0xC5d563A36AE78145C45a50134d48A1215220f80a (Neg Risk CTF Exchange)
2. If allowance is insufficient for the order amount, prompt the user: "Polymarket needs approval to spend your USDC.e. Approve unlimited spending? (one-time setup)"
3. On confirmation, emit build_custom_tx with tx_type: "evm_contract", chain: "Polygon", contract: 0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174, function: "approve", params: [{type:"address", value:"<exchange_address>"}, {type:"uint256", value:"115792089237316195423570985008687907853269984665640564039457584007913129639935"}]
4. After approval tx is signed and confirmed, proceed with the original order

## Betting Shorthand

When the user says "bet" on a Polymarket outcome (e.g. "bet $3 on Kevin Warsh"):
1. Look up the market via polymarket_search + polymarket_market_info
2. Get the current best ask via polymarket_price (use buy_price)
3. Show the user: "[outcome] is trading at $[buy_price]. Buying [shares] shares costs ~$[total] (incl. ~$[fee] fee). Confirm?"
4. On confirmation → polymarket_build_order with FOK order type at buy_price
5. Then sign_typed_data → polymarket_submit_order (standard flow)

"Bet $X" means the user wants to spend $X total. Calculate shares = X / buy_price.

## Discovery Flow

1. `polymarket_search` — find markets by topic (e.g. "Trump", "Bitcoin", "Fed rate")
2. `polymarket_market_info` — get details: outcomes, CLOB token IDs, neg_risk flag
   - Only active, tradable markets are returned (closed/inactive/zero-liquidity filtered out)
   - For large events (many sub-markets), use `question_contains` to filter by text instead of paginating
   - Example: `polymarket_market_info(slug: "presidential-election-2028", question_contains: "Vance")` → returns only matching markets in one call
3. `polymarket_price` — check current probability/price for each outcome
4. `polymarket_orderbook` — check available liquidity at each price level

## Token ID Selection (CRITICAL)

Multi-outcome (negRisk) events have multiple markets, each with different clobTokenIds.
When placing an order:
1. Call polymarket_market_info to get the full event with ALL markets listed
2. BEFORE selecting a token ID, LIST every market's "question" and "outcomes" from the API response in your reasoning
3. Match the user's chosen outcome to the correct market question — it must be an EXACT match to an outcome string returned by the API
4. Use the EXACT clobTokenIds from that matched market (index 0 = first outcome, index 1 = second outcome)
5. NEVER reference, suggest, or use an outcome that does not appear in the API response — if the user names someone/something not in the outcomes list, tell them it's not available on this market
6. NEVER reuse token IDs from a different market within the same event
7. If unsure which market matches, show the user the available outcomes and ask them to pick

## Order Placement Flow

### Pre-trade checklist (ALWAYS follow before placing any order)

1. Check USDC.e balance on Polygon: `evm_get_token_balance` with contract `0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174` on chain `Polygon`
   - Show the Polygon USDC.e balance, NOT Ethereum USDC
   - If the user has USDC on Ethereum but not Polygon, tell them: "You have [X] USDC on Ethereum but Polymarket uses USDC.e on Polygon. You'd need to bridge first."
   - For order cost calculations, compare against Polygon USDC.e balance only
2. Check USDC.e allowance for CTF Exchange: `evm_check_allowance` with token `0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174`, spender = exchange contract (see neg_risk below), chain `Polygon`
3. If allowance insufficient → approve flow (see Approval section below)
4. For FOK/FAK orders: check orderbook liquidity at desired price
5. Calculate and show fee estimate + effective cost (see Fees section)
6. Confirm with user before proceeding to sign

### Placing an order

1. `polymarket_build_order` — returns two EIP-712 payloads + an `order_ref` (server-side reference)
2. `sign_typed_data` action — client signs BOTH payloads (`order_eip712` and `auth_eip712`), returns both signatures
3. `polymarket_submit_order` — pass `order_ref` + both signatures + address. The server retrieves auth_timestamp, order_params, and order_type from the stored build result (no manual data threading needed)

**IMPORTANT:** Always pass `order_ref` to submit_order. This prevents data corruption (stale timestamps, wrong types) that causes 401 errors.

### Order types

| Type | Behavior | Use when |
|------|----------|----------|
| GTC (Good-Til-Cancelled) | Rests on the book at exact price. No slippage. | Default. "Buy at this price." |
| GTD (Good-Til-Date) | Like GTC but expires at a specific time. | "Buy at this price, but cancel if not filled by Friday." |
| FOK (Fill-Or-Kill) | Must fill entirely at `price` or better, or cancels. Market order. | "Buy now at market price." Warn about slippage. |
| FAK (Fill-And-Kill) | Fills what's available at `price` or better. Partial fill OK. | "Buy what you can right now." Warn about partial fills. |

### Order Confirmation (CRITICAL — two-step signing)

When you have built an order via polymarket_build_order, do NOT emit sign_typed_data in the same response. Instead:
1. Respond with order summary: "Buy [shares] shares of [outcome] at $[price]. Cost: $[amount] + $[fee] fee = $[total]. Confirm?"
2. Store the EIP-712 payloads from build_order — you will need them after confirmation.
3. When user confirms (yes, confirm, do it, go) → THEN emit sign_typed_data with the stored payloads.
4. After signing succeeds → immediately call polymarket_submit_order with the signatures.

NEVER bundle sign_typed_data with the order summary text. The user must see what they're signing before the password prompt appears.

### After order placement

Handle status responses:
- `matched` → "Order filled! You now hold X shares."
- `live` → "Order placed on the book. I'll monitor it."
- `delayed` → "Order is being processed. Checking again..."
- `unmatched` → "Order couldn't be placed. [reason]. Want to try a different price?"

## Selling Flow

Same as buying but with `side: SELL`. The user must already hold outcome tokens for that market.

1. Check position: `polymarket_positions`
2. Build sell order: `polymarket_build_order` with `side: SELL`
3. Sign + submit: same flow as buying

## Managing Orders

- `polymarket_open_orders` — list active orders (requires auth signature)
- `polymarket_cancel_order` — cancel a specific order by ID (requires auth signature)

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

## USDC.e Approval (One-time per exchange)

Before placing the first order, USDC.e must be approved for the correct exchange contract.

For non-negRisk markets → approve for CTF Exchange `0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E`
For negRisk markets → approve for Neg Risk CTF Exchange `0xC5d563A36AE78145C45a50134d48A1215220f80a`

Use `build_custom_tx`:
- tx_type: `evm_contract`
- chain: `Polygon`
- contract_address: `0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174` (USDC.e)
- function_name: `approve`
- params: `[{type: "address", value: "<exchange_address>"}, {type: "uint256", value: "115792089237316195423570985008687907853269984665640564039457584007913129639935"}]`

Then `sign_tx` to execute the approval.

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

- Base rate is ~200 bps (2%) but varies per market — `polymarket_build_order` fetches it dynamically
- Always show estimated fee + total effective cost before placing order
- Example: "100 shares at $0.65 = $65.00 + $0.70 fee = $65.70 total"

## NegRisk Markets

Markets with 3+ outcomes (e.g. "Who will win the election?") use the Neg Risk CTF Exchange at `0xC5d563A36AE78145C45a50134d48A1215220f80a`. The `neg_risk` flag on market info indicates this. `polymarket_build_order` auto-detects and uses the correct exchange contract.

Key differences:
- Different exchange contract address
- Different EIP-712 domain (verifyingContract changes)
- USDC.e approval must be for the Neg Risk exchange

## Scheduling

After placing a bet, suggest monitoring:

> "Want me to monitor this position? I'll check every 6 hours and alert you if the market resolves, price moves >10%, or it's closing within 24h."

Use `schedule_task` with:
- intent: "Check Polymarket position for [market_name]. Alert if resolved, price moved >10% from [entry_price], or market closing within 24h."
- context: `{ market_slug, condition_id, token_id, entry_price, user_address, outcome }`
- interval_seconds: 21600 (6 hours)

## Error Recovery

| Error | Resolution |
|-------|-----------|
| Insufficient USDC.e | "You need X more USDC.e on Polygon. Want to swap?" |
| Market closed | "This market has closed and can no longer be traded." |
| Below minimum | "Minimum order is X. Want to increase?" |
| Invalid tick size | Auto-rounded in `polymarket_build_order` |
| CLOB unavailable | "The Polymarket order book is temporarily unavailable. Try again shortly." |

## DO NOTs

- **DO NOT** place orders without checking USDC.e balance and allowance first
- **DO NOT** use FOK/FAK without warning the user about slippage/partial fills
- **DO NOT** place orders on closed markets
- **DO NOT** skip fee disclosure — always show estimated fee + effective cost
- **DO NOT** guess contract addresses — use the addresses in this skill
- **DO NOT** use `search_token` for Polymarket tokens — they are not on CoinGecko
