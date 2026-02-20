package aavev3

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/ethereum"
	"github.com/vultisig/mcp/internal/types"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

// Default gas limits for transactions that follow an approve and therefore
// cannot be estimated against current on-chain state.
const (
	gasLimitSupply = 300_000
	gasLimitRepay  = 300_000
)

// Protocol implements the types.Protocol interface for Aave V3.
type Protocol struct{}

func (p *Protocol) Name() string { return "aave-v3" }

func (p *Protocol) SupportsChain(chainID *big.Int) bool {
	_, ok := GetDeployment(chainID)
	return ok
}

func (p *Protocol) Register(s *server.MCPServer, store *vault.Store, ethClient *ethereum.Client, chainID *big.Int) {
	deploy, _ := GetDeployment(chainID)
	pc := NewProtocolClient(ethClient, deploy)

	s.AddTool(newDepositTool(), handleDeposit(store, ethClient, pc, chainID))
	s.AddTool(newWithdrawTool(), handleWithdraw(store, ethClient, pc, chainID))
	s.AddTool(newBorrowTool(), handleBorrow(store, ethClient, pc, chainID))
	s.AddTool(newRepayTool(), handleRepay(store, ethClient, pc, chainID))
	s.AddTool(newGetBalancesTool(), handleGetBalances(store, pc))
	s.AddTool(newGetRatesTool(), handleGetRates(pc))
}

// --- Tool definitions ---

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

// --- helpers ---

// stripHexPrefix removes a leading "0x" or "0X" from a hex string.
func stripHexPrefix(s string) string {
	return strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
}

// evmTxDetails builds the tx_details map for an EVM transaction.
func evmTxDetails(to string, nonce uint64, gasLimit uint64, gasPrice string, data string, description string) map[string]string {
	return map[string]string{
		"to":          to,
		"value":       "0",
		"nonce":       fmt.Sprintf("%d", nonce),
		"gas_limit":   fmt.Sprintf("%d", gasLimit),
		"gas_price":   gasPrice,
		"data":        data,
		"tx_encoding": types.TxEncodingLegacyRLP,
		"description": description,
	}
}

// addTokenFields adds the common token metadata fields to a tx_details map.
func addTokenFields(m map[string]string, symbol, tokenAddr, amountHuman, amountWei, decimals string) {
	m["token_symbol"] = symbol
	m["token_address"] = tokenAddr
	m["amount_human"] = amountHuman
	m["amount_wei"] = amountWei
	m["decimals"] = decimals
}

// --- Tool handlers ---

func handleDeposit(store *vault.Store, ethClient *ethereum.Client, pc *ProtocolClient, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		assetStr, err := req.RequireString("asset")
		if err != nil {
			return mcp.NewToolResultError("missing asset"), nil
		}
		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount"), nil
		}
		explicit := req.GetString("address", "")

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		asset := ethcommon.HexToAddress(assetStr)
		user := ethcommon.HexToAddress(addr)
		pool := pc.PoolAddress()

		decimals, err := pc.GetTokenDecimals(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token decimals: %v", err)), nil
		}
		symbol, err := pc.GetTokenSymbol(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token symbol: %v", err)), nil
		}

		amount, err := ParseAmount(amountStr, int(decimals))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		nonce, err := ethClient.PendingNonceAt(ctx, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get nonce: %v", err)), nil
		}

		chainName := types.EVMChainName(chainID)
		chainIDStr := chainID.String()
		assetChecksummed := asset.Hex()
		poolChecksummed := pool.Hex()
		decimalsStr := fmt.Sprintf("%d", decimals)

		// Tx 1: approve
		approveData := EncodeApprove(pool, amount)
		approveHex, approveTx, err := ethClient.BuildUnsignedTx(ctx, user, asset, approveData, big.NewInt(0), chainID, nonce, 0)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build approve tx: %v", err)), nil
		}
		approveDets := evmTxDetails(assetChecksummed, nonce, approveTx.Gas(), approveTx.GasPrice().String(), fmt.Sprintf("0x%x", approveData), fmt.Sprintf("Approve %s spending for Aave V3 Pool", symbol))
		approveDets["contract_name"] = fmt.Sprintf("%s Token", symbol)
		addTokenFields(approveDets, symbol, assetChecksummed, amountStr, amount.String(), decimalsStr)

		// Tx 2: supply (depends on approve — fixed gas)
		supplyData := EncodeSupply(asset, amount, user)
		supplyHex, supplyTx, err := ethClient.BuildUnsignedTx(ctx, user, pool, supplyData, big.NewInt(0), chainID, nonce+1, gasLimitSupply)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build supply tx: %v", err)), nil
		}
		supplyDets := evmTxDetails(poolChecksummed, nonce+1, supplyTx.Gas(), supplyTx.GasPrice().String(), fmt.Sprintf("0x%x", supplyData), fmt.Sprintf("Supply %s to Aave V3", symbol))
		supplyDets["contract_name"] = "Aave V3 Pool"
		addTokenFields(supplyDets, symbol, assetChecksummed, amountStr, amount.String(), decimalsStr)

		result := &types.TransactionResult{
			Transactions: []types.Transaction{
				{Sequence: 1, Chain: chainName, ChainID: chainIDStr, Action: "approve", SigningMode: types.SigningModeECDSA, UnsignedTxHex: stripHexPrefix(approveHex), TxDetails: approveDets},
				{Sequence: 2, Chain: chainName, ChainID: chainIDStr, Action: "supply", SigningMode: types.SigningModeECDSA, UnsignedTxHex: stripHexPrefix(supplyHex), TxDetails: supplyDets},
			},
		}
		return result.ToToolResult()
	}
}

func handleWithdraw(store *vault.Store, ethClient *ethereum.Client, pc *ProtocolClient, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		assetStr, err := req.RequireString("asset")
		if err != nil {
			return mcp.NewToolResultError("missing asset"), nil
		}
		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount"), nil
		}
		explicit := req.GetString("address", "")

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		asset := ethcommon.HexToAddress(assetStr)
		user := ethcommon.HexToAddress(addr)
		pool := pc.PoolAddress()

		decimals, err := pc.GetTokenDecimals(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token decimals: %v", err)), nil
		}
		symbol, err := pc.GetTokenSymbol(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token symbol: %v", err)), nil
		}

		amount, err := ParseAmount(amountStr, int(decimals))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		nonce, err := ethClient.PendingNonceAt(ctx, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get nonce: %v", err)), nil
		}

		withdrawData := EncodeWithdraw(asset, amount, user)
		withdrawHex, withdrawTx, err := ethClient.BuildUnsignedTx(ctx, user, pool, withdrawData, big.NewInt(0), chainID, nonce, 0)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build withdraw tx: %v", err)), nil
		}

		dets := evmTxDetails(pool.Hex(), nonce, withdrawTx.Gas(), withdrawTx.GasPrice().String(), fmt.Sprintf("0x%x", withdrawData), fmt.Sprintf("Withdraw %s from Aave V3", symbol))
		dets["contract_name"] = "Aave V3 Pool"
		addTokenFields(dets, symbol, asset.Hex(), amountStr, amount.String(), fmt.Sprintf("%d", decimals))

		result := &types.TransactionResult{
			Transactions: []types.Transaction{
				{Sequence: 1, Chain: types.EVMChainName(chainID), ChainID: chainID.String(), Action: "withdraw", SigningMode: types.SigningModeECDSA, UnsignedTxHex: stripHexPrefix(withdrawHex), TxDetails: dets},
			},
		}
		return result.ToToolResult()
	}
}

func handleBorrow(store *vault.Store, ethClient *ethereum.Client, pc *ProtocolClient, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		assetStr, err := req.RequireString("asset")
		if err != nil {
			return mcp.NewToolResultError("missing asset"), nil
		}
		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount"), nil
		}
		explicit := req.GetString("address", "")

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		asset := ethcommon.HexToAddress(assetStr)
		user := ethcommon.HexToAddress(addr)
		pool := pc.PoolAddress()

		decimals, err := pc.GetTokenDecimals(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token decimals: %v", err)), nil
		}
		symbol, err := pc.GetTokenSymbol(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token symbol: %v", err)), nil
		}

		amount, err := ParseAmount(amountStr, int(decimals))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		nonce, err := ethClient.PendingNonceAt(ctx, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get nonce: %v", err)), nil
		}

		borrowData := EncodeBorrow(asset, amount, user)
		borrowHex, borrowTx, err := ethClient.BuildUnsignedTx(ctx, user, pool, borrowData, big.NewInt(0), chainID, nonce, 0)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build borrow tx: %v", err)), nil
		}

		dets := evmTxDetails(pool.Hex(), nonce, borrowTx.Gas(), borrowTx.GasPrice().String(), fmt.Sprintf("0x%x", borrowData), fmt.Sprintf("Borrow %s from Aave V3 (variable rate)", symbol))
		dets["contract_name"] = "Aave V3 Pool"
		addTokenFields(dets, symbol, asset.Hex(), amountStr, amount.String(), fmt.Sprintf("%d", decimals))

		result := &types.TransactionResult{
			Transactions: []types.Transaction{
				{Sequence: 1, Chain: types.EVMChainName(chainID), ChainID: chainID.String(), Action: "borrow", SigningMode: types.SigningModeECDSA, UnsignedTxHex: stripHexPrefix(borrowHex), TxDetails: dets},
			},
		}
		return result.ToToolResult()
	}
}

func handleRepay(store *vault.Store, ethClient *ethereum.Client, pc *ProtocolClient, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		assetStr, err := req.RequireString("asset")
		if err != nil {
			return mcp.NewToolResultError("missing asset"), nil
		}
		amountStr, err := req.RequireString("amount")
		if err != nil {
			return mcp.NewToolResultError("missing amount"), nil
		}
		explicit := req.GetString("address", "")

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		asset := ethcommon.HexToAddress(assetStr)
		user := ethcommon.HexToAddress(addr)
		pool := pc.PoolAddress()

		decimals, err := pc.GetTokenDecimals(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token decimals: %v", err)), nil
		}
		symbol, err := pc.GetTokenSymbol(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token symbol: %v", err)), nil
		}

		amount, err := ParseAmount(amountStr, int(decimals))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		nonce, err := ethClient.PendingNonceAt(ctx, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get nonce: %v", err)), nil
		}

		chainName := types.EVMChainName(chainID)
		chainIDStr := chainID.String()
		assetChecksummed := asset.Hex()
		poolChecksummed := pool.Hex()
		decimalsStr := fmt.Sprintf("%d", decimals)

		// Tx 1: approve
		approveData := EncodeApprove(pool, amount)
		approveHex, approveTx, err := ethClient.BuildUnsignedTx(ctx, user, asset, approveData, big.NewInt(0), chainID, nonce, 0)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build approve tx: %v", err)), nil
		}
		approveDets := evmTxDetails(assetChecksummed, nonce, approveTx.Gas(), approveTx.GasPrice().String(), fmt.Sprintf("0x%x", approveData), fmt.Sprintf("Approve %s spending for Aave V3 Pool", symbol))
		approveDets["contract_name"] = fmt.Sprintf("%s Token", symbol)
		addTokenFields(approveDets, symbol, assetChecksummed, amountStr, amount.String(), decimalsStr)

		// Tx 2: repay (depends on approve — fixed gas)
		repayData := EncodeRepay(asset, amount, user)
		repayHex, repayTx, err := ethClient.BuildUnsignedTx(ctx, user, pool, repayData, big.NewInt(0), chainID, nonce+1, gasLimitRepay)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build repay tx: %v", err)), nil
		}
		repayDets := evmTxDetails(poolChecksummed, nonce+1, repayTx.Gas(), repayTx.GasPrice().String(), fmt.Sprintf("0x%x", repayData), fmt.Sprintf("Repay %s to Aave V3 (variable rate)", symbol))
		repayDets["contract_name"] = "Aave V3 Pool"
		addTokenFields(repayDets, symbol, assetChecksummed, amountStr, amount.String(), decimalsStr)

		result := &types.TransactionResult{
			Transactions: []types.Transaction{
				{Sequence: 1, Chain: chainName, ChainID: chainIDStr, Action: "approve", SigningMode: types.SigningModeECDSA, UnsignedTxHex: stripHexPrefix(approveHex), TxDetails: approveDets},
				{Sequence: 2, Chain: chainName, ChainID: chainIDStr, Action: "repay", SigningMode: types.SigningModeECDSA, UnsignedTxHex: stripHexPrefix(repayHex), TxDetails: repayDets},
			},
		}
		return result.ToToolResult()
	}
}

func handleGetBalances(store *vault.Store, pc *ProtocolClient) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		user := ethcommon.HexToAddress(addr)
		acct, err := pc.GetUserAccountData(ctx, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get account data: %v", err)), nil
		}

		// Base currency values are 8-decimal USD.
		collateral := ethereum.FormatUnits(acct.TotalCollateralBase, 8)
		debt := ethereum.FormatUnits(acct.TotalDebtBase, 8)
		available := ethereum.FormatUnits(acct.AvailableBorrowsBase, 8)

		// Liquidation threshold and LTV are in bps (10000 = 100%).
		liqThresholdPct := fmt.Sprintf("%.2f%%", float64(acct.CurrentLiquidationThreshold.Int64())/100.0)
		ltvPct := fmt.Sprintf("%.2f%%", float64(acct.LTV.Int64())/100.0)

		// Health factor is 18-decimal WAD.
		healthFactor := ethereum.FormatUnits(acct.HealthFactor, 18)
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

func handleGetRates(pc *ProtocolClient) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		assetStr, err := req.RequireString("asset")
		if err != nil {
			return mcp.NewToolResultError("missing asset"), nil
		}

		asset := ethcommon.HexToAddress(assetStr)

		symbol, err := pc.GetTokenSymbol(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get token symbol: %v", err)), nil
		}

		reserveData, err := pc.GetReserveData(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get reserve data: %v", err)), nil
		}

		configData, err := pc.GetReserveConfigData(ctx, asset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get reserve config: %v", err)), nil
		}

		supplyAPY := RayToAPY(reserveData.LiquidityRate)
		borrowAPY := RayToAPY(reserveData.VariableBorrowRate)

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
