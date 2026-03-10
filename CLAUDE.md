# Vultisig MCP Server

## Security Tier

STANDARD

## Critical Boundaries

- `internal/tools/` — Each tool is a file. Tool parameters define what agents can do.
- `internal/protocols/` — DeFi protocol handlers. Incorrect ABI encoding = lost funds.
- `internal/vault/store.go` — Per-session vault state. Thread-safety critical.
- Skill files (`internal/skills/files/`) — Markdown docs that teach agents multi-step workflows.

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
| `BLOCKCHAIR_API_URL` | `https://api.vultisig.com/blockchair` | Blockchair proxy base URL for UTXO chain queries |
| `THORCHAIN_URL` | `https://thornode.ninerealms.com` | THORChain node base URL for fee rates (BTC, LTC, DOGE, BCH) |
| `MAYACHAIN_URL` | `https://mayanode.mayachain.info` | MayaChain node base URL for fee rates (DASH, ZEC) |
| `SOLANA_RPC_URL` | `https://api.mainnet-beta.solana.com` | Solana JSON-RPC endpoint |
| `JUPITER_API_URL` | `https://api.jup.ag` | Jupiter DEX aggregator API base URL |
| `XRP_RPC_URL` | `https://s1.ripple.com:51234` | XRP Ledger JSON-RPC endpoint |
| `VERIFIER_URL` | `""` | Verifier service base URL — enables plugin management tools when set |
| `VERIFIER_API_KEY` | `""` | Service-to-service key sent as `X-Service-Key` for user-specific verifier queries |
| `GAIA_RPC_URL` | `https://cosmos-rest.publicnode.com` | Cosmos Hub (Gaia) REST endpoint |

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
internal/mayachain/client.go     # MayaChain node client (fee rates via inbound_addresses)
internal/tools/
  tools.go                       # RegisterAll() orchestrator (tools + protocols)
  set_vault_info.go              # Store vault keys in session
  get_address.go                 # Derive address for any supported chain
  evm_get_balance.go             # Query native coin balance (any EVM chain)
  evm_get_token_balance.go       # Query ERC-20 token balance (any EVM chain)
  evm_check_allowance.go         # Query ERC-20 allowance for a spender
  evm_call.go                    # Execute eth_call read-only (any EVM chain)
  evm_tx_info.go                 # Get nonce/gas/fees for tx building
  build_evm_tx.go                # Return EIP-1559 tx args for client to assemble and sign
  build_swap_tx.go               # Build swap transaction via recipes SDK
  search_token.go                # Token discovery via CoinGecko API
  abi_encode.go                  # ABI encode function calls / raw args
  abi_decode.go                  # ABI decode output data
  convert_amount.go
  plugin.go                      # Plugin management tools (recipe schema, policy, billing)
  registry.go
internal/verifier/client.go      # Verifier service client (plugin/billing queries)
internal/coingecko/client.go     # CoinGecko REST API client
internal/blockchair/client.go    # Blockchair UTXO chain API client (via Vultisig proxy)
internal/thorchain/client.go     # THORChain node client (fee rates via inbound_addresses)
internal/solana/client.go        # Solana RPC client wrapper
internal/jupiter/client.go       # Jupiter DEX aggregator API client
internal/xrp/client.go           # XRP Ledger JSON-RPC client
internal/gaia/client.go          # Cosmos Hub (Gaia) REST client
internal/tools/
  btc_fee_rate.go                # Get BTC recommended fee rate from THORChain
  build_btc_send.go              # Return BTC send/swap args for client to build PSBT
  ltc_fee_rate.go                # LTC fee rate from THORChain
  build_ltc_send.go              # Return LTC send/swap args for client to build PSBT
  doge_fee_rate.go               # DOGE fee rate from THORChain
  build_doge_send.go             # Return DOGE send/swap args for client to build PSBT
  bch_fee_rate.go                # BCH fee rate from THORChain
  build_bch_send.go              # Return BCH send/swap args for client to build PSBT
  dash_fee_rate.go               # DASH fee rate from MayaChain
  build_dash_send.go             # Return DASH send/swap args for client to build PSBT
  build_zec_send.go              # Return Zcash send/swap args (client builds Zcash v4 tx, ZIP-317 fee)
  maya_fee_rate.go               # Fee rate for any MayaChain-supported chain
  get_sol_balance.go             # Query native SOL balance
  get_spl_token_balance.go       # Query SPL token balance
  build_solana_tx.go             # Return SOL transfer args for client to build and sign
  build_spl_transfer_tx.go       # Return SPL transfer args with derived ATA addresses
  build_solana_swap.go           # Return Solana swap args via Jupiter (quote + params)
  get_xrp_balance.go             # Query native XRP balance
  build_xrp_send.go              # Return XRP Payment args (live fee/sequence fetched)
  get_atom_balance.go            # Query native ATOM balance on Cosmos Hub
  build_gaia_send.go             # Return Cosmos ATOM transfer args (with optional memo for swaps)
```

## Key Dependencies

- `github.com/mark3labs/mcp-go` — MCP server framework (stdio, tool registration)
- `github.com/kelseyhightower/envconfig` — Struct-tag-based env config
- `github.com/vultisig/vultisig-go` — Address derivation from vault keys
- `github.com/vultisig/recipes` — SDK for swap operations and address derivation helpers
- `github.com/ethereum/go-ethereum` — Ethereum JSON-RPC client
- `github.com/btcsuite/btcd` — Bitcoin/UTXO address validation (txscript, base58)
- `github.com/gagliardetto/solana-go` — Solana RPC client and address/ATA utilities
- `github.com/xyield/xrpl-go` — XRP Ledger address utilities

## EVM Chains

All EVM tools accept a `chain` parameter (default: `Ethereum`):
`Ethereum`, `BSC`, `Polygon`, `Avalanche`, `Arbitrum`, `Optimism`, `Base`, `Blast`, `Mantle`, `Zksync`

All chains use EIP-1559 (type 2) transactions. No legacy tx fallback.

## UTXO Chains

Build tools validate addresses and return args; the client is responsible for fetching UTXOs and building the transaction.

| Chain | Tool prefix | Fee source | Client tx format |
|-------|-------------|------------|-----------------|
| Bitcoin | `btc_*` | THORChain ("BTC") | PSBT |
| Litecoin | `ltc_*` | THORChain ("LTC") | PSBT |
| Dogecoin | `doge_*` | THORChain ("DOGE") | PSBT |
| Bitcoin-Cash | `bch_*` | THORChain ("BCH") | PSBT |
| Dash | `dash_*` | MayaChain ("DASH") | PSBT |
| Zcash | `build_zec_send` | ZIP-317 (client-side) | Zcash v4 |

No `fee_rate` param for ZEC — ZIP-317 fee is computed by the client.
All build tools return `tx_encoding` field (`"psbt"` or `"zcash_v4"`) indicating the expected client format.

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
- Default URLs belong in `config.go` (envconfig defaults), not in client constructors.
- Doc comments on all exported functions, types, methods, and packages (`// FunctionName does ...`)

## Knowledge Base

For deeper context, see [vultisig-knowledge](https://github.com/vultisig/vultisig-knowledge). Read only when needed:

| Situation | Read |
|-----------|------|
| First time in this repo | [repos/mcp.md](https://github.com/vultisig/vultisig-knowledge/blob/main/repos/mcp.md) |
| Touching crypto/signing code | [architecture/mpc-tss-explained.md](https://github.com/vultisig/vultisig-knowledge/blob/main/architecture/mpc-tss-explained.md) |
| Working with agent-backend | [repos/agent-backend.md](https://github.com/vultisig/vultisig-knowledge/blob/main/repos/agent-backend.md) |
| Cross-repo gotchas | [coding/gotchas.md](https://github.com/vultisig/vultisig-knowledge/blob/main/coding/gotchas.md) |
| Checking dependency versions | [coding/dependencies.md](https://github.com/vultisig/vultisig-knowledge/blob/main/coding/dependencies.md) |
