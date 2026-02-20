# Vultisig MCP Server

An [MCP](https://modelcontextprotocol.io/) server that exposes Vultisig vault operations and Ethereum balance queries to LLM-powered agents. Supports both stdio and HTTP transports.

## Quick Start

```bash
# Build
go build -o mcp-server ./cmd/mcp-server/

# Run over stdio (default)
./mcp-server

# Run over HTTP on port 8080
./mcp-server -http :8080

# With a custom RPC endpoint
ETH_RPC_URL=https://your-rpc.example.com ./mcp-server -http :8080
```

## Tools

### `set_vault_info`

Store vault key material for the current session. Must be called before vault-derived balance queries.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `ecdsa_public_key` | Yes | Hex-encoded compressed ECDSA public key (66 hex chars) |
| `eddsa_public_key` | Yes | Hex-encoded EdDSA public key (64 hex chars) |
| `chain_code` | Yes | Hex-encoded 32-byte chain code for BIP-32 derivation |

### `get_address`

Derive the address for a given blockchain network from the vault's key material. Requires `set_vault_info` first.

| Parameter | Required | Description |
|-----------|----------|-------------|
| `chain` | Yes | Blockchain network name (see supported chains below) |

Supported chains: Arbitrum, Avalanche, Base, Bitcoin, Bitcoin-Cash, Blast, BSC, Cosmos, CronosChain, Dash, Dogecoin, Dydx, Ethereum, Kujira, Litecoin, Mantle, MayaChain, Noble, Optimism, Osmosis, Polygon, Ripple, Solana, Sui, Terra, TerraClassic, THORChain, Tron, Zcash, Zksync.

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

### Stdio (default)

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
