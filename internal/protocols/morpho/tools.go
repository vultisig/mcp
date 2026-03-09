package morpho

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
	gasLimitSupply   = 300_000
	gasLimitWithdraw = 300_000
	gasLimitBorrow   = 300_000
	gasLimitRepay    = 300_000
	gasLimitApprove  = 60_000
)

// Protocol implements the protocols.Protocol interface for Morpho Blue.
type Protocol struct{}

func (p *Protocol) Name() string { return "morpho" }

func (p *Protocol) SupportsChain(chainID *big.Int) bool {
	return chainID.Int64() == 1 // Ethereum mainnet only
}

func (p *Protocol) Register(s *server.MCPServer, store *vault.Store, ethClient *evmclient.Client, evmSDK *evmsdk.SDK, chainID *big.Int) {
	toolmeta.Register(s, newSupplyTool(), handleSupply(store, evmSDK, ethClient, chainID), "morpho")
	toolmeta.Register(s, newWithdrawTool(), handleWithdraw(store, evmSDK, ethClient, chainID), "morpho")
	toolmeta.Register(s, newBorrowTool(), handleBorrow(store, evmSDK, ethClient, chainID), "morpho")
	toolmeta.Register(s, newRepayTool(), handleRepay(store, evmSDK, ethClient, chainID), "morpho")
	toolmeta.Register(s, newGetPositionTool(), handleGetPosition(store, ethClient), "morpho")
}

// --- Tool definitions ---

func newSupplyTool() mcp.Tool {
	return mcp.NewTool("morpho_supply",
		mcp.WithDescription("Build unsigned transactions to supply loan tokens to a Morpho Blue market. Returns an approve tx and a supply tx. The market_id identifies which market to supply to."),
		mcp.WithString("market_id", mcp.Description("Morpho market ID (0x-prefixed, 64 hex chars)"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount of loan token to supply in human-readable units"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Supplier's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newWithdrawTool() mcp.Tool {
	return mcp.NewTool("morpho_withdraw",
		mcp.WithDescription("Build an unsigned transaction to withdraw supplied loan tokens from a Morpho Blue market."),
		mcp.WithString("market_id", mcp.Description("Morpho market ID (0x-prefixed, 64 hex chars)"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount of loan token to withdraw in human-readable units"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Withdrawer's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newBorrowTool() mcp.Tool {
	return mcp.NewTool("morpho_borrow",
		mcp.WithDescription("Build an unsigned transaction to borrow loan tokens from a Morpho Blue market. Requires collateral already deposited."),
		mcp.WithString("market_id", mcp.Description("Morpho market ID (0x-prefixed, 64 hex chars)"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount of loan token to borrow in human-readable units"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Borrower's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newRepayTool() mcp.Tool {
	return mcp.NewTool("morpho_repay",
		mcp.WithDescription("Build unsigned transactions to repay a borrow on Morpho Blue. Returns an approve tx and a repay tx."),
		mcp.WithString("market_id", mcp.Description("Morpho market ID (0x-prefixed, 64 hex chars)"), mcp.Required()),
		mcp.WithString("amount", mcp.Description("Amount of loan token to repay in human-readable units"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Repayer's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newGetPositionTool() mcp.Tool {
	return mcp.NewTool("morpho_get_position",
		mcp.WithDescription("Query a user's position in a Morpho Blue market: supply shares, borrow shares, and collateral amount."),
		mcp.WithString("market_id", mcp.Description("Morpho market ID (0x-prefixed, 64 hex chars)"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

// --- Handlers ---

func handleSupply(store *vault.Store, evmSDK *evmsdk.SDK, ethClient *evmclient.Client, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		marketID, amountStr, addr, err := extractMarketParams(ctx, req, store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Resolve market params from on-chain
		mp, err := queryMarketParams(ctx, ethClient, marketID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("resolve market: %v", err)), nil
		}

		// Get loan token decimals
		decimals, err := queryDecimals(ctx, ethClient, mp.LoanToken)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get decimals: %v", err)), nil
		}

		weiAmount, err := ParseAmount(amountStr, decimals)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		user := ethcommon.HexToAddress(addr)

		// 1. Approve loan token spending by Morpho
		approveData, err := erc20ABI.Pack("approve", MorphoAddress, weiAmount)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode approve: %v", err)), nil
		}

		approveTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, mp.LoanToken, big.NewInt(0), approveData, 0, gasLimitApprove)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build approve tx: %v", err)), nil
		}

		// 2. Supply to Morpho
		supplyData, err := morphoABI.Pack("supply", mp, weiAmount, big.NewInt(0), user, []byte{})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode supply: %v", err)), nil
		}

		supplyTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, MorphoAddress, big.NewInt(0), supplyData, 1, gasLimitSupply)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build supply tx: %v", err)), nil
		}

		symbol := querySymbolSafe(ctx, ethClient, mp.LoanToken)

		return buildMultiTxResult(chainID, []unsignedTxInfo{
			{approveTx, approveData, mp.LoanToken, "approve", fmt.Sprintf("Approve %s spending for Morpho Blue", symbol), fmt.Sprintf("%s Token", symbol), big.NewInt(0)},
			{supplyTx, supplyData, MorphoAddress, "supply", fmt.Sprintf("Supply %s to Morpho Blue market", symbol), "Morpho Blue", big.NewInt(0)},
		}, symbol, mp.LoanToken.Hex(), amountStr, weiAmount.String(), fmt.Sprintf("%d", decimals))
	}
}

func handleWithdraw(store *vault.Store, evmSDK *evmsdk.SDK, ethClient *evmclient.Client, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		marketID, amountStr, addr, err := extractMarketParams(ctx, req, store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		mp, err := queryMarketParams(ctx, ethClient, marketID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("resolve market: %v", err)), nil
		}

		decimals, err := queryDecimals(ctx, ethClient, mp.LoanToken)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get decimals: %v", err)), nil
		}

		weiAmount, err := ParseAmount(amountStr, decimals)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		user := ethcommon.HexToAddress(addr)

		// withdraw(MarketParams, assets, shares=0, onBehalf, receiver)
		withdrawData, err := morphoABI.Pack("withdraw", mp, weiAmount, big.NewInt(0), user, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode withdraw: %v", err)), nil
		}

		withdrawTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, MorphoAddress, big.NewInt(0), withdrawData, 0, gasLimitWithdraw)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build withdraw tx: %v", err)), nil
		}

		symbol := querySymbolSafe(ctx, ethClient, mp.LoanToken)

		nonce, gasLimit, maxFee, maxPriorityFee, err := decodeTxFields(withdrawTx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode tx: %v", err)), nil
		}

		dets := txDetails(MorphoAddress.Hex(), "0", nonce, gasLimit, maxFee, maxPriorityFee, withdrawData, fmt.Sprintf("Withdraw %s from Morpho Blue market", symbol), "Morpho Blue")
		dets["token_symbol"] = symbol
		dets["token_address"] = mp.LoanToken.Hex()
		dets["amount_human"] = amountStr
		dets["amount_wei"] = weiAmount.String()
		dets["decimals"] = fmt.Sprintf("%d", decimals)

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

func handleBorrow(store *vault.Store, evmSDK *evmsdk.SDK, ethClient *evmclient.Client, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		marketID, amountStr, addr, err := extractMarketParams(ctx, req, store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		mp, err := queryMarketParams(ctx, ethClient, marketID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("resolve market: %v", err)), nil
		}

		decimals, err := queryDecimals(ctx, ethClient, mp.LoanToken)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get decimals: %v", err)), nil
		}

		weiAmount, err := ParseAmount(amountStr, decimals)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		user := ethcommon.HexToAddress(addr)

		// borrow(MarketParams, assets, shares=0, onBehalf, receiver)
		borrowData, err := morphoABI.Pack("borrow", mp, weiAmount, big.NewInt(0), user, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode borrow: %v", err)), nil
		}

		borrowTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, MorphoAddress, big.NewInt(0), borrowData, 0, gasLimitBorrow)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build borrow tx: %v", err)), nil
		}

		symbol := querySymbolSafe(ctx, ethClient, mp.LoanToken)

		nonce, gasLimit, maxFee, maxPriorityFee, err := decodeTxFields(borrowTx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode tx: %v", err)), nil
		}

		dets := txDetails(MorphoAddress.Hex(), "0", nonce, gasLimit, maxFee, maxPriorityFee, borrowData, fmt.Sprintf("Borrow %s from Morpho Blue market", symbol), "Morpho Blue")
		dets["token_symbol"] = symbol
		dets["token_address"] = mp.LoanToken.Hex()
		dets["amount_human"] = amountStr
		dets["amount_wei"] = weiAmount.String()
		dets["decimals"] = fmt.Sprintf("%d", decimals)

		result := &types.TransactionResult{
			Transactions: []types.Transaction{{
				Sequence:      1,
				Chain:         types.EVMChainName(chainID),
				ChainID:       chainID.String(),
				Action:        "borrow",
				SigningMode:   types.SigningModeECDSA,
				UnsignedTxHex: hex.EncodeToString(borrowTx),
				TxDetails:     dets,
			}},
		}
		return result.ToToolResult()
	}
}

func handleRepay(store *vault.Store, evmSDK *evmsdk.SDK, ethClient *evmclient.Client, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		marketID, amountStr, addr, err := extractMarketParams(ctx, req, store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		mp, err := queryMarketParams(ctx, ethClient, marketID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("resolve market: %v", err)), nil
		}

		decimals, err := queryDecimals(ctx, ethClient, mp.LoanToken)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get decimals: %v", err)), nil
		}

		weiAmount, err := ParseAmount(amountStr, decimals)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		user := ethcommon.HexToAddress(addr)

		// 1. Approve
		approveData, err := erc20ABI.Pack("approve", MorphoAddress, weiAmount)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode approve: %v", err)), nil
		}

		approveTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, mp.LoanToken, big.NewInt(0), approveData, 0, gasLimitApprove)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build approve tx: %v", err)), nil
		}

		// 2. Repay
		repayData, err := morphoABI.Pack("repay", mp, weiAmount, big.NewInt(0), user, []byte{})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode repay: %v", err)), nil
		}

		repayTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, MorphoAddress, big.NewInt(0), repayData, 1, gasLimitRepay)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build repay tx: %v", err)), nil
		}

		symbol := querySymbolSafe(ctx, ethClient, mp.LoanToken)

		return buildMultiTxResult(chainID, []unsignedTxInfo{
			{approveTx, approveData, mp.LoanToken, "approve", fmt.Sprintf("Approve %s spending for Morpho Blue", symbol), fmt.Sprintf("%s Token", symbol), big.NewInt(0)},
			{repayTx, repayData, MorphoAddress, "repay", fmt.Sprintf("Repay %s to Morpho Blue market", symbol), "Morpho Blue", big.NewInt(0)},
		}, symbol, mp.LoanToken.Hex(), amountStr, weiAmount.String(), fmt.Sprintf("%d", decimals))
	}
}

func handleGetPosition(store *vault.Store, ethClient *evmclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		marketIDStr, err := req.RequireString("market_id")
		if err != nil {
			return mcp.NewToolResultError("missing market_id"), nil
		}

		marketID, err := ParseMarketID(marketIDStr)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		explicit := req.GetString("address", "")
		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		user := ethcommon.HexToAddress(addr)
		eth := ethClient.ETH()

		// Query position(bytes32 id, address user)
		posData, err := morphoABI.Pack("position", marketID, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode position: %v", err)), nil
		}
		posResult, err := eth.CallContract(ctx, callMsg(MorphoAddress, posData), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query position: %v", err)), nil
		}

		posValues, err := morphoABI.Unpack("position", posResult)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode position: %v", err)), nil
		}

		supplyShares := posValues[0].(*big.Int)
		borrowShares := posValues[1].(*big.Int)
		collateral := posValues[2].(*big.Int)

		// Query market state for totals
		mktData, err := morphoABI.Pack("market", marketID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode market: %v", err)), nil
		}
		mktResult, err := eth.CallContract(ctx, callMsg(MorphoAddress, mktData), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query market: %v", err)), nil
		}

		mktValues, err := morphoABI.Unpack("market", mktResult)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode market: %v", err)), nil
		}

		totalSupplyAssets := mktValues[0].(*big.Int)
		totalSupplyShares := mktValues[1].(*big.Int)
		totalBorrowAssets := mktValues[2].(*big.Int)
		totalBorrowShares := mktValues[3].(*big.Int)

		// Convert shares to approximate assets
		var supplyAssets, borrowAssets *big.Int
		if totalSupplyShares.Sign() > 0 && supplyShares.Sign() > 0 {
			supplyAssets = new(big.Int).Mul(supplyShares, totalSupplyAssets)
			supplyAssets.Div(supplyAssets, totalSupplyShares)
		} else {
			supplyAssets = big.NewInt(0)
		}
		if totalBorrowShares.Sign() > 0 && borrowShares.Sign() > 0 {
			borrowAssets = new(big.Int).Mul(borrowShares, totalBorrowAssets)
			borrowAssets.Div(borrowAssets, totalBorrowShares)
		} else {
			borrowAssets = big.NewInt(0)
		}

		// Try to resolve market params for token info
		mp, mpErr := queryMarketParams(ctx, ethClient, marketID)
		loanSymbol := "tokens"
		collatSymbol := "tokens"
		loanDecimals := 18
		collatDecimals := 18
		if mpErr == nil {
			loanSymbol = querySymbolSafe(ctx, ethClient, mp.LoanToken)
			collatSymbol = querySymbolSafe(ctx, ethClient, mp.CollateralToken)
			if d, err := queryDecimals(ctx, ethClient, mp.LoanToken); err == nil {
				loanDecimals = d
			}
			if d, err := queryDecimals(ctx, ethClient, mp.CollateralToken); err == nil {
				collatDecimals = d
			}
		}

		text := fmt.Sprintf(`Morpho Blue Position
Address: %s
Market ID: %s

Supply: %s %s (shares: %s)
Borrow: %s %s (shares: %s)
Collateral: %s %s

Market Totals:
  Total Supply: %s %s
  Total Borrow: %s %s`,
			addr,
			marketIDStr,
			evmclient.FormatUnits(supplyAssets, loanDecimals), loanSymbol, supplyShares.String(),
			evmclient.FormatUnits(borrowAssets, loanDecimals), loanSymbol, borrowShares.String(),
			evmclient.FormatUnits(collateral, collatDecimals), collatSymbol,
			evmclient.FormatUnits(totalSupplyAssets, loanDecimals), loanSymbol,
			evmclient.FormatUnits(totalBorrowAssets, loanDecimals), loanSymbol,
		)

		return mcp.NewToolResultText(text), nil
	}
}

// --- On-chain queries ---

func queryMarketParams(ctx context.Context, ethClient *evmclient.Client, marketID [32]byte) (MarketParams, error) {
	eth := ethClient.ETH()

	data, err := morphoABI.Pack("idToMarketParams", marketID)
	if err != nil {
		return MarketParams{}, fmt.Errorf("encode idToMarketParams: %w", err)
	}

	result, err := eth.CallContract(ctx, callMsg(MorphoAddress, data), nil)
	if err != nil {
		return MarketParams{}, fmt.Errorf("call idToMarketParams: %w", err)
	}

	values, err := morphoABI.Unpack("idToMarketParams", result)
	if err != nil {
		return MarketParams{}, fmt.Errorf("decode idToMarketParams: %w", err)
	}

	return MarketParams{
		LoanToken:       values[0].(ethcommon.Address),
		CollateralToken: values[1].(ethcommon.Address),
		Oracle:          values[2].(ethcommon.Address),
		Irm:             values[3].(ethcommon.Address),
		Lltv:            values[4].(*big.Int),
	}, nil
}

func queryDecimals(ctx context.Context, ethClient *evmclient.Client, token ethcommon.Address) (int, error) {
	eth := ethClient.ETH()

	data, err := erc20ABI.Pack("decimals")
	if err != nil {
		return 0, err
	}

	result, err := eth.CallContract(ctx, callMsg(token, data), nil)
	if err != nil {
		return 0, err
	}

	values, err := erc20ABI.Unpack("decimals", result)
	if err != nil {
		return 0, err
	}

	return int(values[0].(uint8)), nil
}

func querySymbolSafe(ctx context.Context, ethClient *evmclient.Client, token ethcommon.Address) string {
	eth := ethClient.ETH()

	data, err := erc20ABI.Pack("symbol")
	if err != nil {
		return "TOKEN"
	}

	result, err := eth.CallContract(ctx, callMsg(token, data), nil)
	if err != nil {
		return "TOKEN"
	}

	values, err := erc20ABI.Unpack("symbol", result)
	if err != nil {
		return "TOKEN"
	}

	return values[0].(string)
}

// --- Helpers ---

func extractMarketParams(ctx context.Context, req mcp.CallToolRequest, store *vault.Store) ([32]byte, string, string, error) {
	marketIDStr, err := req.RequireString("market_id")
	if err != nil {
		return [32]byte{}, "", "", fmt.Errorf("missing market_id")
	}

	marketID, err := ParseMarketID(marketIDStr)
	if err != nil {
		return [32]byte{}, "", "", err
	}

	amountStr, err := req.RequireString("amount")
	if err != nil {
		return [32]byte{}, "", "", fmt.Errorf("missing amount")
	}

	explicit := req.GetString("address", "")
	addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
	if err != nil {
		return [32]byte{}, "", "", err
	}

	return marketID, amountStr, addr, nil
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
