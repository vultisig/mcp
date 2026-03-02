package tools

import (
	"fmt"

	"github.com/mark3labs/mcp-go/server"

	btcsdk "github.com/vultisig/recipes/sdk/btc"
	"github.com/vultisig/recipes/sdk/swap"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/coingecko"
	evmclient "github.com/vultisig/mcp/internal/evm"
	"github.com/vultisig/mcp/internal/jupiter"
	"github.com/vultisig/mcp/internal/protocols"
	solanaclient "github.com/vultisig/mcp/internal/solana"
	"github.com/vultisig/mcp/internal/thorchain"
	"github.com/vultisig/mcp/internal/vault"
	xrpclient "github.com/vultisig/mcp/internal/xrp"
)

func RegisterAll(s *server.MCPServer, store *vault.Store, pool *evmclient.Pool, cgClient *coingecko.Client, bcClient *blockchair.Client, swapSvc *swap.Service, utxoBuilder *btcsdk.Builder, tcClient *thorchain.Client, solClient *solanaclient.Client, jupClient *jupiter.Client, xrpClient *xrpclient.Client) error {
	s.AddTool(newSetVaultInfoTool(), handleSetVaultInfo(store))
	s.AddTool(newGetAddressTool(), handleGetAddress(store))
	s.AddTool(newEVMGetBalanceTool(), handleEVMGetBalance(store, pool))
	s.AddTool(newEVMGetTokenBalanceTool(), handleEVMGetTokenBalance(store, pool))
	s.AddTool(newEVMCheckAllowanceTool(), handleEVMCheckAllowance(store, pool))
	s.AddTool(newSearchTokenTool(), handleSearchToken(cgClient))
	s.AddTool(newGetUTXOBalanceTool(), handleGetUTXOBalance(store, bcClient))
	s.AddTool(newGetUTXOTransactionsTool(), handleGetUTXOTransactions(store, bcClient))
	s.AddTool(newListUTXOsTool(), handleListUTXOs(store, bcClient))
	s.AddTool(newBuildSwapTxTool(), handleBuildSwapTx(swapSvc))
	s.AddTool(newConvertAmountTool(), handleConvertAmount())
	s.AddTool(newABIEncodeTool(), handleABIEncode())
	s.AddTool(newABIDecodeTool(), handleABIDecode())
	s.AddTool(newEVMCallTool(), handleEVMCall(pool))
	s.AddTool(newEVMTxInfoTool(), handleEVMTxInfo(store, pool))
	s.AddTool(newBuildEVMTxTool(), handleBuildEVMTx())
	s.AddTool(newBuildUTXOTxTool(), handleBuildUTXOTx())
	s.AddTool(newBTCFeeRateTool(), handleBTCFeeRate(tcClient))
	s.AddTool(newBuildBTCSendTool(), handleBuildBTCSend(store, utxoBuilder, bcClient))
	s.AddTool(newGetSOLBalanceTool(), handleGetSOLBalance(store, solClient))
	s.AddTool(newGetSPLTokenBalanceTool(), handleGetSPLTokenBalance(store, solClient))
	s.AddTool(newBuildSolanaTxTool(), handleBuildSolanaTx(store, solClient))
	s.AddTool(newBuildSPLTransferTxTool(), handleBuildSPLTransferTx(store, solClient))
	s.AddTool(newBuildSolanaSwapTool(), handleBuildSolanaSwap(store, jupClient))
	s.AddTool(newGetXRPBalanceTool(), handleGetXRPBalance(store, xrpClient))
	s.AddTool(newBuildXRPSendTool(), handleBuildXRPSend(store, xrpClient))

	err := protocols.RegisterAll(s, store, pool)
	if err != nil {
		return fmt.Errorf("register protocols: %w", err)
	}
	return nil
}
