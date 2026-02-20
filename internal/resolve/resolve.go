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
		return "", fmt.Errorf("no address provided and no vault info set for this session — call set_vault_info first")
	}

	addr, _, _, err := address.GetAddress(v.ECDSAPublicKey, v.ChainCode, common.Ethereum)
	if err != nil {
		return "", fmt.Errorf("derive ethereum address: %w", err)
	}
	return addr, nil
}
