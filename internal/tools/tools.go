package tools

import (
	"math/big"

	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/recipes/sdk/evm"
	"github.com/vultisig/recipes/sdk/swap"

	"github.com/vultisig/mcp/internal/blockchair"
	"github.com/vultisig/mcp/internal/coingecko"
	"github.com/vultisig/mcp/internal/ethereum"
	"github.com/vultisig/mcp/internal/protocols"
	"github.com/vultisig/mcp/internal/vault"
)

func RegisterAll(s *server.MCPServer, store *vault.Store, ethClient *ethereum.Client, evmSDK *evm.SDK, chainID *big.Int, cgClient *coingecko.Client, bcClient *blockchair.Client, swapSvc *swap.Service) {
	s.AddTool(newSetVaultInfoTool(), handleSetVaultInfo(store))
	s.AddTool(newGetAddressTool(), handleGetAddress(store))
	s.AddTool(newGetETHBalanceTool(), handleGetETHBalance(store, ethClient))
	s.AddTool(newGetTokenBalanceTool(), handleGetTokenBalance(store, ethClient))
	s.AddTool(newFindTokenTool(), handleFindToken(cgClient))
	s.AddTool(newGetUTXOBalanceTool(), handleGetUTXOBalance(store, bcClient))
	s.AddTool(newGetUTXOTransactionsTool(), handleGetUTXOTransactions(store, bcClient))
	s.AddTool(newListUTXOsTool(), handleListUTXOs(store, bcClient))
	s.AddTool(newBuildSwapTxTool(), handleBuildSwapTx(swapSvc))
	s.AddTool(newConvertAmountTool(), handleConvertAmount())
	s.AddTool(newABIEncodeTool(), handleABIEncode())
	s.AddTool(newABIDecodeTool(), handleABIDecode())

	protocols.RegisterAll(s, store, ethClient, evmSDK, chainID)
}
