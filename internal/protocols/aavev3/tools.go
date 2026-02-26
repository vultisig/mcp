package aavev3

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	reth "github.com/vultisig/recipes/chain/evm/ethereum"
	evmsdk "github.com/vultisig/recipes/sdk/evm"
	aavev3sdk "github.com/vultisig/recipes/sdk/evm/aavev3"

	evmclient "github.com/vultisig/mcp/internal/evm"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/types"
	"github.com/vultisig/mcp/internal/vault"
)

const (
	gasLimitSupply = 300_000
	gasLimitRepay  = 300_000
)

type Protocol struct{}

func (p *Protocol) Name() string { return "aave-v3" }

func (p *Protocol) SupportsChain(chainID *big.Int) bool {
	_, ok := aavev3sdk.GetDeployment(chainID)
	return ok
}

func (p *Protocol) Register(s *server.MCPServer, store *vault.Store, ethClient *evmclient.Client, evmSDK *evmsdk.SDK, chainID *big.Int) {
	deploy, _ := aavev3sdk.GetDeployment(chainID)
	aaveClient := aavev3sdk.NewClient(ethClient, deploy)

	s.AddTool(newDepositTool(), handleDeposit(store, evmSDK, aaveClient, chainID))
	s.AddTool(newWithdrawTool(), handleWithdraw(store, evmSDK, aaveClient, chainID))
	s.AddTool(newBorrowTool(), handleBorrow(store, evmSDK, aaveClient, chainID))
	s.AddTool(newRepayTool(), handleRepay(store, evmSDK, aaveClient, chainID))
	s.AddTool(newGetBalancesTool(), handleGetBalances(store, aaveClient))
	s.AddTool(newGetRatesTool(), handleGetRates(aaveClient))
}

func newDepositTool() mcp.Tool {
	return mcp.NewTool("aave_v3_deposit",
		mcp.WithDescription("Build unsigned transactions to deposit (supply) tokens into Aave V3. Returns an approve tx and a supply tx, both fully populated and ready to sign."),
		mcp.WithString("asset", mcp.Description("ERC-20 token contract address (0x-prefixed)"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount to deposit in human-readable units (e.g. \"100.5\") or \"max\" for full balance"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Depositor's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newWithdrawTool() mcp.Tool {
	return mcp.NewTool("aave_v3_withdraw",
		mcp.WithDescription("Build an unsigned transaction to withdraw tokens from Aave V3. Returns a fully populated transaction ready to sign. Use amount \"max\" for full withdrawal."),
		mcp.WithString("asset", mcp.Description("ERC-20 token contract address (0x-prefixed)"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount to withdraw in human-readable units or \"max\""), mcp.Required()),
		mcp.WithString("address", mcp.Description("Withdrawer's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newBorrowTool() mcp.Tool {
	return mcp.NewTool("aave_v3_borrow",
		mcp.WithDescription("Build an unsigned transaction to borrow tokens from Aave V3 at variable rate. Returns a fully populated transaction ready to sign."),
		mcp.WithString("asset", mcp.Description("ERC-20 token contract address (0x-prefixed)"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount to borrow in human-readable units"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Borrower's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newRepayTool() mcp.Tool {
	return mcp.NewTool("aave_v3_repay",
		mcp.WithDescription("Build unsigned transactions to repay a borrow on Aave V3. Returns an approve tx and a repay tx, both fully populated and ready to sign. Use amount \"max\" to repay entire debt."),
		mcp.WithString("asset", mcp.Description("ERC-20 token contract address (0x-prefixed)"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount to repay in human-readable units or \"max\""), mcp.Required()),
		mcp.WithString("address", mcp.Description("Repayer's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newGetBalancesTool() mcp.Tool {
	return mcp.NewTool("aave_v3_get_balances",
		mcp.WithDescription("Query Aave V3 account summary: total collateral, total debt, available borrows (all in USD), liquidation threshold, LTV, and health factor."),
		mcp.WithString("address", mcp.Description("Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newGetRatesTool() mcp.Tool {
	return mcp.NewTool("aave_v3_get_rates",
		mcp.WithDescription("Query Aave V3 supply APY, variable borrow APY, and reserve configuration for a given token."),
		mcp.WithString("asset", mcp.Description("ERC-20 token contract address (0x-prefixed)"), mcp.Required()),
	)
}

func stripHexPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}

func evmTxDetails(to string, nonce uint64, gasLimit uint64, maxFeePerGas string, maxPriorityFeePerGas string, data string, description string) map[string]string {
	return map[string]string{
		"to":                       to,
		"value":                    "0",
		"nonce":                    fmt.Sprintf("%d", nonce),
		"gas_limit":                fmt.Sprintf("%d", gasLimit),
		"max_fee_per_gas":          maxFeePerGas,
		"max_priority_fee_per_gas": maxPriorityFeePerGas,
		"data":                     data,
		"tx_encoding":              types.TxEncodingEIP1559RLP,
		"description":              description,
	}
}

func addTokenFields(m map[string]string, symbol, tokenAddr, amountHuman, amountWei, decimals string) {
	m["token_symbol"] = symbol
	m["token_address"] = tokenAddr
	m["amount_human"] = amountHuman
	m["amount_wei"] = amountWei
	m["decimals"] = decimals
}

type txBuildAction struct {
	action       string
	description  string
	contractName string
}

func handleDeposit(store *vault.Store, evmSDK *evmsdk.SDK, aaveClient *aavev3sdk.Client, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		assetStr, amountStr, addr, err := extractTxParams(ctx, req, store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		asset := ethcommon.HexToAddress(assetStr)
		user := ethcommon.HexToAddress(addr)

		txs, err := aavev3sdk.BuildDepositTx(ctx, aaveClient, asset, amountStr, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build deposit: %v", err)), nil
		}

		symbol, err := aaveClient.GetTokenSymbol(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get token symbol: %v", err)), nil
		}
		decimals, err := aaveClient.GetTokenDecimals(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get token decimals: %v", err)), nil
		}

		amount, err := aavev3sdk.ParseAmount(amountStr, int(decimals))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		actions := []txBuildAction{
			{"approve", fmt.Sprintf("Approve %s spending for Aave V3 Pool", symbol), fmt.Sprintf("%s Token", symbol)},
			{"supply", fmt.Sprintf("Supply %s to Aave V3", symbol), "Aave V3 Pool"},
		}
		gasOverrides := []uint64{0, gasLimitSupply}

		return buildTxResult(ctx, evmSDK, user, chainID, txs, actions, gasOverrides, symbol, asset.Hex(), amountStr, amount.String(), fmt.Sprintf("%d", decimals))
	}
}

func handleWithdraw(store *vault.Store, evmSDK *evmsdk.SDK, aaveClient *aavev3sdk.Client, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		assetStr, amountStr, addr, err := extractTxParams(ctx, req, store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		asset := ethcommon.HexToAddress(assetStr)
		user := ethcommon.HexToAddress(addr)

		txs, err := aavev3sdk.BuildWithdrawTx(ctx, aaveClient, asset, amountStr, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build withdraw: %v", err)), nil
		}

		symbol, err := aaveClient.GetTokenSymbol(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get token symbol: %v", err)), nil
		}
		decimals, err := aaveClient.GetTokenDecimals(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get token decimals: %v", err)), nil
		}

		amount, err := aavev3sdk.ParseAmount(amountStr, int(decimals))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		actions := []txBuildAction{
			{"withdraw", fmt.Sprintf("Withdraw %s from Aave V3", symbol), "Aave V3 Pool"},
		}

		return buildTxResult(ctx, evmSDK, user, chainID, txs, actions, []uint64{0}, symbol, asset.Hex(), amountStr, amount.String(), fmt.Sprintf("%d", decimals))
	}
}

func handleBorrow(store *vault.Store, evmSDK *evmsdk.SDK, aaveClient *aavev3sdk.Client, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		assetStr, amountStr, addr, err := extractTxParams(ctx, req, store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		asset := ethcommon.HexToAddress(assetStr)
		user := ethcommon.HexToAddress(addr)

		txs, err := aavev3sdk.BuildBorrowTx(ctx, aaveClient, asset, amountStr, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build borrow: %v", err)), nil
		}

		symbol, err := aaveClient.GetTokenSymbol(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get token symbol: %v", err)), nil
		}
		decimals, err := aaveClient.GetTokenDecimals(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get token decimals: %v", err)), nil
		}

		amount, err := aavev3sdk.ParseAmount(amountStr, int(decimals))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		actions := []txBuildAction{
			{"borrow", fmt.Sprintf("Borrow %s from Aave V3 (variable rate)", symbol), "Aave V3 Pool"},
		}

		return buildTxResult(ctx, evmSDK, user, chainID, txs, actions, []uint64{0}, symbol, asset.Hex(), amountStr, amount.String(), fmt.Sprintf("%d", decimals))
	}
}

func handleRepay(store *vault.Store, evmSDK *evmsdk.SDK, aaveClient *aavev3sdk.Client, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		assetStr, amountStr, addr, err := extractTxParams(ctx, req, store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		asset := ethcommon.HexToAddress(assetStr)
		user := ethcommon.HexToAddress(addr)

		txs, err := aavev3sdk.BuildRepayTx(ctx, aaveClient, asset, amountStr, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build repay: %v", err)), nil
		}

		symbol, err := aaveClient.GetTokenSymbol(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get token symbol: %v", err)), nil
		}
		decimals, err := aaveClient.GetTokenDecimals(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get token decimals: %v", err)), nil
		}

		amount, err := aavev3sdk.ParseAmount(amountStr, int(decimals))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		actions := []txBuildAction{
			{"approve", fmt.Sprintf("Approve %s spending for Aave V3 Pool", symbol), fmt.Sprintf("%s Token", symbol)},
			{"repay", fmt.Sprintf("Repay %s to Aave V3 (variable rate)", symbol), "Aave V3 Pool"},
		}
		gasOverrides := []uint64{0, gasLimitRepay}

		return buildTxResult(ctx, evmSDK, user, chainID, txs, actions, gasOverrides, symbol, asset.Hex(), amountStr, amount.String(), fmt.Sprintf("%d", decimals))
	}
}

func handleGetBalances(store *vault.Store, aaveClient *aavev3sdk.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		user := ethcommon.HexToAddress(addr)
		acct, err := aaveClient.GetUserAccountData(ctx, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get account data: %v", err)), nil
		}

		collateral := evmclient.FormatUnits(acct.TotalCollateralBase, 8)
		debt := evmclient.FormatUnits(acct.TotalDebtBase, 8)
		available := evmclient.FormatUnits(acct.AvailableBorrowsBase, 8)

		liqThresholdPct := fmt.Sprintf("%.2f%%", float64(acct.CurrentLiquidationThreshold.Int64())/100.0)
		ltvPct := fmt.Sprintf("%.2f%%", float64(acct.LTV.Int64())/100.0)

		healthFactor := evmclient.FormatUnits(acct.HealthFactor, 18)
		if acct.TotalDebtBase.Sign() == 0 {
			healthFactor = "∞ (no debt)"
		}

		result := fmt.Sprintf(`Aave V3 Account Summary
Address: %s

Total Collateral: $%s USD
Total Debt: $%s USD
Available to Borrow: $%s USD
Liquidation Threshold: %s
Loan-to-Value (LTV): %s
Health Factor: %s`,
			addr,
			collateral, debt, available,
			liqThresholdPct, ltvPct, healthFactor,
		)

		return mcp.NewToolResultText(result), nil
	}
}

func handleGetRates(aaveClient *aavev3sdk.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		assetStr, err := req.RequireString("asset")
		if err != nil {
			return mcp.NewToolResultError("missing asset"), nil
		}

		asset := ethcommon.HexToAddress(assetStr)

		symbol, err := aaveClient.GetTokenSymbol(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token symbol: %v", err)), nil
		}

		reserveData, err := aaveClient.GetReserveData(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get reserve data: %v", err)), nil
		}

		configData, err := aaveClient.GetReserveConfigData(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get reserve config: %v", err)), nil
		}

		supplyAPY := aavev3sdk.RayToAPY(reserveData.LiquidityRate)
		borrowAPY := aavev3sdk.RayToAPY(reserveData.VariableBorrowRate)

		ltvPct := fmt.Sprintf("%.2f%%", float64(configData.LTV.Int64())/100.0)
		liqThresholdPct := fmt.Sprintf("%.2f%%", float64(configData.LiquidationThreshold.Int64())/100.0)
		liqBonusPct := fmt.Sprintf("%.2f%%", float64(configData.LiquidationBonus.Int64()-10000)/100.0)

		result := fmt.Sprintf(`Aave V3 Reserve Info — %s (%s)

Supply APY: %.2f%%
Variable Borrow APY: %.2f%%

Reserve Configuration:
  Decimals: %s
  LTV: %s
  Liquidation Threshold: %s
  Liquidation Bonus: %s
  Can be Collateral: %v
  Borrowing Enabled: %v
  Active: %v
  Frozen: %v`,
			symbol, assetStr,
			supplyAPY, borrowAPY,
			configData.Decimals.String(),
			ltvPct, liqThresholdPct, liqBonusPct,
			configData.UsageAsCollateralEnabled,
			configData.BorrowingEnabled,
			configData.IsActive,
			configData.IsFrozen,
		)

		return mcp.NewToolResultText(result), nil
	}
}

func extractTxParams(ctx context.Context, req mcp.CallToolRequest, store *vault.Store) (asset, amount, addr string, err error) {
	asset, err = req.RequireString("asset")
	if err != nil {
		return "", "", "", fmt.Errorf("missing asset")
	}
	amount, err = req.RequireString("amount")
	if err != nil {
		return "", "", "", fmt.Errorf("missing amount")
	}

	explicit := req.GetString("address", "")
	addr, err = resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
	if err != nil {
		return "", "", "", err
	}

	return asset, amount, addr, nil
}

func decodeTxFields(unsignedTx evmsdk.UnsignedTx) (nonce uint64, gasLimit uint64, maxFeePerGas string, maxPriorityFeePerGas string, err error) {
	txData, err := reth.DecodeUnsignedPayload(unsignedTx)
	if err != nil {
		return 0, 0, "", "", fmt.Errorf("decode unsigned tx: %w", err)
	}
	tx := ethtypes.NewTx(txData)
	return tx.Nonce(), tx.Gas(), tx.GasFeeCap().String(), tx.GasTipCap().String(), nil
}

func buildTxResult(
	ctx context.Context,
	evmSDK *evmsdk.SDK,
	user ethcommon.Address,
	chainID *big.Int,
	txs []aavev3sdk.TxData,
	actions []txBuildAction,
	gasOverrides []uint64,
	symbol, assetAddr, amountHuman, amountWei, decimals string,
) (*mcp.CallToolResult, error) {
	chainName := types.EVMChainName(chainID)
	chainIDStr := chainID.String()

	var resultTxs []types.Transaction
	for i, tx := range txs {
		gasOverride := uint64(0)
		if i < len(gasOverrides) {
			gasOverride = gasOverrides[i]
		}

		var unsignedTx evmsdk.UnsignedTx
		var buildErr error
		if gasOverride > 0 {
			unsignedTx, buildErr = evmSDK.MakeTxWithGasLimit(ctx, user, tx.To, tx.Value, tx.Data, uint64(i), gasOverride)
		} else {
			unsignedTx, buildErr = evmSDK.MakeTx(ctx, user, tx.To, tx.Value, tx.Data, uint64(i))
		}
		if buildErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build tx %d: %v", i+1, buildErr)), nil
		}

		nonce, gasLimit, maxFee, maxPriorityFee, decodeErr := decodeTxFields(unsignedTx)
		if decodeErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to decode tx %d: %v", i+1, decodeErr)), nil
		}

		action := actions[i]
		dets := evmTxDetails(tx.To.Hex(), nonce, gasLimit, maxFee, maxPriorityFee, fmt.Sprintf("0x%x", tx.Data), action.description)
		dets["contract_name"] = action.contractName
		addTokenFields(dets, symbol, assetAddr, amountHuman, amountWei, decimals)

		resultTxs = append(resultTxs, types.Transaction{
			Sequence:      i + 1,
			Chain:         chainName,
			ChainID:       chainIDStr,
			Action:        action.action,
			SigningMode:   types.SigningModeECDSA,
			UnsignedTxHex: hex.EncodeToString(unsignedTx),
			TxDetails:     dets,
		})
	}

	result := &types.TransactionResult{Transactions: resultTxs}
	return result.ToToolResult()
}
