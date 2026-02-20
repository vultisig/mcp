package types

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/mark3labs/mcp-go/mcp"
)

// TransactionResult is the top-level JSON envelope returned by every
// transaction-building MCP tool.
type TransactionResult struct {
	Transactions []Transaction `json:"transactions"`
}

// Transaction represents a single signable transaction in a multi-tx flow.
type Transaction struct {
	Sequence      int               `json:"sequence"`
	Chain         string            `json:"chain"`
	ChainID       string            `json:"chain_id"`
	Action        string            `json:"action"`
	SigningMode   string            `json:"signing_mode"`
	UnsignedTxHex string            `json:"unsigned_tx_hex"`
	TxDetails     map[string]string `json:"tx_details"`
}

// Signing modes.
const (
	SigningModeECDSA = "ecdsa_secp256k1"
	SigningModeEdDSA = "eddsa_ed25519"
)

// EVM tx encoding types.
const (
	TxEncodingLegacyRLP  = "legacy_rlp"
	TxEncodingEIP1559RLP = "eip1559_rlp"
)

// evmChainNames maps chain ID to a human-readable chain name.
var evmChainNames = map[uint64]string{
	1:     "ethereum",
	10:    "optimism",
	56:    "bsc",
	137:   "polygon",
	324:   "zksync",
	5000:  "mantle",
	8453:  "base",
	42161: "arbitrum",
	43114: "avalanche",
}

// EVMChainName returns the canonical chain name for a given chain ID,
// falling back to "evm-<id>" for unknown chains.
func EVMChainName(chainID *big.Int) string {
	if name, ok := evmChainNames[chainID.Uint64()]; ok {
		return name
	}
	return fmt.Sprintf("evm-%s", chainID.String())
}

// ToToolResult serialises the TransactionResult as JSON and wraps it in an
// MCP tool result.
func (r *TransactionResult) ToToolResult() (*mcp.CallToolResult, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("marshal transaction result: %w", err)
	}
	return mcp.NewToolResultText(string(data)), nil
}
