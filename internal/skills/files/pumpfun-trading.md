---
name: Pump.fun Trading
description: Buy, sell, and create pump.fun memecoins on Solana
tags: [pumpfun, solana, memecoin, trading, token-creation]
---

# Pump.fun Trading

Buy, sell, and create pump.fun memecoins on Solana. Pump.fun tokens use bonding curves — price rises as more people buy.

## Key Facts

- All pump.fun tokens are SPL tokens on Solana (6 decimals)
- Tokens start on a bonding curve; after ~85 SOL in real reserves, they "graduate" to PumpSwap/Raydium
- Jupiter automatically routes through pump.fun bonding curves — use `build_solana_swap` for buying/selling
- Program ID: `6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P`

## Checking Token Info

Before trading, check the token's bonding curve state:

1. Call `get_pumpfun_token_info` with the token's mint address
2. Review: price, market cap, graduation progress, and whether it's graduated
3. If `status: "graduated"`, the token has moved to PumpSwap/Raydium — still tradeable via Jupiter

## Buying a Pump.fun Token

Use the existing Solana swap workflow:

1. **Check balance**: `get_sol_balance` — ensure user has enough SOL
2. **Check token**: `get_pumpfun_token_info` — verify it exists and is active
3. **Build swap**: `build_solana_swap` with:
   - `input_mint`: empty (native SOL)
   - `output_mint`: the pump.fun token mint address
   - `amount`: amount in lamports
   - `slippage_bps`: recommend 500–1000 (5–10%) for pump.fun tokens
4. Confirm with user, then sign

**Slippage**: Pump.fun tokens are volatile. Default 1% slippage often fails. Recommend 5–10% (500–1000 bps) unless the user specifies otherwise. Always warn about high slippage.

## Selling a Pump.fun Token

1. **Check balance**: `get_spl_token_balance` with the token mint
2. **Check token**: `get_pumpfun_token_info` — show current price
3. **Build swap**: `build_solana_swap` with:
   - `input_mint`: the pump.fun token mint address
   - `output_mint`: empty (native SOL)
   - `amount`: amount in token base units (6 decimals)
   - `slippage_bps`: recommend 500–1000
4. Confirm with user, then sign

## Creating a New Token

Token creation requires multiple steps. The client handles the actual transaction.

1. User provides: name, symbol, description, and optionally an image
2. **Metadata**: The client must upload metadata to pump.fun's IPFS endpoint first
3. **Mint keypair**: The client generates a new Solana keypair for the mint
4. **Build create**: Call `build_pumpfun_create` with name, symbol, metadata_uri, and mint_address
5. Optionally include `initial_buy_amount` for the creator to buy tokens at launch
6. Confirm with user, then sign (requires both creator and mint keypair signatures)

## Pre-checks

STOP — before any trade, the user MUST have specified an exact amount. If they said "buy some [TOKEN]" without a number, respond with "How much SOL do you want to spend?" and WAIT.

ALWAYS check balance before building a swap. If insufficient SOL, tell the user and do NOT build.

## DO NOTs

- **DO NOT** trade without checking `get_pumpfun_token_info` first — the token may be graduated, a rug, or nonexistent
- **DO NOT** use default 1% slippage for pump.fun tokens — recommend 5–10%
- **DO NOT** assume any amount — always ask the user
- **DO NOT** call `build_pumpfun_create` without a metadata_uri — metadata must be uploaded first
- **DO NOT** fabricate mint addresses — the client must generate the keypair
- **DO NOT** recommend pump.fun tokens as investments — these are highly speculative memecoins
