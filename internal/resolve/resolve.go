package resolve

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/server"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/vultisig/mcp/internal/vault"
)

// SessionIDFromCtx extracts the MCP session ID from the context,
// falling back to "default" when no session is present.
func SessionIDFromCtx(ctx context.Context) string {
	if sess := server.ClientSessionFromContext(ctx); sess != nil {
		return sess.SessionID()
	}
	return "default"
}

// EVMAddress returns an explicit address if non-empty, otherwise derives
// the Ethereum address from the vault's ECDSA public key and chain code.
func EVMAddress(explicit, sessionID string, store *vault.Store) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	v, ok := store.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("no address provided and no vault info set. Pass the user's address explicitly via the 'address' parameter (from wallet context) — do NOT call set_vault_info")
	}

	addr, _, _, err := address.GetAddress(v.ECDSAPublicKey, v.ChainCode, common.Ethereum)
	if err != nil {
		return "", fmt.Errorf("derive ethereum address: %w", err)
	}
	return addr, nil
}

// ChainAddress returns an explicit address if non-empty, otherwise derives
// the address for the given chain from the vault's key material.
// Uses EdDSA key for EdDSA chains (Solana, Sui, etc.) and ECDSA key for others.
func ChainAddress(explicit, sessionID string, store *vault.Store, chainName string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	v, ok := store.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("no address provided and no vault info set. Pass the user's address explicitly via the 'address' parameter (from wallet context) — do NOT call set_vault_info")
	}

	chain, err := common.FromString(chainName)
	if err != nil {
		return "", fmt.Errorf("unsupported chain %q: %w", chainName, err)
	}

	rootPubKey := v.ECDSAPublicKey
	if chain.IsEdDSA() {
		rootPubKey = v.EdDSAPublicKey
	}

	addr, _, _, err := address.GetAddress(rootPubKey, v.ChainCode, chain)
	if err != nil {
		return "", fmt.Errorf("derive %s address: %w", chainName, err)
	}
	return addr, nil
}
