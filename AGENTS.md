# vultisig/mcp — Agent Reference

## Overview

Go-based MCP server exposing blockchain tools (balance queries, transaction building, token search, ABI encoding/decoding) for multi-chain cryptocurrency operations via the Model Context Protocol.

## Quick Start

```bash
git clone https://github.com/vultisig/mcp.git
cd mcp
go build ./cmd/mcp-server/
go test ./...
go vet ./...
```

## Before You Change Code

1. Run `go test ./...` and `go vet ./...` to establish baseline
2. If touching `internal/vault/`: extra caution — handles cryptographic key material per session
3. If touching `go.mod`: do not remove the `replace` directives for `gogo/protobuf` and `agl/ed25519`
4. If adding a new tool: register it in `internal/tools/tools.go` via `RegisterAll()`
5. If adding a new chain client: default URLs go in `internal/config/config.go` (envconfig defaults)

## Patterns

- MCP tools registered via `mcp-go` framework with tool handler functions
- User-facing errors: `mcp.NewToolResultError()` (IsError: true)
- Go errors: protocol-level failures only
- Per-session vault state: `internal/vault/store.go` with `sync.RWMutex`
- EVM client pool: lazy-init per-chain clients in `internal/evm/pool.go`
- No inline error assignment: define variable first, then `if err != nil`
- All logs to stderr, stdout reserved for MCP stdio protocol

## Security Notes

- Never log key material or vault shares
- Never interact with mainnet RPCs from agent context
- Do not modify `.env`, credential, or secret files
- Always validate blockchain addresses before use
- ERC-20 queries use raw ABI encoding (no abigen)

## Knowledge Base

For deeper context beyond this file, see [vultisig-knowledge](https://github.com/vultisig/vultisig-knowledge).

Key docs:
- [architecture/mpc-tss-explained.md](https://github.com/vultisig/vultisig-knowledge/blob/main/architecture/mpc-tss-explained.md)
- [architecture/signing-flow.md](https://github.com/vultisig/vultisig-knowledge/blob/main/architecture/signing-flow.md)
- [coding/gotchas.md](https://github.com/vultisig/vultisig-knowledge/blob/main/coding/gotchas.md)
