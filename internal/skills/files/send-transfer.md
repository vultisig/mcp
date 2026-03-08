---
name: Send Transfer
description: Build and confirm token send transactions
tags: [send, transfer, transaction]
---

# Send Transfer

Build and confirm token send (transfer) transactions across supported chains.

## Pre-checks

STOP — before calling build_send_tx, the user MUST have specified an exact amount. If they said "send ETH to 0x..." without a number, respond with "How much [TOKEN] do you want to send?" and WAIT. Do NOT use their full balance. Do NOT assume any amount.

ALWAYS check the user's Balances context before calling build_send_tx. If insufficient balance, tell the user and do NOT build.

If any required param (chain/symbol, address, amount) is missing, ask the user for it. Do NOT call build_send_tx until all params are known.

## Building a Send

When the user wants to send tokens and all required params are known (coin + address + amount), emit build_send_tx as a respond_to_user action (NOT as an MCP tool call). The app handles conversion to base units, so use human-readable amounts.

### Parameters

- **chain**: the blockchain name (e.g. "Ethereum", "Bitcoin")
- **symbol**: the token ticker (e.g. "ETH", "USDC")
- **address**: the recipient address
- **amount**: human-readable units (e.g. "0.1" for 0.1 ETH)
- **memo**: optional memo/tag. All native gas token sends support memos (EVM chains encode it in the tx data field, UTXO chains use OP_RETURN, Cosmos/THORChain use the memo field, etc.). Only omit for ERC20/SPL/other non-native token transfers.

## Send Confirmation (respond_to_user required)

When emitting build_send_tx, call respond_to_user with BOTH text AND actions:

- text: "Send [amount] [SYMBOL] to [truncated_address] on [chain]. Ready to send?"
- actions: [{type: "build_send_tx", title: "Build Send", params: {chain, symbol, address, amount, memo}}]

Truncate address: first 6 and last 4 chars (e.g. "0x1234...5678").

RULES:
- MUST call respond_to_user (not plain text — plain text breaks the app)
- Zero extra words before or after the template
- Text MUST end with "Ready to send?"

## Post-Confirmation

After you show the confirmation and the user responds:
- User confirms (yes, confirm, go, do it) → call respond_to_user with actions: [{type: "sign_tx", title: "Sign Send"}].
- User cancels → call respond_to_user acknowledging, no sign_tx.
- NEVER say "Transaction submitted" unless sign_tx was actually emitted AND returned success.

## DO NOTs

- **DO NOT** call build_send_tx without a specific amount from the user
- **DO NOT** assume the user wants to send their full balance
- **DO NOT** call build_send_tx until all required params (chain, symbol, address, amount) are known
- **DO NOT** add commentary to the confirmation template
