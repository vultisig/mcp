package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vultisig/vultisig-go/address"
	"github.com/vultisig/vultisig-go/common"

	"github.com/vultisig/mcp/internal/pumpfun"
	"github.com/vultisig/mcp/internal/resolve"
	solanaclient "github.com/vultisig/mcp/internal/solana"
	"github.com/vultisig/mcp/internal/vault"
)

func newBuildPumpfunCreateTool() mcp.Tool {
	return mcp.NewTool("build_pumpfun_create",
		mcp.WithDescription(
			"Return parameters for creating a new pump.fun token on Solana. "+
				"The client is responsible for generating the mint keypair, uploading metadata to IPFS, "+
				"and building/signing the transaction.",
		),
		mcp.WithString("from",
			mcp.Description("Creator's Solana address (base58). Optional if vault info is set."),
		),
		mcp.WithString("name",
			mcp.Description("Token name (max 32 characters)."),
			mcp.Required(),
		),
		mcp.WithString("symbol",
			mcp.Description("Token ticker symbol (max 10 characters)."),
			mcp.Required(),
		),
		mcp.WithString("metadata_uri",
			mcp.Description("IPFS metadata URI (e.g. from pump.fun/api/ipfs upload)."),
			mcp.Required(),
		),
		mcp.WithString("mint_address",
			mcp.Description("Public key (base58) of the new mint keypair. Client must generate this keypair."),
			mcp.Required(),
		),
		mcp.WithString("initial_buy_amount",
			mcp.Description("Optional initial buy amount in lamports."),
		),
	)
}

func handleBuildPumpfunCreate(store *vault.Store) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError("missing name parameter"), nil
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return mcp.NewToolResultError("name must not be empty"), nil
		}
		// on-chain limit is byte-based
		if len(name) > 32 {
			return mcp.NewToolResultError(fmt.Sprintf("name too long: %d bytes, max 32", len(name))), nil
		}

		symbol, err := req.RequireString("symbol")
		if err != nil {
			return mcp.NewToolResultError("missing symbol parameter"), nil
		}
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			return mcp.NewToolResultError("symbol must not be empty"), nil
		}
		if len(symbol) > 10 {
			return mcp.NewToolResultError(fmt.Sprintf("symbol too long: %d bytes, max 10", len(symbol))), nil
		}

		metadataURI, err := req.RequireString("metadata_uri")
		if err != nil {
			return mcp.NewToolResultError("missing metadata_uri parameter"), nil
		}
		metadataURI = strings.TrimSpace(metadataURI)
		if metadataURI == "" {
			return mcp.NewToolResultError("metadata_uri must not be empty"), nil
		}

		mintStr, err := req.RequireString("mint_address")
		if err != nil {
			return mcp.NewToolResultError("missing mint_address parameter"), nil
		}
		mintPubkey, err := solanaclient.ParsePublicKey(mintStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid mint_address: %v", err)), nil
		}

		sessionID := resolve.SessionIDFromCtx(ctx)
		v, ok := store.Get(sessionID)
		if !ok {
			return mcp.NewToolResultError("no vault info set — call set_vault_info first"), nil
		}

		fromAddr, _, _, err := address.GetAddress(v.EdDSAPublicKey, v.ChainCode, common.Solana)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("derive Solana address: %v", err)), nil
		}

		explicit := req.GetString("from", "")
		if explicit != "" {
			_, err = solanaclient.ParsePublicKey(explicit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid from address: %v", err)), nil
			}
			if explicit != fromAddr {
				return mcp.NewToolResultError(fmt.Sprintf(
					"explicit from address %q does not match vault-derived address %q", explicit, fromAddr)), nil
			}
		}

		bondingCurvePDA, _, err := pumpfun.DeriveBondingCurvePDA(mintPubkey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("derive bonding curve PDA: %v", err)), nil
		}

		bondingCurveATA, _, err := pumpfun.DeriveBondingCurveATA(bondingCurvePDA, mintPubkey)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("derive bonding curve ATA: %v", err)), nil
		}

		result := map[string]any{
			"chain":             "Solana",
			"action":            "pumpfun_create",
			"signing_mode":      "eddsa_ed25519",
			"from":              fromAddr,
			"mint":              mintStr,
			"name":              name,
			"symbol":            symbol,
			"metadata_uri":      metadataURI,
			"program_id":        pumpfun.ProgramID.String(),
			"bonding_curve":     bondingCurvePDA.String(),
			"bonding_curve_ata": bondingCurveATA.String(),
		}

		initialBuyStr := req.GetString("initial_buy_amount", "")
		if initialBuyStr != "" {
			initialBuy, err := strconv.ParseUint(initialBuyStr, 10, 64)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid initial_buy_amount format: %q", initialBuyStr)), nil
			}
			if initialBuy == 0 {
				return mcp.NewToolResultError(fmt.Sprintf("initial_buy_amount must be greater than zero: %q", initialBuyStr)), nil
			}
			result["initial_buy_amount"] = initialBuyStr
		}

		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("marshal result: %v", err)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
