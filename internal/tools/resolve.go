package tools

import (
	"fmt"

	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/vultisig/mcp/internal/vault"
)

// resolveAddress returns an explicit address if provided, otherwise derives
// the Ethereum address from the vault's ECDSA public key and chain code.
func resolveAddress(explicit string, v vault.Info) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	addr, _, _, err := address.GetAddress(v.ECDSAPublicKey, v.ChainCode, common.Ethereum)
	if err != nil {
		return "", fmt.Errorf("derive ethereum address: %w", err)
	}
	return addr, nil
}
