# Vultisig MCP Server

## Build & Run

```bash
go build ./cmd/mcp-server/       # Build
go test ./...                     # Run all tests
go vet ./...                      # Lint
./mcp-server                      # Run (stdio transport, default)
./mcp-server -http :8080          # Run (HTTP transport on port 8080)
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-http` | (empty) | HTTP listen address (e.g. `:8080`). If empty, serves over stdio. |

## Environment Variables

Config uses `github.com/kelseyhightower/envconfig`. All EVM RPC URLs default to public nodes.

| Variable | Default | Description |
|----------|---------|-------------|
| `EVM_ETHEREUM_URL` | `https://ethereum-rpc.publicnode.com` | Ethereum JSON-RPC endpoint |
| `EVM_BSC_URL` | `https://bsc-rpc.publicnode.com` | BNB Smart Chain endpoint |
| `EVM_POLYGON_URL` | `https://polygon-bor-rpc.publicnode.com` | Polygon endpoint |
| `EVM_AVALANCHE_URL` | `https://avalanche-c-chain-rpc.publicnode.com` | Avalanche C-Chain endpoint |
| `EVM_ARBITRUM_URL` | `https://arbitrum-one-rpc.publicnode.com` | Arbitrum One endpoint |
| `EVM_OPTIMISM_URL` | `https://optimism-rpc.publicnode.com` | Optimism endpoint |
| `EVM_BASE_URL` | `https://base-rpc.publicnode.com` | Base endpoint |
| `EVM_BLAST_URL` | `https://blast-rpc.publicnode.com` | Blast endpoint |
| `EVM_MANTLE_URL` | `https://mantle-rpc.publicnode.com` | Mantle endpoint |
| `EVM_ZKSYNC_URL` | `https://mainnet.era.zksync.io` | zkSync Era endpoint |
| `COINGECKO_API_KEY` | (empty) | CoinGecko API key (optional, raises rate limits) |
| `BLOCKCHAIR_API_URL` | `https://api.vultisig.com/blockchair` | Blockchair proxy base URL for UTXO chain queries |
| `THORCHAIN_URL` | `https://thornode.ninerealms.com` | THORChain node base URL for fee rates |

## Architecture

```
cmd/mcp-server/main.go          # Entry point, wiring
internal/config/config.go        # Env-based configuration (envconfig)
internal/logging/logging.go      # Tool call & session lifecycle logging
internal/vault/store.go          # Per-session vault state (sync.RWMutex map)
internal/evm/
  client.go                      # go-ethereum ethclient wrapper
  pool.go                        # Lazy-init per-chain client pool
  chains.go                      # Chain name → chain ID, ticker, default RPC URL
  format.go                      # FormatUnits, DecodeABIString helpers
internal/tools/
  tools.go                       # RegisterAll() orchestrator (tools + protocols)
  set_vault_info.go              # Store vault keys in session
  get_address.go                 # Derive address for any supported chain
  evm_get_balance.go             # Query native coin balance (any EVM chain)
  evm_get_token_balance.go       # Query ERC-20 token balance (any EVM chain)
  evm_check_allowance.go         # Query ERC-20 allowance for a spender
  evm_call.go                    # Execute eth_call read-only (any EVM chain)
  evm_tx_info.go                 # Get nonce/gas/fees for tx building
  build_evm_tx.go                # Build unsigned EIP-1559 transaction
  build_swap_tx.go               # Build swap transaction via recipes SDK
  build_utxo_tx.go               # Build unsigned UTXO transaction
  search_token.go                # Token discovery via CoinGecko API
  abi_encode.go                  # ABI encode function calls / raw args
  abi_decode.go                  # ABI decode output data
  convert_amount.go 
  registry.go 
internal/coingecko/client.go     # CoinGecko REST API client
internal/blockchair/client.go    # Blockchair UTXO chain API client (via Vultisig proxy)
internal/thorchain/client.go     # THORChain node client (fee rates via inbound_addresses)
internal/tools/
  get_utxo_balance.go            # Query UTXO chain address balance
  get_utxo_transactions.go       # List recent tx hashes for UTXO chain address
  list_utxos.go                  # List unspent transaction outputs
  btc_fee_rate.go                # Get BTC recommended fee rate from THORChain
  build_btc_send.go    
```

## Key Dependencies

- `github.com/mark3labs/mcp-go` — MCP server framework (stdio, tool registration)
- `github.com/kelseyhightower/envconfig` — Struct-tag-based env config
- `github.com/vultisig/vultisig-go` — Address derivation from vault keys
- `github.com/vultisig/recipes` — SDK for EVM, BTC (UTXO builder, PSBT), and swap operations
- `github.com/ethereum/go-ethereum` — Ethereum JSON-RPC client
- `github.com/btcsuite/btcd` — Bitcoin transaction primitives (wire, txscript, chainhash)

## EVM Chains

All EVM tools accept a `chain` parameter (default: `Ethereum`):
`Ethereum`, `BSC`, `Polygon`, `Avalanche`, `Arbitrum`, `Optimism`, `Base`, `Blast`, `Mantle`, `Zksync`

All chains use EIP-1559 (type 2) transactions. No legacy tx fallback.

## Logging

All logs go to stderr (prefixed `[mcp]`), keeping stdout clean for the stdio MCP protocol. Log tags:

- `[CALL]` — tool invocation with arguments
- `[OK]` — tool completed successfully with result preview
- `[FAIL]` — tool returned a user-facing error (IsError: true)
- `[ERROR]` — tool returned a Go error (protocol-level)
- `[SESSION]` — session registered/unregistered
- `[INIT]` — client initialized with name and version
- `[RPC_ERR]` — any JSON-RPC level error

Logging is implemented via mcp-go `ToolHandlerMiddleware` (for tool call timing) and `Hooks` (for session lifecycle).

## Conventions

- User-facing errors use `mcp.NewToolResultError()` (IsError: true). Go errors for protocol-level failures only.
- Session ID extracted via `server.ClientSessionFromContext(ctx).SessionID()`, falls back to `"default"`.
- ERC-20 queries use raw ABI encoding (no abigen).
- Replace directives in go.mod are required for `github.com/gogo/protobuf` and `github.com/agl/ed25519`.
- EVM client pool (`internal/evm.Pool`) lazily creates per-chain clients on first use and caches them.
