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
	pmtools "github.com/vultisig/mcp/internal/tools/polymarket"
	solanaclient "github.com/vultisig/mcp/internal/solana"
	"github.com/vultisig/mcp/internal/thorchain"
	"github.com/vultisig/mcp/internal/vault"
	xrpclient "github.com/vultisig/mcp/internal/xrp"
)

func RegisterAll(s *server.MCPServer, store *vault.Store, pool *evmclient.Pool, cgClient *coingecko.Client, bcClient *blockchair.Client, swapSvc *swap.Service, tcClient *thorchain.Client, mcClient *mayachain.Client, solClient *solanaclient.Client, jupClient *jupiter.Client, xrpClient *xrpclient.Client) error {
	s.AddTool(newSetVaultInfoTool(), handleSetVaultInfo(store))
	s.AddTool(newGetAddressTool(), handleGetAddress(store))
	s.AddTool(newEVMGetBalanceTool(), handleEVMGetBalance(store, pool))
	s.AddTool(newEVMGetTokenBalanceTool(), handleEVMGetTokenBalance(store, pool))
	s.AddTool(newEVMCheckAllowanceTool(), handleEVMCheckAllowance(store, pool))
	s.AddTool(newSearchTokenTool(), handleSearchToken(cgClient))
	s.AddTool(newBuildSwapTxTool(), handleBuildSwapTx(swapSvc))
	s.AddTool(newConvertAmountTool(), handleConvertAmount())
	s.AddTool(newABIEncodeTool(), handleABIEncode())
	s.AddTool(newABIDecodeTool(), handleABIDecode())
	s.AddTool(newEVMCallTool(), handleEVMCall(pool))
	s.AddTool(newEVMTxInfoTool(), handleEVMTxInfo(store, pool))
	s.AddTool(newBuildEVMTxTool(), handleBuildEVMTx())
	s.AddTool(newBTCFeeRateTool(), handleBTCFeeRate(tcClient))
	s.AddTool(newBuildBTCSendTool(), handleBuildBTCSend(store, bcClient))
	s.AddTool(newLTCFeeRateTool(), handleLTCFeeRate(tcClient))
	s.AddTool(newBuildLTCSendTool(), handleBuildLTCSend(store, bcClient))
	s.AddTool(newDOGEFeeRateTool(), handleDOGEFeeRate(tcClient))
	s.AddTool(newBuildDOGESendTool(), handleBuildDOGESend(store, bcClient))
	s.AddTool(newBCHFeeRateTool(), handleBCHFeeRate(tcClient))
	s.AddTool(newBuildBCHSendTool(), handleBuildBCHSend(store, bcClient))
	s.AddTool(newDASHFeeRateTool(), handleDASHFeeRate(mcClient))
	s.AddTool(newBuildDASHSendTool(), handleBuildDASHSend(store, bcClient))
	s.AddTool(newBuildZECSendTool(), handleBuildZECSend(store, bcClient))
	s.AddTool(newMayaFeeRateTool(), handleMayaFeeRate(mcClient))
	s.AddTool(newGetSOLBalanceTool(), handleGetSOLBalance(store, solClient))
	s.AddTool(newGetSPLTokenBalanceTool(), handleGetSPLTokenBalance(store, solClient))
	s.AddTool(newBuildSolanaTxTool(), handleBuildSolanaTx(store, solClient))
	s.AddTool(newBuildSPLTransferTxTool(), handleBuildSPLTransferTx(store, solClient))
	s.AddTool(newBuildSolanaSwapTool(), handleBuildSolanaSwap(store, jupClient))
	s.AddTool(newGetXRPBalanceTool(), handleGetXRPBalance(store, xrpClient))
	s.AddTool(newBuildXRPSendTool(), handleBuildXRPSend(store, xrpClient))

	// Polymarket prediction market tools
	pmtools.RegisterAll(s, store, pool)

	err := protocols.RegisterAll(s, store, pool)
	if err != nil {
		return fmt.Errorf("register protocols: %w", err)
	}
	return nil
}
