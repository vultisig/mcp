package sky

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

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
	gasLimitDeposit  = 200_000
	gasLimitWithdraw = 200_000
	gasLimitRedeem   = 200_000
	gasLimitApprove  = 60_000
)

// Protocol implements the protocols.Protocol interface for Sky/Maker.
type Protocol struct{}

func (p *Protocol) Name() string { return "sky" }

func (p *Protocol) SupportsChain(chainID *big.Int) bool {
	return chainID.Int64() == 1 // Ethereum mainnet only
}

func (p *Protocol) Register(s *server.MCPServer, store *vault.Store, ethClient *evmclient.Client, evmSDK *evmsdk.SDK, chainID *big.Int) {
	toolmeta.Register(s, newDepositTool(), handleDeposit(store, evmSDK, chainID), "sky")
	toolmeta.Register(s, newWithdrawTool(), handleWithdraw(store, evmSDK, chainID), "sky")
	toolmeta.Register(s, newRedeemTool(), handleRedeem(store, evmSDK, chainID), "sky")
	toolmeta.Register(s, newGetBalancesTool(), handleGetBalances(store, ethClient), "sky")
	toolmeta.Register(s, newGetRatesTool(), handleGetRates(ethClient), "sky")
}

// --- Tool definitions ---

func newDepositTool() mcp.Tool {
	return mcp.NewTool("sky_deposit",
		mcp.WithDescription("Build unsigned transactions to deposit into Sky savings vaults (DAI→sDAI or USDS→sUSDS). Returns an approve tx and a deposit tx. Uses the ERC-4626 vault standard."),
		mcp.WithString("vault", mcp.Description("Vault to deposit into: \"sdai\" (DAI savings) or \"susds\" (USDS savings)"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount of underlying token to deposit in human-readable units (e.g. \"1000\")"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Depositor's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newWithdrawTool() mcp.Tool {
	return mcp.NewTool("sky_withdraw",
		mcp.WithDescription("Build an unsigned transaction to withdraw underlying tokens from a Sky savings vault. Specify the amount of underlying (DAI or USDS) to receive."),
		mcp.WithString("vault", mcp.Description("Vault to withdraw from: \"sdai\" or \"susds\""), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount of underlying token to withdraw in human-readable units"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Owner's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newRedeemTool() mcp.Tool {
	return mcp.NewTool("sky_redeem",
		mcp.WithDescription("Build an unsigned transaction to redeem vault shares for underlying tokens. Specify the number of shares (sDAI or sUSDS) to burn."),
		mcp.WithString("vault", mcp.Description("Vault to redeem from: \"sdai\" or \"susds\""), mcp.Required()),
		mcp.WithString("shares", mcp.Description("Number of vault shares to redeem in human-readable units"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Owner's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newGetBalancesTool() mcp.Tool {
	return mcp.NewTool("sky_get_balances",
		mcp.WithDescription("Query Sky savings balances: sDAI and sUSDS share balances with their underlying DAI/USDS values."),
		mcp.WithString("address", mcp.Description("Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newGetRatesTool() mcp.Tool {
	return mcp.NewTool("sky_get_rates",
		mcp.WithDescription("Query current Sky savings vault exchange rates and total deposits for sDAI and sUSDS."),
	)
}

// --- Handlers ---

func handleDeposit(store *vault.Store, evmSDK *evmsdk.SDK, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		vc, err := resolveVault(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		amountStr, addr, err := extractParams(ctx, req, store, "amount")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		weiAmount, err := ParseAmount(amountStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		user := ethcommon.HexToAddress(addr)

		// 1. Approve underlying token spending by vault
		approveData, err := erc20ABI.Pack("approve", vc.VaultAddress, weiAmount)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode approve: %v", err)), nil
		}

		approveTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, vc.UnderlyingAddress, big.NewInt(0), approveData, 0, gasLimitApprove)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build approve tx: %v", err)), nil
		}

		// 2. Deposit into vault
		depositData, err := erc4626ABI.Pack("deposit", weiAmount, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode deposit: %v", err)), nil
		}

		depositTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, vc.VaultAddress, big.NewInt(0), depositData, 1, gasLimitDeposit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build deposit tx: %v", err)), nil
		}

		return buildMultiTxResult(chainID, []unsignedTxInfo{
			{approveTx, approveData, vc.UnderlyingAddress, "approve", fmt.Sprintf("Approve %s spending for %s vault", vc.UnderlyingSymbol, vc.VaultSymbol), fmt.Sprintf("%s Token", vc.UnderlyingSymbol), big.NewInt(0)},
			{depositTx, depositData, vc.VaultAddress, "deposit", fmt.Sprintf("Deposit %s into %s vault", vc.UnderlyingSymbol, vc.VaultSymbol), vc.VaultSymbol, big.NewInt(0)},
		}, vc.UnderlyingSymbol, vc.UnderlyingAddress.Hex(), amountStr, weiAmount.String(), fmt.Sprintf("%d", vc.Decimals))
	}
}

func handleWithdraw(store *vault.Store, evmSDK *evmsdk.SDK, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		vc, err := resolveVault(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		amountStr, addr, err := extractParams(ctx, req, store, "amount")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		weiAmount, err := ParseAmount(amountStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		user := ethcommon.HexToAddress(addr)

		// withdraw(assets, receiver, owner)
		withdrawData, err := erc4626ABI.Pack("withdraw", weiAmount, user, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode withdraw: %v", err)), nil
		}

		withdrawTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, vc.VaultAddress, big.NewInt(0), withdrawData, 0, gasLimitWithdraw)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build withdraw tx: %v", err)), nil
		}

		nonce, gasLimit, maxFee, maxPriorityFee, err := decodeTxFields(withdrawTx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode tx: %v", err)), nil
		}

		dets := txDetails(vc.VaultAddress.Hex(), "0", nonce, gasLimit, maxFee, maxPriorityFee, withdrawData, fmt.Sprintf("Withdraw %s from %s vault", vc.UnderlyingSymbol, vc.VaultSymbol), vc.VaultSymbol)
		dets["token_symbol"] = vc.UnderlyingSymbol
		dets["token_address"] = vc.UnderlyingAddress.Hex()
		dets["amount_human"] = amountStr
		dets["amount_wei"] = weiAmount.String()
		dets["decimals"] = fmt.Sprintf("%d", vc.Decimals)

		result := &types.TransactionResult{
			Transactions: []types.Transaction{{
				Sequence:      1,
				Chain:         types.EVMChainName(chainID),
				ChainID:       chainID.String(),
				Action:        "withdraw",
				SigningMode:   types.SigningModeECDSA,
				UnsignedTxHex: hex.EncodeToString(withdrawTx),
				TxDetails:     dets,
			}},
		}
		return result.ToToolResult()
	}
}

func handleRedeem(store *vault.Store, evmSDK *evmsdk.SDK, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		vc, err := resolveVault(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sharesStr, addr, err := extractParams(ctx, req, store, "shares")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		weiShares, err := ParseAmount(sharesStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid shares: %v", err)), nil
		}

		user := ethcommon.HexToAddress(addr)

		// redeem(shares, receiver, owner)
		redeemData, err := erc4626ABI.Pack("redeem", weiShares, user, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode redeem: %v", err)), nil
		}

		redeemTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, vc.VaultAddress, big.NewInt(0), redeemData, 0, gasLimitRedeem)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build redeem tx: %v", err)), nil
		}

		nonce, gasLimit, maxFee, maxPriorityFee, err := decodeTxFields(redeemTx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode tx: %v", err)), nil
		}

		dets := txDetails(vc.VaultAddress.Hex(), "0", nonce, gasLimit, maxFee, maxPriorityFee, redeemData, fmt.Sprintf("Redeem %s shares for %s", vc.VaultSymbol, vc.UnderlyingSymbol), vc.VaultSymbol)
		dets["token_symbol"] = vc.VaultSymbol
		dets["token_address"] = vc.VaultAddress.Hex()
		dets["amount_human"] = sharesStr
		dets["amount_wei"] = weiShares.String()
		dets["decimals"] = fmt.Sprintf("%d", vc.Decimals)

		result := &types.TransactionResult{
			Transactions: []types.Transaction{{
				Sequence:      1,
				Chain:         types.EVMChainName(chainID),
				ChainID:       chainID.String(),
				Action:        "redeem",
				SigningMode:   types.SigningModeECDSA,
				UnsignedTxHex: hex.EncodeToString(redeemTx),
				TxDetails:     dets,
			}},
		}
		return result.ToToolResult()
	}
}

func handleGetBalances(store *vault.Store, ethClient *evmclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")

		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		user := ethcommon.HexToAddress(addr)
		eth := ethClient.ETH()

		var lines string
		lines = fmt.Sprintf("Sky Savings Balances\nAddress: %s\n", addr)

		for _, name := range []string{"sdai", "susds"} {
			vc := Vaults[name]

			// Query vault share balance
			balData, err := erc4626ABI.Pack("balanceOf", user)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("encode %s balanceOf: %v", vc.VaultSymbol, err)), nil
			}
			balResult, err := eth.CallContract(ctx, callMsg(vc.VaultAddress, balData), nil)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query %s balance: %v", vc.VaultSymbol, err)), nil
			}
			shareBal := new(big.Int).SetBytes(balResult)

			// Convert shares to underlying
			var underlyingVal *big.Int
			if shareBal.Sign() > 0 {
				convData, err := erc4626ABI.Pack("convertToAssets", shareBal)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("encode convertToAssets: %v", err)), nil
				}
				convResult, err := eth.CallContract(ctx, callMsg(vc.VaultAddress, convData), nil)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("query conversion: %v", err)), nil
				}
				underlyingVal = new(big.Int).SetBytes(convResult)
			} else {
				underlyingVal = big.NewInt(0)
			}

			lines += fmt.Sprintf("\n%s Balance: %s %s (≈ %s %s)",
				vc.VaultSymbol,
				evmclient.FormatUnits(shareBal, 18),
				vc.VaultSymbol,
				evmclient.FormatUnits(underlyingVal, 18),
				vc.UnderlyingSymbol,
			)
		}

		return mcp.NewToolResultText(lines), nil
	}
}

func handleGetRates(ethClient *evmclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		eth := ethClient.ETH()
		oneToken := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

		var lines string
		lines = "Sky Savings Vault Rates\n"

		for _, name := range []string{"sdai", "susds"} {
			vc := Vaults[name]

			// Exchange rate: 1 share → underlying
			convData, err := erc4626ABI.Pack("convertToAssets", oneToken)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("encode convertToAssets: %v", err)), nil
			}
			convResult, err := eth.CallContract(ctx, callMsg(vc.VaultAddress, convData), nil)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query %s rate: %v", vc.VaultSymbol, err)), nil
			}
			rateWei := new(big.Int).SetBytes(convResult)

			// Total assets in vault
			totalData, err := erc4626ABI.Pack("totalAssets")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("encode totalAssets: %v", err)), nil
			}
			totalResult, err := eth.CallContract(ctx, callMsg(vc.VaultAddress, totalData), nil)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query %s totalAssets: %v", vc.VaultSymbol, err)), nil
			}
			totalAssets := new(big.Int).SetBytes(totalResult)

			lines += fmt.Sprintf("\n%s (%s → %s):\n  Exchange Rate: 1 %s = %s %s\n  Total Deposits: %s %s",
				vc.VaultSymbol,
				vc.UnderlyingSymbol,
				vc.VaultSymbol,
				vc.VaultSymbol,
				evmclient.FormatUnits(rateWei, 18),
				vc.UnderlyingSymbol,
				evmclient.FormatUnits(totalAssets, 18),
				vc.UnderlyingSymbol,
			)
		}

		return mcp.NewToolResultText(lines), nil
	}
}

// --- Helpers ---

func resolveVault(req mcp.CallToolRequest) (VaultConfig, error) {
	vaultName, err := req.RequireString("vault")
	if err != nil {
		return VaultConfig{}, fmt.Errorf("missing vault: must be \"sdai\" or \"susds\"")
	}

	vc, ok := Vaults[vaultName]
	if !ok {
		return VaultConfig{}, fmt.Errorf("invalid vault %q: must be \"sdai\" or \"susds\"", vaultName)
	}
	return vc, nil
}

func extractParams(ctx context.Context, req mcp.CallToolRequest, store *vault.Store, amountField string) (amount, addr string, err error) {
	amount, err = req.RequireString(amountField)
	if err != nil {
		return "", "", fmt.Errorf("missing %s", amountField)
	}

	explicit := req.GetString("address", "")
	addr, err = resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
	if err != nil {
		return "", "", err
	}
	return amount, addr, nil
}

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
