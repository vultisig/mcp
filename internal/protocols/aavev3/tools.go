package aavev3

import (
	"context"
	"fmt"
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/ethereum"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

// Protocol implements the protocols.Protocol interface for Aave V3.
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

		// Query token info.
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

		// Get nonce.
		nonce, err := ethClient.PendingNonceAt(ctx, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get nonce: %v", err)), nil
		}

		// Tx 1: approve(Pool, amount) on asset token.
		approveData := EncodeApprove(pool, amount)
		approveHex, approveTx, err := ethClient.BuildUnsignedTx(ctx, user, asset, approveData, big.NewInt(0), chainID, nonce)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build approve tx: %v", err)), nil
		}

		// Tx 2: supply(asset, amount, user, 0) on Pool.
		supplyData := EncodeSupply(asset, amount, user)
		supplyHex, supplyTx, err := ethClient.BuildUnsignedTx(ctx, user, pool, supplyData, big.NewInt(0), chainID, nonce+1)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build supply tx: %v", err)), nil
		}

		result := fmt.Sprintf(`Aave V3 Deposit
Chain ID: %s
Asset: %s (%s)
Amount: %s (%s wei, %d decimals)
User: %s

Transaction 1 of 2 — Approve %s:
  To: %s (asset token)
  Value: 0
  Nonce: %d
  Gas Limit: %d
  Gas Price: %s wei
  Data: 0x%x
  Unsigned Raw Tx: %s

Transaction 2 of 2 — Supply to Aave V3:
  To: %s (Pool)
  Value: 0
  Nonce: %d
  Gas Limit: %d
  Gas Price: %s wei
  Data: 0x%x
  Unsigned Raw Tx: %s

Sign each unsigned raw transaction and broadcast in order.`,
			chainID.String(),
			assetStr, symbol,
			amountStr, amount.String(), decimals,
			addr,
			symbol,
			assetStr,
			nonce, approveTx.Gas(), approveTx.GasPrice().String(),
			approveData,
			approveHex,
			pool.Hex(),
			nonce+1, supplyTx.Gas(), supplyTx.GasPrice().String(),
			supplyData,
			supplyHex,
		)

		return mcp.NewToolResultText(result), nil
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
		withdrawHex, withdrawTx, err := ethClient.BuildUnsignedTx(ctx, user, pool, withdrawData, big.NewInt(0), chainID, nonce)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build withdraw tx: %v", err)), nil
		}

		result := fmt.Sprintf(`Aave V3 Withdraw
Chain ID: %s
Asset: %s (%s)
Amount: %s (%s wei, %d decimals)
User: %s

Transaction 1 of 1 — Withdraw from Aave V3:
  To: %s (Pool)
  Value: 0
  Nonce: %d
  Gas Limit: %d
  Gas Price: %s wei
  Data: 0x%x
  Unsigned Raw Tx: %s

Sign the unsigned raw transaction and broadcast.`,
			chainID.String(),
			assetStr, symbol,
			amountStr, amount.String(), decimals,
			addr,
			pool.Hex(),
			nonce, withdrawTx.Gas(), withdrawTx.GasPrice().String(),
			withdrawData,
			withdrawHex,
		)

		return mcp.NewToolResultText(result), nil
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
		borrowHex, borrowTx, err := ethClient.BuildUnsignedTx(ctx, user, pool, borrowData, big.NewInt(0), chainID, nonce)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build borrow tx: %v", err)), nil
		}

		result := fmt.Sprintf(`Aave V3 Borrow
Chain ID: %s
Asset: %s (%s)
Amount: %s (%s wei, %d decimals)
Interest Rate: Variable
User: %s

Transaction 1 of 1 — Borrow from Aave V3:
  To: %s (Pool)
  Value: 0
  Nonce: %d
  Gas Limit: %d
  Gas Price: %s wei
  Data: 0x%x
  Unsigned Raw Tx: %s

Sign the unsigned raw transaction and broadcast.`,
			chainID.String(),
			assetStr, symbol,
			amountStr, amount.String(), decimals,
			addr,
			pool.Hex(),
			nonce, borrowTx.Gas(), borrowTx.GasPrice().String(),
			borrowData,
			borrowHex,
		)

		return mcp.NewToolResultText(result), nil
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

		// Tx 1: approve(Pool, amount) on asset token.
		approveData := EncodeApprove(pool, amount)
		approveHex, approveTx, err := ethClient.BuildUnsignedTx(ctx, user, asset, approveData, big.NewInt(0), chainID, nonce)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build approve tx: %v", err)), nil
		}

		// Tx 2: repay(asset, amount, 2, user) on Pool.
		repayData := EncodeRepay(asset, amount, user)
		repayHex, repayTx, err := ethClient.BuildUnsignedTx(ctx, user, pool, repayData, big.NewInt(0), chainID, nonce+1)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to build repay tx: %v", err)), nil
		}

		result := fmt.Sprintf(`Aave V3 Repay
Chain ID: %s
Asset: %s (%s)
Amount: %s (%s wei, %d decimals)
Interest Rate: Variable
User: %s

Transaction 1 of 2 — Approve %s:
  To: %s (asset token)
  Value: 0
  Nonce: %d
  Gas Limit: %d
  Gas Price: %s wei
  Data: 0x%x
  Unsigned Raw Tx: %s

Transaction 2 of 2 — Repay to Aave V3:
  To: %s (Pool)
  Value: 0
  Nonce: %d
  Gas Limit: %d
  Gas Price: %s wei
  Data: 0x%x
  Unsigned Raw Tx: %s

Sign each unsigned raw transaction and broadcast in order.`,
			chainID.String(),
			assetStr, symbol,
			amountStr, amount.String(), decimals,
			addr,
			symbol,
			assetStr,
			nonce, approveTx.Gas(), approveTx.GasPrice().String(),
			approveData,
			approveHex,
			pool.Hex(),
			nonce+1, repayTx.Gas(), repayTx.GasPrice().String(),
			repayData,
			repayHex,
		)

		return mcp.NewToolResultText(result), nil
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
