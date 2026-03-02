# Vultisig MCP Server

An [MCP](https://modelcontextprotocol.io/) server that exposes Vultisig vault operations, multi-chain balance queries, transaction building, and DeFi protocol interactions to LLM-powered agents. Supports both stdio and HTTP transports.

## Quick Start

```bash
# Build
go build -o mcp-server ./cmd/mcp-server/

# Run over stdio (default)
./mcp-server

# Run over HTTP on port 8080
./mcp-server -http :8080
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `EVM_ETHEREUM_URL` | `https://ethereum-rpc.publicnode.com` | Ethereum JSON-RPC endpoint |
| `EVM_BSC_URL` | `https://bsc-rpc.publicnode.com` | BNB Smart Chain JSON-RPC endpoint |
| `EVM_POLYGON_URL` | `https://polygon-bor-rpc.publicnode.com` | Polygon JSON-RPC endpoint |
| `EVM_AVALANCHE_URL` | `https://avalanche-c-chain-rpc.publicnode.com` | Avalanche C-Chain JSON-RPC endpoint |
| `EVM_ARBITRUM_URL` | `https://arbitrum-one-rpc.publicnode.com` | Arbitrum One JSON-RPC endpoint |
| `EVM_OPTIMISM_URL` | `https://optimism-rpc.publicnode.com` | Optimism JSON-RPC endpoint |
| `EVM_BASE_URL` | `https://base-rpc.publicnode.com` | Base JSON-RPC endpoint |
| `EVM_BLAST_URL` | `https://blast-rpc.publicnode.com` | Blast JSON-RPC endpoint |
| `EVM_MANTLE_URL` | `https://mantle-rpc.publicnode.com` | Mantle JSON-RPC endpoint |
| `EVM_ZKSYNC_URL` | `https://mainnet.era.zksync.io` | zkSync Era JSON-RPC endpoint |
| `BLOCKCHAIR_API_URL` | `https://api.vultisig.com/blockchair` | Blockchair proxy base URL for UTXO chain queries |
| `THORCHAIN_URL` | `https://thornode.ninerealms.com` | THORChain node URL for fee rates (BTC, LTC, DOGE, BCH) |
| `MAYACHAIN_URL` | `https://mayanode.mayachain.info` | MayaChain node URL for fee rates (DASH, ZEC) |
| `SOLANA_RPC_URL` | `https://api.mainnet-beta.solana.com` | Solana JSON-RPC endpoint |
| `JUPITER_API_URL` | `https://api.jup.ag` | Jupiter DEX aggregator API base URL |

## Tools

### Vault

#### `set_vault_info`

Store vault key material for the current session. Must be called before any vault-derived address or balance queries.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `ecdsa_public_key` | Yes | Hex-encoded compressed ECDSA public key (33 bytes / 66 hex chars) |
| `eddsa_public_key` | Yes | Hex-encoded EdDSA public key (32 bytes / 64 hex chars) |
| `chain_code` | Yes | Hex-encoded 32-byte chain code for BIP-32 derivation |

#### `get_address`

Derive the address for a given blockchain network from vault key material. Requires `set_vault_info` first.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `chain` | Yes | Blockchain network name (see supported chains below) |

Supported chains: Arbitrum, Avalanche, Base, Bitcoin, Bitcoin-Cash, Blast, BSC, Cosmos, CronosChain, Dash, Dogecoin, Dydx, Ethereum, Kujira, Litecoin, Mantle, MayaChain, Noble, Optimism, Osmosis, Polygon, Ripple, Solana, Sui, Terra, TerraClassic, THORChain, Tron, Zcash, Zksync.

---

### Token Discovery

#### `search_token`

Search for tokens by ticker symbol, name, or contract address via CoinGecko. Returns token metadata and all known contract deployments across chains, ranked by market cap.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `query` | Yes | Ticker (e.g. `USDC`), name (e.g. `Uniswap`), or contract address |

---

### EVM (multi-chain)

All EVM tools accept a `chain` parameter (default: `Ethereum`). Supported chains: Ethereum, BSC, Polygon, Avalanche, Arbitrum, Optimism, Base, Blast, Mantle, Zksync.

#### `evm_get_balance`

Query the native coin balance of an address. Address falls back to vault-derived if omitted.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `chain` | No | EVM chain name (default: `Ethereum`) |
| `address` | No | Wallet address. Falls back to vault-derived if omitted. |

#### `evm_get_token_balance`

Query an ERC-20 token balance. Returns symbol, balance, and decimals.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `chain` | No | EVM chain name (default: `Ethereum`) |
| `contract_address` | Yes | ERC-20 token contract address (0x-prefixed) |
| `address` | No | Holder address. Falls back to vault-derived if omitted. |

#### `evm_check_allowance`

Check how much of an ERC-20 token a spender is allowed to transfer. Used to determine if an approve transaction is needed before a swap or protocol interaction.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `chain` | No | EVM chain name (default: `Ethereum`) |
| `contract_address` | Yes | ERC-20 token contract address (0x-prefixed) |
| `owner` | No | Token holder address. Falls back to vault-derived if omitted. |
| `spender` | Yes | Address allowed to spend tokens (e.g. DEX router contract) |

#### `evm_tx_info`

Get nonce, gas prices, and chain ID for building an EVM transaction. Optionally estimates gas if `to`/`data`/`value` are provided.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `chain` | No | EVM chain name (default: `Ethereum`) |
| `address` | No | Sender address. Falls back to vault-derived if omitted. |
| `to` | No | Destination address for gas estimation |
| `data` | No | Hex calldata for gas estimation |
| `value` | No | Wei value for gas estimation (decimal string) |

#### `evm_call`

Execute a read-only `eth_call` against a contract. Returns raw hex output and optionally decodes it.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `chain` | No | EVM chain name (default: `Ethereum`) |
| `to` | Yes | Contract address (0x-prefixed) |
| `data` | Yes | Hex-encoded calldata (0x-prefixed) |
| `from` | No | Sender address for call context |
| `value` | No | Wei value to send with the call (decimal string) |
| `block` | No | Block number (decimal) or `"latest"` (default) |
| `output_types` | No | Comma-separated ABI types to decode output (e.g. `"uint256,address"`) |

#### `build_evm_tx`

Build an unsigned EIP-1559 (type 2) transaction for any EVM chain. Obtain fee/nonce parameters from `evm_tx_info` first.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `chain` | No | EVM chain name (default: `Ethereum`). Sets `chain_id` when not explicitly overridden. |
| `to` | Yes | Destination address (0x-prefixed) |
| `value` | Yes | Wei value to transfer (decimal string) |
| `nonce` | Yes | Sender nonce (decimal string) |
| `gas_limit` | Yes | Gas limit (decimal string) |
| `max_fee_per_gas` | Yes | Max fee per gas in wei (decimal string) |
| `max_priority_fee_per_gas` | Yes | Max priority fee (tip) per gas in wei (decimal string) |
| `data` | No | Hex-encoded calldata (default `"0x"`) |
| `chain_id` | No | Chain ID override (decimal string) |

---

### Swaps

#### `build_swap_tx`

Build unsigned transaction(s) for a token swap. Supports THORChain, MayaChain, 1inch, LiFi, Jupiter, and Uniswap. Returns the swap transaction and an optional ERC-20 approval transaction.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `from_chain` | Yes | Source chain (e.g. `"Ethereum"`, `"Bitcoin"`) |
| `from_symbol` | Yes | Source token symbol (e.g. `"ETH"`, `"USDC"`) |
| `from_address` | No | Source token contract address (empty for native coins) |
| `from_decimals` | Yes | Source token decimals (e.g. `18` for ETH, `6` for USDC) |
| `to_chain` | Yes | Destination chain |
| `to_symbol` | Yes | Destination token symbol |
| `to_address` | No | Destination token contract address (empty for native coins) |
| `to_decimals` | Yes | Destination token decimals |
| `amount` | Yes | Amount in base units (e.g. `"1000000"` for 1 USDC) |
| `sender` | Yes | Sender wallet address |
| `destination` | Yes | Destination wallet address |

---

### Bitcoin

#### `btc_fee_rate`

Get the recommended Bitcoin fee rate in sat/vB from THORChain. No parameters.

#### `build_btc_send`

Build an unsigned Bitcoin PSBT for a send or swap. Automatically selects UTXOs, calculates fees, and handles change. Requires `set_vault_info` first.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `to_address` | Yes | Recipient Bitcoin address (or THORChain vault address for swaps) |
| `amount` | Yes | Amount to send in satoshis (decimal string) |
| `fee_rate` | Yes | Fee rate in sat/vB (use `btc_fee_rate` tool to get recommended rate) |
| `memo` | No | OP_RETURN memo (e.g. THORChain swap instruction) |
| `address` | No | Sender Bitcoin address. Falls back to vault-derived if omitted. |

---

### Litecoin

#### `ltc_fee_rate`

Get the recommended Litecoin fee rate in sat/vB from THORChain. No parameters.

#### `build_ltc_send`

Build an unsigned Litecoin PSBT for a send or swap. Requires `set_vault_info` first.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `to_address` | Yes | Recipient Litecoin address |
| `amount` | Yes | Amount to send in litoshis (decimal string) |
| `fee_rate` | Yes | Fee rate in sat/vB (use `ltc_fee_rate` to get recommended rate) |
| `memo` | No | OP_RETURN memo (e.g. THORChain swap instruction) |
| `address` | No | Sender Litecoin address. Falls back to vault-derived if omitted. |

---

### Dogecoin

#### `doge_fee_rate`

Get the recommended Dogecoin fee rate in sat/vB from THORChain. No parameters.

#### `build_doge_send`

Build an unsigned Dogecoin PSBT for a send or swap. Requires `set_vault_info` first.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `to_address` | Yes | Recipient Dogecoin address |
| `amount` | Yes | Amount to send in koinus (1 DOGE = 100,000,000 koinus, decimal string) |
| `fee_rate` | Yes | Fee rate in sat/vB (use `doge_fee_rate` to get recommended rate) |
| `memo` | No | OP_RETURN memo (e.g. THORChain swap instruction) |
| `address` | No | Sender Dogecoin address. Falls back to vault-derived if omitted. |

---

### Bitcoin Cash

#### `bch_fee_rate`

Get the recommended Bitcoin Cash fee rate in sat/vB from THORChain. No parameters.

#### `build_bch_send`

Build an unsigned Bitcoin Cash PSBT for a send or swap. Requires `set_vault_info` first.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `to_address` | Yes | Recipient Bitcoin Cash address (CashAddr or legacy format) |
| `amount` | Yes | Amount to send in satoshis (decimal string) |
| `fee_rate` | Yes | Fee rate in sat/vB (use `bch_fee_rate` to get recommended rate) |
| `memo` | No | OP_RETURN memo (e.g. THORChain swap instruction) |
| `address` | No | Sender Bitcoin Cash address. Falls back to vault-derived if omitted. |

---

### Dash

#### `dash_fee_rate`

Get the recommended Dash fee rate in sat/vB from MayaChain. No parameters.

#### `build_dash_send`

Build an unsigned Dash PSBT for a send or swap. Requires `set_vault_info` first.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `to_address` | Yes | Recipient Dash address |
| `amount` | Yes | Amount to send in duffs (1 DASH = 100,000,000 duffs, decimal string) |
| `fee_rate` | Yes | Fee rate in sat/vB (use `dash_fee_rate` to get recommended rate) |
| `memo` | No | OP_RETURN memo (e.g. MayaChain swap instruction) |
| `address` | No | Sender Dash address. Falls back to vault-derived if omitted. |

---

### Zcash

#### `build_zec_send`

Build an unsigned Zcash v4 (Sapling) transaction for a send or swap. Fee is calculated automatically using ZIP-317 — no `fee_rate` parameter needed. Requires `set_vault_info` first.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `to_address` | Yes | Recipient Zcash transparent address (t1... or t3...) |
| `amount` | Yes | Amount to send in zatoshis (1 ZEC = 100,000,000 zatoshis, decimal string) |
| `memo` | No | OP_RETURN memo (e.g. MayaChain swap instruction, max 80 bytes) |
| `address` | No | Sender Zcash address. Falls back to vault-derived if omitted. |

---

### MayaChain

#### `maya_fee_rate`

Get the recommended fee rate for any MayaChain-supported chain in sat/vB.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `chain` | Yes | MayaChain chain identifier: `BTC`, `ETH`, `ARB`, `ZEC`, `DASH`, or `THOR` |

---

### Solana

#### `get_sol_balance`

Query the native SOL balance of a Solana address.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `address` | No | Solana address (base58). Falls back to vault-derived if omitted. |

#### `get_spl_token_balance`

Query the SPL token balance of a Solana address for a given mint.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `mint` | Yes | Token mint address (base58) |
| `address` | No | Solana address (base58). Falls back to vault-derived if omitted. |

#### `build_solana_tx`

Build an unsigned native SOL transfer transaction. Requires `set_vault_info` first.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `to` | Yes | Recipient Solana address (base58) |
| `amount` | Yes | Amount in lamports (decimal string) |
| `from` | No | Sender Solana address. Falls back to vault-derived if omitted. |

#### `build_spl_transfer_tx`

Build an unsigned SPL token transfer transaction. Auto-detects token program (SPL vs Token-2022) and creates the destination ATA if it doesn't exist. Requires `set_vault_info` first.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `to` | Yes | Recipient Solana address (base58, the owner — not the ATA) |
| `mint` | Yes | Token mint address (base58) |
| `amount` | Yes | Amount in base units (decimal string) |
| `from` | No | Sender Solana address. Falls back to vault-derived if omitted. |

#### `build_solana_swap`

Build an unsigned Solana swap transaction via Jupiter aggregator. Requires `set_vault_info` first.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `output_mint` | Yes | Destination token mint address (base58). Leave empty for native SOL. |
| `amount` | Yes | Amount to swap in base units (lamports for SOL, smallest unit for tokens) |
| `input_mint` | No | Source token mint address (base58). Empty for native SOL. |
| `slippage_bps` | No | Slippage tolerance in basis points (default: 100 = 1%) |
| `from` | No | Sender Solana address. Falls back to vault-derived if omitted. |

---

### Encoding / Utilities

#### `abi_encode`

ABI-encode a Solidity function call or pack raw arguments. Handles 4-byte selector prepending for function calls.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `signature` | Yes | Function signature (e.g. `"transfer(address,uint256)"`) or bare types (e.g. `"uint256,address"`) |
| `args` | Yes | Array of argument values as strings (addresses as `0x`-hex, integers as decimal) |

#### `abi_decode`

ABI-decode hex-encoded calldata or return data.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `signature` | Yes | Function signature or bare types string |
| `data` | Yes | Hex-encoded data to decode (0x-prefixed) |

#### `convert_amount`

Convert between human-readable and base unit amounts.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `amount` | Yes | Amount to convert |
| `decimals` | Yes | Number of decimal places (e.g. `18` for ETH, `6` for USDC) |
| `direction` | Yes | `"to_base"` (human→base) or `"to_human"` (base→human) |

---

### Aave V3 (Ethereum)

Aave V3 tools operate on Ethereum mainnet only.

#### `aave_v3_deposit`

Build unsigned transactions to deposit (supply) tokens into Aave V3. Returns an approve tx and a supply tx.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `asset` | Yes | ERC-20 token contract address (0x-prefixed) |
| `amount` | Yes | Amount in human-readable units (e.g. `"100.5"`) or `"max"` for full balance |
| `address` | No | Depositor's Ethereum address. Falls back to vault-derived if omitted. |

#### `aave_v3_withdraw`

Build an unsigned transaction to withdraw tokens from Aave V3.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `asset` | Yes | ERC-20 token contract address (0x-prefixed) |
| `amount` | Yes | Amount in human-readable units or `"max"` for full withdrawal |
| `address` | No | Withdrawer's Ethereum address. Falls back to vault-derived if omitted. |

#### `aave_v3_borrow`

Build an unsigned transaction to borrow tokens from Aave V3 at variable rate.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `asset` | Yes | ERC-20 token contract address (0x-prefixed) |
| `amount` | Yes | Amount in human-readable units |
| `address` | No | Borrower's Ethereum address. Falls back to vault-derived if omitted. |

#### `aave_v3_repay`

Build unsigned transactions to repay a borrow on Aave V3. Returns an approve tx and a repay tx.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `asset` | Yes | ERC-20 token contract address (0x-prefixed) |
| `amount` | Yes | Amount in human-readable units or `"max"` to repay entire debt |
| `address` | No | Repayer's Ethereum address. Falls back to vault-derived if omitted. |

#### `aave_v3_get_balances`

Query Aave V3 account summary: total collateral, debt, available borrows (USD), LTV, and health factor.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `address` | No | Ethereum address. Falls back to vault-derived if omitted. |

#### `aave_v3_get_rates`

Query Aave V3 supply APY, variable borrow APY, and reserve configuration for a token.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `asset` | Yes | ERC-20 token contract address (0x-prefixed) |

---

## MCP Client Configuration

### Stdio (default)

```json
{
  "mcpServers": {
    "vultisig": {
      "command": "/path/to/mcp-server",
      "env": {
        "EVM_ETHEREUM_URL": "https://ethereum-rpc.publicnode.com"
      }
    }
  }
}
```

### HTTP

```bash
# Start the server
./mcp-server -http :8080

# MCP endpoint: POST/GET/DELETE http://localhost:8080/mcp
```

```json
{
  "mcpServers": {
    "vultisig": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```