---
name: Custom Transactions
description: Build deposit, EVM contract call, CosmWasm, and THORChain transactions
tags: [custom-tx, thorchain, evm, cosmwasm, deposit, contract-call]
---

# Custom Transactions

Build advanced on-chain operations: THORChain/Maya deposits, EVM smart contract calls, CosmWasm execution, and THORChain position queries.

## build_custom_tx

Use build_custom_tx for advanced on-chain operations. Parameters vary by tx_type.

### Deposit (tx_type: "deposit")

For THORChain/Maya MsgDeposit operations (bond, unbond, leave, etc.).
- **chain**: "THORChain" or "MayaChain"
- **symbol**: the token ticker (e.g. "RUNE", "CACAO")
- **amount**: human-readable units (e.g. "1000" for 1000 RUNE)
- **memo**: the THORChain memo (e.g. "BOND:thor1abc...", "UNBOND:thor1abc...:100000000", "LEAVE:thor1abc...")

CRITICAL: For UNBOND, set amount to "0" — the unbond amount goes ONLY in the memo in base units (1 RUNE = 100000000). Example: "UNBOND:thor1nodeaddr:100000000" unbonds 1 RUNE.

### EVM Contract Call (tx_type: "evm_contract")

For calling smart contracts on EVM chains (Ethereum, Arbitrum, etc.).
- **chain**: the EVM chain name (e.g. "Ethereum", "Arbitrum")
- **contract_address**: the contract address (e.g. "0xa0b86991...")
- **function_name**: the Solidity function name (e.g. "approve", "transfer")
- **params**: array of {type, value} objects. Supported types: "address", "uint256", "string", "bytes", "bool"
- **value**: optional ETH/native token value to send with the call, in human-readable units (default "0")

Example approve call: tx_type="evm_contract", chain="Ethereum", contract_address="0xUSDC", function_name="approve", params=[{type:"address",value:"0xSpender"},{type:"uint256",value:"1000000"}]

### CosmWasm Execute (tx_type: "wasm_execute")

For executing CosmWasm smart contracts (e.g. on THORChain).
- **chain**: the chain name (e.g. "THORChain")
- **contract_address**: the WASM contract address
- **execute_msg**: JSON string of the execute message (e.g. '{"stake":{}}')
- **funds**: optional array of {denom, amount} objects for coins to send with execution. Denoms are lowercase (e.g. "rune"), amounts in base units.

### Confirmation

When you emit a build_custom_tx action, include a confirmation prompt in the same response:

TEMPLATE for deposit: "[ACTION] [amount] [SYMBOL] on [chain]. Ready to execute?"
TEMPLATE for evm_contract: "Call [function_name] on [truncated_contract] on [chain]. Ready to execute?"
TEMPLATE for wasm_execute: "Execute contract [truncated_contract] on [chain]. Ready to execute?"

When the user confirms → return sign_tx action with empty params.
When the user cancels → acknowledge briefly, no sign_tx.

## THORChain Position Queries

Use thorchain_query to look up THORChain/Midgard data when the user asks about their THORChain positions, LP, bonds, savers, stakes, or network info.

### Parameters

- **query_type** (required): one of "lp_positions", "saver_positions", "bond_positions", "node_details", "pool_info", "rune_pool", "network_info", "stake_positions", "trade_accounts"
- **asset** (optional): for pool_info, specify the pool asset (e.g. "BTC.BTC", "ETH.ETH")

The user's THORChain address is auto-resolved. Query mapping:
- "What are my LP positions?" → lp_positions
- "Show my savers" → saver_positions
- "What nodes am I bonded to?" → bond_positions
- "Show my stakes" → stake_positions
- "What's the BTC pool?" → pool_info, asset="BTC.BTC"
- "My RUNE pool position" → rune_pool
- "THORChain network stats" → network_info
- "My trade accounts" → trade_accounts

## Reading EVM Contract State

Use read_evm_contract to call read-only (view/pure) functions on EVM smart contracts. This does NOT create a transaction — it's a free eth_call.

### Parameters

- **chain**: the EVM chain name (e.g. "Ethereum", "Arbitrum")
- **contract_address**: the contract address (e.g. "0xa0b86991...")
- **function_name**: the Solidity function signature (e.g. "allowance(address,address)", "balanceOf(address)")
- **params**: array of {type, value} objects matching the function inputs. Supported types: "address", "uint256", "string", "bytes", "bool"
- **output_types**: array of output type strings (e.g. ["uint256"]). Supported: "address", "uint256", "string", "bytes", "bool"

### Common Uses

- Check ERC20 allowance: function_name="allowance(address,address)", params=[{type:"address",value:"OWNER"},{type:"address",value:"SPENDER"}], output_types=["uint256"]
- Check ERC20 balance: function_name="balanceOf(address)", params=[{type:"address",value:"HOLDER"}], output_types=["uint256"]

The result is returned as action data with decoded output values. Use the user's address from Addresses context as the owner/holder.

## DO NOTs

- **DO NOT** guess contract addresses — only use values from context or known constants
- **DO NOT** set a non-zero amount for UNBOND deposits — the amount goes in the memo
- **DO NOT** use read_evm_contract for state-changing operations — it's read-only
