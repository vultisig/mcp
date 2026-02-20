package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

// supportedChains lists every chain name accepted by this tool, matching
// the chainToString map in vultisig-go/common. Polkadot and TON are
// excluded because address.GetAddress does not support them yet.
var supportedChains = []string{
	"Arbitrum",
	"Avalanche",
	"Base",
	"Bitcoin",
	"Bitcoin-Cash",
	"Blast",
	"BSC",
	"Cosmos",
	"CronosChain",
	"Dash",
	"Dogecoin",
	"Dydx",
	"Ethereum",
	"Kujira",
	"Litecoin",
	"Mantle",
	"MayaChain",
	"Noble",
	"Optimism",
	"Osmosis",
	"Polygon",
	"Ripple",
	"Solana",
	"Sui",
	"Terra",
	"TerraClassic",
	"THORChain",
	"Tron",
	"Zcash",
	"Zksync",
}

func newGetAddressTool() mcp.Tool {
	return mcp.NewTool("get_address",
		mcp.WithDescription("Derive the address for a given blockchain network from the vault's key material. Requires set_vault_info to be called first."),
		mcp.WithString("chain",
			mcp.Description("Blockchain network name"),
			mcp.Required(),
			mcp.Enum(supportedChains...),
		),
	)
}

func handleGetAddress(store *vault.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chainName, err := req.RequireString("chain")
		if err != nil {
			return mcp.NewToolResultError("missing chain parameter"), nil
		}

		chain, err := common.FromString(chainName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("unsupported chain %q: %v", chainName, err)), nil
		}

		sessionID := resolve.SessionIDFromCtx(ctx)
		v, ok := store.Get(sessionID)
		if !ok {
			return mcp.NewToolResultError("no vault info set for this session — call set_vault_info first"), nil
		}

		// Pick the right root public key based on whether the chain uses EdDSA.
		rootPubKey := v.ECDSAPublicKey
		if chain.IsEdDSA() {
			rootPubKey = v.EdDSAPublicKey
		}

		addr, derivedPubKey, isEdDSA, err := address.GetAddress(rootPubKey, v.ChainCode, chain)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to derive address for %s: %v", chainName, err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf(
			"Chain: %s\nAddress: %s\nDerived Public Key: %s\nKey Type: %s",
			chainName, addr, derivedPubKey, keyType(isEdDSA),
		)), nil
	}
}

func keyType(isEdDSA bool) string {
	if isEdDSA {
		return "EdDSA"
	}
	return "ECDSA"
}
