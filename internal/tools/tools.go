package tools

import (
	"fmt"

	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/recipes/sdk/swap"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/coingecko"
	evmclient "github.com/vultisig/mcp/internal/evm"
	"github.com/vultisig/mcp/internal/jupiter"
	"github.com/vultisig/mcp/internal/mayachain"
	"github.com/vultisig/mcp/internal/protocols"
	"github.com/vultisig/mcp/internal/thorchain"
	"github.com/vultisig/mcp/internal/toolmeta"
	pmtools "github.com/vultisig/mcp/internal/tools/polymarket"
	solanaclient "github.com/vultisig/mcp/internal/solana"
	"github.com/vultisig/mcp/internal/vault"
	xrpclient "github.com/vultisig/mcp/internal/xrp"
)

func RegisterAll(s *server.MCPServer, store *vault.Store, pool *evmclient.Pool, cgClient *coingecko.Client, bcClient *blockchair.Client, swapSvc *swap.Service, tcClient *thorchain.Client, mcClient *mayachain.Client, solClient *solanaclient.Client, jupClient *jupiter.Client, xrpClient *xrpclient.Client) error {
	// Utility tools (always-on)
	toolmeta.Register(s, newSetVaultInfoTool(), handleSetVaultInfo(store), "utility")
	toolmeta.Register(s, newGetAddressTool(), handleGetAddress(store), "utility")
	toolmeta.Register(s, newSearchTokenTool(), handleSearchToken(cgClient), "utility")
	toolmeta.Register(s, newGetPriceTool(), handleGetPrice(cgClient), "utility")
	toolmeta.Register(s, newGetTxStatusTool(), handleGetTxStatus(pool, bcClient, solClient, xrpClient), "utility")
	toolmeta.Register(s, newConvertAmountTool(), handleConvertAmount(), "utility")

	// Swap
	toolmeta.Register(s, newBuildSwapTxTool(), handleBuildSwapTx(swapSvc), "swap")

	// EVM tools
	toolmeta.Register(s, newEVMGetBalanceTool(), handleEVMGetBalance(store, pool), "balance", "evm")
	toolmeta.Register(s, newEVMGetTokenBalanceTool(), handleEVMGetTokenBalance(store, pool), "balance", "evm")
	toolmeta.Register(s, newEVMCheckAllowanceTool(), handleEVMCheckAllowance(store, pool), "contract", "evm")
	toolmeta.Register(s, newEVMCallTool(), handleEVMCall(pool), "contract", "evm")
	toolmeta.Register(s, newEVMTxInfoTool(), handleEVMTxInfo(store, pool), "contract", "evm", "fee")
	toolmeta.Register(s, newBuildEVMTxTool(), handleBuildEVMTx(), "send", "evm")

	// ABI tools
	toolmeta.Register(s, newABIEncodeTool(), handleABIEncode(), "contract")
	toolmeta.Register(s, newABIDecodeTool(), handleABIDecode(), "contract")

	// Bitcoin
	toolmeta.Register(s, newBTCFeeRateTool(), handleBTCFeeRate(tcClient), "fee", "bitcoin")
	toolmeta.Register(s, newBuildBTCSendTool(), handleBuildBTCSend(store, bcClient), "send", "bitcoin")

	// Litecoin
	toolmeta.Register(s, newLTCFeeRateTool(), handleLTCFeeRate(tcClient), "fee")
	toolmeta.Register(s, newBuildLTCSendTool(), handleBuildLTCSend(store, bcClient), "send")

	// Dogecoin
	toolmeta.Register(s, newDOGEFeeRateTool(), handleDOGEFeeRate(tcClient), "fee")
	toolmeta.Register(s, newBuildDOGESendTool(), handleBuildDOGESend(store, bcClient), "send")

	// Bitcoin Cash
	toolmeta.Register(s, newBCHFeeRateTool(), handleBCHFeeRate(tcClient), "fee")
	toolmeta.Register(s, newBuildBCHSendTool(), handleBuildBCHSend(store, bcClient), "send")

	// Dash
	toolmeta.Register(s, newDASHFeeRateTool(), handleDASHFeeRate(mcClient), "fee")
	toolmeta.Register(s, newBuildDASHSendTool(), handleBuildDASHSend(store, bcClient), "send")

	// Zcash
	toolmeta.Register(s, newBuildZECSendTool(), handleBuildZECSend(store, bcClient), "send")

	// MayaChain
	toolmeta.Register(s, newMayaFeeRateTool(), handleMayaFeeRate(mcClient), "fee")

	// Solana
	toolmeta.Register(s, newGetSOLBalanceTool(), handleGetSOLBalance(store, solClient), "balance", "solana")
	toolmeta.Register(s, newGetSPLTokenBalanceTool(), handleGetSPLTokenBalance(store, solClient), "balance", "solana")
	toolmeta.Register(s, newBuildSolanaTxTool(), handleBuildSolanaTx(store, solClient), "send", "solana")
	toolmeta.Register(s, newBuildSPLTransferTxTool(), handleBuildSPLTransferTx(store, solClient), "send", "solana")
	toolmeta.Register(s, newBuildSolanaSwapTool(), handleBuildSolanaSwap(store, jupClient), "swap", "solana")

	// XRP
	toolmeta.Register(s, newGetXRPBalanceTool(), handleGetXRPBalance(store, xrpClient), "balance", "xrp")
	toolmeta.Register(s, newBuildXRPSendTool(), handleBuildXRPSend(store, xrpClient), "send", "xrp")

	// Polymarket prediction market tools
	pmtools.RegisterAll(s, store, pool)

	err := protocols.RegisterAll(s, store, pool)
	if err != nil {
		return fmt.Errorf("register protocols: %w", err)
	}
	return nil
}
