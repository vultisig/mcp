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

| Variable | Default | Description |
|----------|---------|-------------|
| `ETH_RPC_URL` | `https://ethereum-rpc.publicnode.com` | Ethereum JSON-RPC endpoint |

## Architecture

```
cmd/mcp-server/main.go          # Entry point, wiring
internal/config/config.go        # Env-based configuration
internal/logging/logging.go      # Tool call & session lifecycle logging
internal/vault/store.go          # Per-session vault state (sync.RWMutex map)
internal/ethereum/client.go      # go-ethereum ethclient wrapper
internal/tools/
  tools.go                       # RegisterAll() orchestrator
  resolve.go                     # Shared address resolution (explicit or vault-derived)
  set_vault_info.go              # Store vault keys in session
  get_eth_balance.go             # Query native ETH balance
  get_token_balance.go           # Query ERC-20 token balance
```

## Key Dependencies

- `github.com/mark3labs/mcp-go` — MCP server framework (stdio, tool registration)
- `github.com/vultisig/vultisig-go` — Address derivation from vault keys
- `github.com/ethereum/go-ethereum` — Ethereum JSON-RPC client

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
