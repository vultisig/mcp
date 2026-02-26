package tools

import (
	"math/big"

	"github.com/mark3labs/mcp-go/server"

	btcsdk "github.com/vultisig/recipes/sdk/btc"
	"github.com/vultisig/recipes/sdk/evm"
	"github.com/vultisig/recipes/sdk/swap"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/coingecko"
	"github.com/vultisig/mcp/internal/ethereum"
	"github.com/vultisig/mcp/internal/protocols"
	"github.com/vultisig/mcp/internal/thorchain"
	"github.com/vultisig/mcp/internal/vault"
)

func RegisterAll(s *server.MCPServer, store *vault.Store, ethClient *ethereum.Client, evmSDK *evm.SDK, chainID *big.Int, cgClient *coingecko.Client, bcClient *blockchair.Client, swapSvc *swap.Service, utxoBuilder *btcsdk.Builder, tcClient *thorchain.Client) {
	s.AddTool(newSetVaultInfoTool(), handleSetVaultInfo(store))
	s.AddTool(newGetAddressTool(), handleGetAddress(store))
	s.AddTool(newGetETHBalanceTool(), handleGetETHBalance(store, ethClient))
	s.AddTool(newGetTokenBalanceTool(), handleGetTokenBalance(store, ethClient))
	s.AddTool(newSearchTokenTool(), handleSearchToken(cgClient))
	s.AddTool(newGetUTXOBalanceTool(), handleGetUTXOBalance(store, bcClient))
	s.AddTool(newGetUTXOTransactionsTool(), handleGetUTXOTransactions(store, bcClient))
	s.AddTool(newListUTXOsTool(), handleListUTXOs(store, bcClient))
	s.AddTool(newBuildSwapTxTool(), handleBuildSwapTx(swapSvc))
	s.AddTool(newConvertAmountTool(), handleConvertAmount())
	s.AddTool(newABIEncodeTool(), handleABIEncode())
	s.AddTool(newABIDecodeTool(), handleABIDecode())
	s.AddTool(newEVMCallTool(), handleEVMCall(ethClient))
	s.AddTool(newEVMTxInfoTool(), handleEVMTxInfo(store, ethClient, chainID))
	s.AddTool(newBuildEVMTxTool(), handleBuildEVMTx(chainID))
	s.AddTool(newBuildUTXOTxTool(), handleBuildUTXOTx())
	s.AddTool(newBTCFeeRateTool(), handleBTCFeeRate(tcClient))
	s.AddTool(newBuildBTCSendTool(), handleBuildBTCSend(store, utxoBuilder, bcClient))

	protocols.RegisterAll(s, store, ethClient, evmSDK, chainID)
}
