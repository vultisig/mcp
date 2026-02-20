# Vultisig MCP Server

An [MCP](https://modelcontextprotocol.io/) server that exposes Vultisig vault operations and Ethereum balance queries to LLM-powered agents. Communicates over stdio transport.

## Quick Start

```bash
# Build
go build -o mcp-server ./cmd/mcp-server/

# Run (uses public Ethereum RPC by default)
./mcp-server

# Or with a custom RPC endpoint
ETH_RPC_URL=https://your-rpc.example.com ./mcp-server
```

## Tools

### `set_vault_info`

Store vault key material for the current session. Must be called before vault-derived balance queries.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `ecdsa_public_key` | Yes | Hex-encoded compressed ECDSA public key (66 hex chars) |
| `eddsa_public_key` | Yes | Hex-encoded EdDSA public key (64 hex chars) |
| `chain_code` | Yes | Hex-encoded 32-byte chain code for BIP-32 derivation |

### `get_eth_balance`

Query the native ETH balance. Derives the address from vault keys if not provided.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `address` | No | Ethereum address (0x-prefixed). Falls back to vault-derived address. |

### `get_token_balance`

Query an ERC-20 token balance. Derives the holder address from vault keys if not provided.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `contract_address` | Yes | ERC-20 token contract address (0x-prefixed) |
| `address` | No | Holder address (0x-prefixed). Falls back to vault-derived address. |

## MCP Client Configuration

```json
{
  "mcpServers": {
    "vultisig": {
      "command": "/path/to/mcp-server",
      "env": {
        "ETH_RPC_URL": "https://ethereum-rpc.publicnode.com"
      }
    }
  }
}
```
