package eigenlayer

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	reth "github.com/vultisig/recipes/chain/evm/ethereum"
	evmsdk "github.com/vultisig/recipes/sdk/evm"

	evmclient "github.com/vultisig/mcp/internal/evm"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/toolmeta"
	"github.com/vultisig/mcp/internal/types"
	"github.com/vultisig/mcp/internal/vault"
)

const (
	gasLimitDeposit = 300_000
	gasLimitApprove = 60_000
)

// Protocol implements the protocols.Protocol interface for EigenLayer.
type Protocol struct{}

func (p *Protocol) Name() string { return "eigenlayer" }

func (p *Protocol) SupportsChain(chainID *big.Int) bool {
	return chainID.Int64() == 1 // Ethereum mainnet only
}

func (p *Protocol) Register(s *server.MCPServer, store *vault.Store, ethClient *evmclient.Client, evmSDK *evmsdk.SDK, chainID *big.Int) {
	toolmeta.Register(s, newDepositTool(), handleDeposit(store, evmSDK, chainID), "eigenlayer")
	toolmeta.Register(s, newGetPositionTool(), handleGetPosition(store, ethClient), "eigenlayer")
	toolmeta.Register(s, newGetDelegationTool(), handleGetDelegation(store, ethClient), "eigenlayer")
	toolmeta.Register(s, newGetStrategiesTool(), handleGetStrategies(ethClient), "eigenlayer")
}

// --- Tool definitions ---

func newDepositTool() mcp.Tool {
	return mcp.NewTool("eigen_deposit",
		mcp.WithDescription("Build unsigned transactions to deposit an LST (e.g. stETH, cbETH, rETH) into an EigenLayer strategy for restaking. Returns an approve tx and a deposit tx."),
		mcp.WithString("strategy", mcp.Description("Strategy name: \"stETH\", \"cbETH\", or \"rETH\""), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount of token to deposit in human-readable units"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Depositor's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newGetPositionTool() mcp.Tool {
	return mcp.NewTool("eigen_get_position",
		mcp.WithDescription("Query a user's EigenLayer restaking positions across all known strategies (stETH, cbETH, rETH). Shows shares and underlying token value."),
		mcp.WithString("address", mcp.Description("Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newGetDelegationTool() mcp.Tool {
	return mcp.NewTool("eigen_get_delegation",
		mcp.WithDescription("Query which EigenLayer operator a staker is delegated to."),
		mcp.WithString("address", mcp.Description("Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newGetStrategiesTool() mcp.Tool {
	return mcp.NewTool("eigen_get_strategies",
		mcp.WithDescription("List available EigenLayer strategies with their token symbols and strategy contract addresses."),
	)
}

// --- Handlers ---

func handleDeposit(store *vault.Store, evmSDK *evmsdk.SDK, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		strategyName, err := req.RequireString("strategy")
		if err != nil {
			return mcp.NewToolResultError("missing strategy"), nil
		}

		si, ok := Strategies[strategyName]
		if !ok {
			available := make([]string, 0, len(Strategies))
			for k := range Strategies {
				available = append(available, k)
			}
			return mcp.NewToolResultError(fmt.Sprintf("unknown strategy %q: available strategies are %s", strategyName, strings.Join(available, ", "))), nil
		}

		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount"), nil
		}

		weiAmount, err := ParseAmount(amountStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		explicit := req.GetString("address", "")
		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		user := ethcommon.HexToAddress(addr)

		// 1. Approve token spending by StrategyManager
		approveData, err := erc20ABI.Pack("approve", StrategyManagerAddress, weiAmount)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode approve: %v", err)), nil
		}

		approveTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, si.Token, big.NewInt(0), approveData, 0, gasLimitApprove)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build approve tx: %v", err)), nil
		}

		// 2. Deposit into strategy
		depositData, err := strategyManagerABI.Pack("depositIntoStrategy", si.Strategy, si.Token, weiAmount)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode deposit: %v", err)), nil
		}

		depositTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, StrategyManagerAddress, big.NewInt(0), depositData, 1, gasLimitDeposit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build deposit tx: %v", err)), nil
		}

		return buildMultiTxResult(chainID, []unsignedTxInfo{
			{approveTx, approveData, si.Token, "approve", fmt.Sprintf("Approve %s spending for EigenLayer StrategyManager", si.Symbol), fmt.Sprintf("%s Token", si.Symbol), big.NewInt(0)},
			{depositTx, depositData, StrategyManagerAddress, "deposit", fmt.Sprintf("Deposit %s into EigenLayer strategy", si.Symbol), "EigenLayer StrategyManager", big.NewInt(0)},
		}, si.Symbol, si.Token.Hex(), amountStr, weiAmount.String(), fmt.Sprintf("%d", si.Decimals))
	}
}

func handleGetPosition(store *vault.Store, ethClient *evmclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		user := ethcommon.HexToAddress(addr)
		eth := ethClient.ETH()

		// Build strategy address array for batch query
		strategyNames := []string{"stETH", "cbETH", "rETH"}
		strategyAddrs := make([]ethcommon.Address, len(strategyNames))
		for i, name := range strategyNames {
			strategyAddrs[i] = Strategies[name].Strategy
		}

		// Query all shares in one call via DelegationManager.getWithdrawableShares
		sharesData, err := delegationManagerABI.Pack("getWithdrawableShares", user, strategyAddrs)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode getWithdrawableShares: %v", err)), nil
		}
		sharesResult, err := eth.CallContract(ctx, callMsg(DelegationManagerAddress, sharesData), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query shares: %v", err)), nil
		}

		sharesValues, err := delegationManagerABI.Unpack("getWithdrawableShares", sharesResult)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode shares: %v", err)), nil
		}

		withdrawableShares := sharesValues[0].([]*big.Int)

		var lines string
		lines = fmt.Sprintf("EigenLayer Restaking Positions\nAddress: %s\n", addr)

		for i, name := range strategyNames {
			si := Strategies[name]
			shares := withdrawableShares[i]

			lines += fmt.Sprintf("\n%s Strategy (%s):\n  Withdrawable Shares: %s %s",
				si.Symbol,
				si.Strategy.Hex()[:10]+"...",
				evmclient.FormatUnits(shares, si.Decimals),
				si.Symbol,
			)
		}

		return mcp.NewToolResultText(lines), nil
	}
}

func handleGetDelegation(store *vault.Store, ethClient *evmclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		user := ethcommon.HexToAddress(addr)
		eth := ethClient.ETH()

		// Query delegatedTo
		delData, err := delegationManagerABI.Pack("delegatedTo", user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode delegatedTo: %v", err)), nil
		}
		delResult, err := eth.CallContract(ctx, callMsg(DelegationManagerAddress, delData), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query delegatedTo: %v", err)), nil
		}

		values, err := delegationManagerABI.Unpack("delegatedTo", delResult)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode delegatedTo: %v", err)), nil
		}
		operator := values[0].(ethcommon.Address)

		// Query isDelegated
		isDelegatedData, err := delegationManagerABI.Pack("isDelegated", user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode isDelegated: %v", err)), nil
		}
		isDelegatedResult, err := eth.CallContract(ctx, callMsg(DelegationManagerAddress, isDelegatedData), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query isDelegated: %v", err)), nil
		}

		isDelegatedValues, err := delegationManagerABI.Unpack("isDelegated", isDelegatedResult)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode isDelegated: %v", err)), nil
		}
		delegated := isDelegatedValues[0].(bool)

		status := "Not delegated"
		if delegated {
			status = fmt.Sprintf("Delegated to operator: %s", operator.Hex())
		}

		text := fmt.Sprintf(`EigenLayer Delegation Status
Address: %s

%s`,
			addr,
			status,
		)

		return mcp.NewToolResultText(text), nil
	}
}

func handleGetStrategies(ethClient *evmclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var lines string
		lines = "EigenLayer Available Strategies\n"

		for _, name := range []string{"stETH", "cbETH", "rETH"} {
			si := Strategies[name]
			lines += fmt.Sprintf("\n%s:\n  Strategy: %s\n  Token: %s",
				si.Symbol,
				si.Strategy.Hex(),
				si.Token.Hex(),
			)
		}

		return mcp.NewToolResultText(lines), nil
	}
}

// --- Helpers ---

type unsignedTxInfo struct {
	raw          evmsdk.UnsignedTx
	calldata     []byte
	to           ethcommon.Address
	action       string
	description  string
	contractName string
	value        *big.Int
}

func buildMultiTxResult(chainID *big.Int, txInfos []unsignedTxInfo, symbol, tokenAddr, amountHuman, amountWei, decimals string) (*mcp.CallToolResult, error) {
	chainName := types.EVMChainName(chainID)
	chainIDStr := chainID.String()

	var resultTxs []types.Transaction
	for i, info := range txInfos {
		nonce, gasLimit, maxFee, maxPriorityFee, err := decodeTxFields(info.raw)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode tx %d: %v", i+1, err)), nil
		}

		valueStr := "0"
		if info.value != nil && info.value.Sign() > 0 {
			valueStr = info.value.String()
		}

		dets := txDetails(info.to.Hex(), valueStr, nonce, gasLimit, maxFee, maxPriorityFee, info.calldata, info.description, info.contractName)
		dets["token_symbol"] = symbol
		dets["token_address"] = tokenAddr
		dets["amount_human"] = amountHuman
		dets["amount_wei"] = amountWei
		dets["decimals"] = decimals

		resultTxs = append(resultTxs, types.Transaction{
			Sequence:      i + 1,
			Chain:         chainName,
			ChainID:       chainIDStr,
			Action:        info.action,
			SigningMode:   types.SigningModeECDSA,
			UnsignedTxHex: hex.EncodeToString(info.raw),
			TxDetails:     dets,
		})
	}

	result := &types.TransactionResult{Transactions: resultTxs}
	return result.ToToolResult()
}

func decodeTxFields(unsignedTx evmsdk.UnsignedTx) (nonce uint64, gasLimit uint64, maxFeePerGas string, maxPriorityFeePerGas string, err error) {
	txData, err := reth.DecodeUnsignedPayload(unsignedTx)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("decode unsigned tx: %w", err)
	}
	tx := ethtypes.NewTx(txData)
	return tx.Nonce(), tx.Gas(), tx.GasFeeCap().String(), tx.GasTipCap().String(), nil
}

func txDetails(to, value string, nonce, gasLimit uint64, maxFeePerGas, maxPriorityFeePerGas string, data []byte, description, contractName string) map[string]string {
	return map[string]string{
		"to":                       to,
		"value":                    value,
		"nonce":                    fmt.Sprintf("%d", nonce),
		"gas_limit":                fmt.Sprintf("%d", gasLimit),
		"max_fee_per_gas":          maxFeePerGas,
		"max_priority_fee_per_gas": maxPriorityFeePerGas,
		"data":                     fmt.Sprintf("0x%x", data),
		"tx_encoding":              types.TxEncodingEIP1559RLP,
		"description":              description,
		"contract_name":            contractName,
	}
}

func callMsg(to ethcommon.Address, data []byte) ethereum.CallMsg {
	return ethereum.CallMsg{To: &to, Data: data}
}
