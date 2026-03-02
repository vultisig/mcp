package types

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/vultisig/recipes/chain/evm"
)

type TransactionResult struct {
	Transactions []Transaction `json:"transactions"`
}

type Transaction struct {
	Sequence      int               `json:"sequence"`
	Chain         string            `json:"chain"`
	ChainID       string            `json:"chain_id"`
	Action        string            `json:"action"`
	SigningMode   string            `json:"signing_mode"`
	UnsignedTxHex string            `json:"unsigned_tx_hex"`
	TxDetails     map[string]string `json:"tx_details"`
}

const (
	SigningModeECDSA = "ecdsa_secp256k1"
	SigningModeEdDSA = "eddsa_ed25519"
)

const (
	TxEncodingLegacyRLP  = "legacy_rlp"
	TxEncodingEIP1559RLP = "eip1559_rlp"
	TxEncodingPSBT       = "psbt"
	TxEncodingZcashV4    = "zcash_v4"
)

var evmChainNames = buildEVMChainNames()

func buildEVMChainNames() map[int64]string {
	m := make(map[int64]string)
	for _, cfg := range evm.AllEVMChainConfigs() {
		m[cfg.EVMChainID] = cfg.ID
	}
	return m
}

func EVMChainName(chainID *big.Int) string {
	if name, ok := evmChainNames[chainID.Int64()]; ok {
		return name
	}
	return fmt.Sprintf("evm-%s", chainID.String())
}

func (r *TransactionResult) ToToolResult() (*mcp.CallToolResult, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("marshal transaction result: %w", err)
	}
	return mcp.NewToolResultText(string(data)), nil
}
