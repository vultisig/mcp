package resolve

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
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

// VaultInfoFromArgs extracts inline vault info from tool call arguments.
// Returns nil if any of the three required fields are missing.
func VaultInfoFromArgs(req mcp.CallToolRequest) *vault.Info {
	ecdsa, err1 := req.RequireString("ecdsa_public_key")
	eddsa, err2 := req.RequireString("eddsa_public_key")
	cc, err3 := req.RequireString("chain_code")
	if err1 != nil || err2 != nil || err3 != nil {
		return nil
	}
	return &vault.Info{ECDSAPublicKey: ecdsa, EdDSAPublicKey: eddsa, ChainCode: cc}
}

// ResolveVault returns vault info from inline args first, falling back to
// the session store. This allows both stateless (HTTP) and stateful (stdio)
// callers to work.
func ResolveVault(req mcp.CallToolRequest, ctx context.Context, store *vault.Store) *vault.Info {
	if vi := VaultInfoFromArgs(req); vi != nil {
		return vi
	}
	sessionID := SessionIDFromCtx(ctx)
	if v, ok := store.Get(sessionID); ok {
		return &v
	}
	return nil
}

// EVMAddress returns an explicit address if non-empty, otherwise derives
// the Ethereum address from the vault's ECDSA public key and chain code.
func EVMAddress(explicit string, vi *vault.Info) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if vi == nil {
		return "", fmt.Errorf("no address provided and no vault info available. Pass the user's address explicitly via the 'address' parameter (from wallet context)")
	}

	addr, _, _, err := address.GetAddress(vi.ECDSAPublicKey, vi.ChainCode, common.Ethereum)
	if err != nil {
		return "", fmt.Errorf("derive ethereum address: %w", err)
	}
	return addr, nil
}

// ChainAddress returns an explicit address if non-empty, otherwise derives
// the address for the given chain from the vault's key material.
// Uses EdDSA key for EdDSA chains (Solana, Sui, etc.) and ECDSA key for others.
func ChainAddress(explicit string, vi *vault.Info, chainName string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if vi == nil {
		return "", fmt.Errorf("no address provided and no vault info available. Pass the user's address explicitly via the 'address' parameter (from wallet context)")
	}

	chain, err := common.FromString(chainName)
	if err != nil {
		return "", fmt.Errorf("unsupported chain %q: %w", chainName, err)
	}

	rootPubKey := vi.ECDSAPublicKey
	if chain.IsEdDSA() {
		rootPubKey = vi.EdDSAPublicKey
	}

	addr, _, _, err := address.GetAddress(rootPubKey, vi.ChainCode, chain)
	if err != nil {
		return "", fmt.Errorf("derive %s address: %w", chainName, err)
	}
	return addr, nil
}
