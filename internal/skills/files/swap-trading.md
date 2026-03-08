---
name: Swap Trading
description: Build and confirm token swap transactions across chains
tags: [swap, trading, cross-chain, dex]
---

# Swap Trading

Build and confirm token swap transactions. Supports THORChain, Mayachain, 1inch, LiFi, Jupiter, and Uniswap providers.

## Pre-checks

STOP — before calling build_swap_tx, the user MUST have specified an exact amount. If they said "swap USDC to ETH" or "swap my USDC" without a number, respond with "How much [TOKEN] do you want to swap?" and WAIT. Do NOT use their full balance. Do NOT assume any amount.

ALWAYS check the user's Balances context before calling build_swap_tx. Use the balance for the specific chain (e.g. "USDT on Ethereum"), NOT the sum across all chains. If insufficient, tell the user and do NOT build. Warn if source balance is under ~$5 (DEX minimums for swaps).

## Building a Swap

Emit build_swap_tx as a respond_to_user action (NOT as an MCP tool call). The app handles building, quoting providers, and storing the transaction for signing.

### Parameter Rules

- **from_contract** and **from_decimals**: copy EXACTLY from "Coins in Vault" context. The "contract" field is from_contract, the "decimals" field is from_decimals. For native tokens (contract = "native"), set from_contract to empty string "".
- **to_contract** and **to_decimals**: copy EXACTLY from "Coins in Vault" if the destination token is there. If not in vault, omit both fields (app resolves automatically).
- NEVER fabricate or guess contract addresses. Only use values copied verbatim from context.
- If the source token is NOT in "Coins in Vault", the user doesn't have it — tell them.
- If the destination token is NOT in "Coins in Vault", omit to_contract and to_decimals.

### Amount

Use human-readable amounts (e.g. "10" for 10 USDC, "0.5" for 0.5 ETH). The app converts to base units using the decimals you provide.

### Fiat/Dollar Amounts

When the user specifies a fiat/dollar amount (e.g., "$10 of ETH", "100 USD worth of BTC"), do NOT put the fiat number in the amount field. Instead:
1. First call get_market_price for the source token to get the current price.
2. After receiving the price result, calculate: token_amount = fiat_amount / price.
3. Compare the calculated token_amount against the user's balance in Balances context. If insufficient, tell the user (e.g. "You only have 0.899 ETH (~$1,786), which isn't enough for a $10,000 swap.") and do NOT call build_swap_tx.
4. Use the calculated token_amount (human-readable) in the build_swap_tx action.

## Swap Confirmation (respond_to_user required)

When emitting build_swap_tx, call respond_to_user with BOTH text AND actions:

- text: "Swap [amount] [FROM] to [TO]. Ready to swap?"
- actions: [{type: "build_swap_tx", title: "Build Swap", params: {from_chain, from_symbol, from_contract, from_decimals, to_chain, to_symbol, to_contract, to_decimals, amount}}]

If cross-chain, add "(cross-chain)" to the text.

EXAMPLE — user says "swap 10 USDC to ETH":
Call respond_to_user with:
- text: "Swap 10 USDC to ETH. Ready to swap?"
- actions: [{type: "build_swap_tx", title: "Build Swap", params: {from_chain: "Ethereum", from_symbol: "USDC", from_contract: "0x...", from_decimals: 6, to_chain: "Ethereum", to_symbol: "ETH", to_contract: "", to_decimals: 18, amount: "10"}}]

WRONG (NEVER do these):
- Returning plain text without calling respond_to_user ← BREAKS the app
- "You have 173 USDC. Building swap for 10 USDC → ETH." ← NO commentary
- "I'll swap 10 USDC to ETH for you!" ← NO preamble
- Emitting sign_tx at this step ← too early, wait for user confirmation
- Using base units in amount (e.g. "10000000") ← use human-readable "10"

RULES:
- MUST call respond_to_user (not plain text)
- ENTIRE text must match the template exactly. Zero extra words.
- Do NOT say "Building", "Processing", "I'll", "Let me", "Great", or ANY preamble.

## Post-Confirmation

After you show the confirmation and the user responds:
- User confirms (yes, confirm, do it, go, etc.) → call respond_to_user with actions: [{type: "sign_tx", title: "Sign Swap"}].
- User wants changes → new respond_to_user with adjusted build_swap_tx params.
- User cancels → call respond_to_user acknowledging, no sign_tx.
- NEVER say "Transaction submitted" unless sign_tx was actually emitted AND returned success.

## DO NOTs

- **DO NOT** call build_swap_tx without a specific amount from the user
- **DO NOT** assume the user wants to swap their full balance
- **DO NOT** use base units — use human-readable amounts (app converts)
- **DO NOT** fabricate contract addresses — only use values from "Coins in Vault" or search_token
- **DO NOT** add commentary to the confirmation template
- **DO NOT** call build_swap_tx as an MCP tool for signing flows — always emit as respond_to_user action
