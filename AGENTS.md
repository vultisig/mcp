# mcp — Agent Reference

## Overview

MCP server exposing crypto operations (22+ tools, 4 skills) to AI agents. Supports EVM chains, UTXO chains (BTC, LTC, DOGE, BCH, DASH, ZEC), Solana, and XRP. stdio or HTTP transport.

## Quick Start

```bash
git clone https://github.com/vultisig/mcp.git
cd mcp
go build ./cmd/mcp-server/
go test ./... -race
# Run stdio: go run ./cmd/mcp-server/
# Run HTTP: go run ./cmd/mcp-server/ -http :8080
```

## Before You Change Code

1. Run `go test ./... -race`
2. If adding a tool: create a new file in `internal/tools/`, register in `tools.go` RegisterAll()
3. If adding a protocol: follow the Aave V3 pattern in `internal/protocols/`
4. If touching ABI encoding: test thoroughly — encoding errors = lost funds

## Patterns

- One file per tool in `internal/tools/`
- Tools register via `mcp-go` server.AddTool()
- Vault context stored per-session (thread-safe sync.RWMutex map)
- Skills are markdown files in `internal/skills/files/`
- EVM client pool lazily creates per-chain clients on first use
- User-facing errors: `mcp.NewToolResultError()` (IsError: true)
- Go errors for protocol-level failures only
- All logs to stderr (stdout reserved for stdio MCP protocol)

## Security Notes

- Never broadcast transactions — clients handle that
- ABI encoding errors = lost funds. Test encoding/decoding thoroughly.
- Vault keys stored in-memory only (per-session, never persisted)
- ERC-20 queries use raw ABI encoding (no abigen)

## Knowledge Base

For deeper context beyond this file, see [vultisig-knowledge](https://github.com/vultisig/vultisig-knowledge).

Key docs for this repo:
- [repos/mcp.md](https://github.com/vultisig/vultisig-knowledge/blob/main/repos/mcp.md)
- [architecture/mpc-tss-explained.md](https://github.com/vultisig/vultisig-knowledge/blob/main/architecture/mpc-tss-explained.md)
- [coding/gotchas.md](https://github.com/vultisig/vultisig-knowledge/blob/main/coding/gotchas.md)
- [coding/dependencies.md](https://github.com/vultisig/vultisig-knowledge/blob/main/coding/dependencies.md)
