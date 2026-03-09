package etherfi

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
	gasLimitStake   = 150_000
	gasLimitWrap    = 100_000
	gasLimitUnwrap  = 100_000
	gasLimitApprove = 60_000
)

// Protocol implements the protocols.Protocol interface for Ether.fi.
type Protocol struct{}

func (p *Protocol) Name() string { return "etherfi" }

func (p *Protocol) SupportsChain(chainID *big.Int) bool {
	return chainID.Int64() == 1 // Ethereum mainnet only
}

func (p *Protocol) Register(s *server.MCPServer, store *vault.Store, ethClient *evmclient.Client, evmSDK *evmsdk.SDK, chainID *big.Int) {
	toolmeta.Register(s, newStakeTool(), handleStake(store, evmSDK, chainID), "etherfi")
	toolmeta.Register(s, newWrapTool(), handleWrap(store, evmSDK, chainID), "etherfi")
	toolmeta.Register(s, newUnwrapTool(), handleUnwrap(store, evmSDK, chainID), "etherfi")
	toolmeta.Register(s, newGetBalancesTool(), handleGetBalances(store, ethClient), "etherfi")
	toolmeta.Register(s, newGetRatesTool(), handleGetRates(ethClient), "etherfi")
}

// --- Tool definitions ---

func newStakeTool() mcp.Tool {
	return mcp.NewTool("etherfi_stake",
		mcp.WithDescription("Build an unsigned transaction to stake ETH with Ether.fi and receive eETH. Sends ETH to the Ether.fi liquidity pool."),
		mcp.WithString("amount", mcp.Description("Amount of ETH to stake in human-readable units (e.g. \"0.5\")"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Staker's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newWrapTool() mcp.Tool {
	return mcp.NewTool("etherfi_wrap",
		mcp.WithDescription("Build unsigned transactions to wrap eETH into weETH. Returns an approve tx and a wrap tx. weETH is non-rebasing and better for DeFi integrations."),
		mcp.WithString("amount", mcp.Description("Amount of eETH to wrap in human-readable units"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Owner's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newUnwrapTool() mcp.Tool {
	return mcp.NewTool("etherfi_unwrap",
		mcp.WithDescription("Build an unsigned transaction to unwrap weETH back into eETH. No approval needed."),
		mcp.WithString("amount", mcp.Description("Amount of weETH to unwrap in human-readable units"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Owner's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newGetBalancesTool() mcp.Tool {
	return mcp.NewTool("etherfi_get_balances",
		mcp.WithDescription("Query Ether.fi staking balances: eETH balance, weETH balance, and the equivalent eETH value of weETH holdings."),
		mcp.WithString("address", mcp.Description("Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newGetRatesTool() mcp.Tool {
	return mcp.NewTool("etherfi_get_rates",
		mcp.WithDescription("Query current Ether.fi eETH/weETH exchange rate and total pooled ETH."),
	)
}

// --- Handlers ---

func handleStake(store *vault.Store, evmSDK *evmsdk.SDK, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		amountStr, addr, err := extractParams(ctx, req, store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		weiAmount, err := ParseAmount(amountStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		user := ethcommon.HexToAddress(addr)

		// deposit() with ETH value — no arguments, just payable
		calldata, err := liquidityPoolABI.Pack("deposit")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode deposit: %v", err)), nil
		}

		unsignedTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, LiquidityPoolAddress, weiAmount, calldata, 0, gasLimitStake)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build stake tx: %v", err)), nil
		}

		nonce, gasLimit, maxFee, maxPriorityFee, err := decodeTxFields(unsignedTx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode tx: %v", err)), nil
		}

		dets := txDetails(LiquidityPoolAddress.Hex(), weiAmount.String(), nonce, gasLimit, maxFee, maxPriorityFee, calldata, "Stake ETH with Ether.fi to receive eETH", "Ether.fi Liquidity Pool")
		dets["token_symbol"] = "ETH"
		dets["amount_human"] = amountStr
		dets["amount_wei"] = weiAmount.String()
		dets["decimals"] = "18"

		result := &types.TransactionResult{
			Transactions: []types.Transaction{{
				Sequence:      1,
				Chain:         types.EVMChainName(chainID),
				ChainID:       chainID.String(),
				Action:        "stake",
				SigningMode:   types.SigningModeECDSA,
				UnsignedTxHex: hex.EncodeToString(unsignedTx),
				TxDetails:     dets,
			}},
		}
		return result.ToToolResult()
	}
}

func handleWrap(store *vault.Store, evmSDK *evmsdk.SDK, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		amountStr, addr, err := extractParams(ctx, req, store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		weiAmount, err := ParseAmount(amountStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		user := ethcommon.HexToAddress(addr)

		// 1. Approve eETH spending by weETH contract
		approveData, err := eETHABI.Pack("approve", WeETHAddress, weiAmount)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode approve: %v", err)), nil
		}

		approveTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, EETHAddress, big.NewInt(0), approveData, 0, gasLimitApprove)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build approve tx: %v", err)), nil
		}

		// 2. Wrap eETH to weETH
		wrapData, err := weETHABI.Pack("wrap", weiAmount)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode wrap: %v", err)), nil
		}

		wrapTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, WeETHAddress, big.NewInt(0), wrapData, 1, gasLimitWrap)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build wrap tx: %v", err)), nil
		}

		return buildMultiTxResult(chainID, []unsignedTxInfo{
			{approveTx, approveData, EETHAddress, "approve", "Approve eETH spending for weETH wrapping", "eETH Token", big.NewInt(0)},
			{wrapTx, wrapData, WeETHAddress, "wrap", "Wrap eETH into weETH", "weETH", big.NewInt(0)},
		}, "eETH", EETHAddress.Hex(), amountStr, weiAmount.String(), "18")
	}
}

func handleUnwrap(store *vault.Store, evmSDK *evmsdk.SDK, chainID *big.Int) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		amountStr, addr, err := extractParams(ctx, req, store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		weiAmount, err := ParseAmount(amountStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid amount: %v", err)), nil
		}

		user := ethcommon.HexToAddress(addr)

		unwrapData, err := weETHABI.Pack("unwrap", weiAmount)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode unwrap: %v", err)), nil
		}

		unwrapTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, WeETHAddress, big.NewInt(0), unwrapData, 0, gasLimitUnwrap)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build unwrap tx: %v", err)), nil
		}

		nonce, gasLimit, maxFee, maxPriorityFee, err := decodeTxFields(unwrapTx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode tx: %v", err)), nil
		}

		dets := txDetails(WeETHAddress.Hex(), "0", nonce, gasLimit, maxFee, maxPriorityFee, unwrapData, "Unwrap weETH back to eETH", "weETH")
		dets["token_symbol"] = "weETH"
		dets["token_address"] = WeETHAddress.Hex()
		dets["amount_human"] = amountStr
		dets["amount_wei"] = weiAmount.String()
		dets["decimals"] = "18"

		result := &types.TransactionResult{
			Transactions: []types.Transaction{{
				Sequence:      1,
				Chain:         types.EVMChainName(chainID),
				ChainID:       chainID.String(),
				Action:        "unwrap",
				SigningMode:   types.SigningModeECDSA,
				UnsignedTxHex: hex.EncodeToString(unwrapTx),
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

		// Query eETH balance
		eETHBalData, err := eETHABI.Pack("balanceOf", user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode eETH balanceOf: %v", err)), nil
		}
		eETHResult, err := eth.CallContract(ctx, callMsg(EETHAddress, eETHBalData), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query eETH balance: %v", err)), nil
		}
		eETHBal := new(big.Int).SetBytes(eETHResult)

		// Query weETH balance
		weETHBalData, err := weETHABI.Pack("balanceOf", user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode weETH balanceOf: %v", err)), nil
		}
		weETHResult, err := eth.CallContract(ctx, callMsg(WeETHAddress, weETHBalData), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query weETH balance: %v", err)), nil
		}
		weETHBal := new(big.Int).SetBytes(weETHResult)

		// Convert weETH to eETH equivalent
		var weETHInEETH *big.Int
		if weETHBal.Sign() > 0 {
			convData, err := weETHABI.Pack("getEETHByWeETH", weETHBal)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("encode conversion: %v", err)), nil
			}
			convResult, err := eth.CallContract(ctx, callMsg(WeETHAddress, convData), nil)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query conversion rate: %v", err)), nil
			}
			weETHInEETH = new(big.Int).SetBytes(convResult)
		} else {
			weETHInEETH = big.NewInt(0)
		}

		totalEETH := new(big.Int).Add(eETHBal, weETHInEETH)

		text := fmt.Sprintf(`Ether.fi Staking Balances
Address: %s

eETH Balance: %s eETH
weETH Balance: %s weETH (≈ %s eETH)

Total Staked (eETH equivalent): %s eETH`,
			addr,
			evmclient.FormatUnits(eETHBal, 18),
			evmclient.FormatUnits(weETHBal, 18),
			evmclient.FormatUnits(weETHInEETH, 18),
			evmclient.FormatUnits(totalEETH, 18),
		)

		return mcp.NewToolResultText(text), nil
	}
}

func handleGetRates(ethClient *evmclient.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		eth := ethClient.ETH()
		oneToken := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

		// Exchange rate: 1 weETH → eETH
		convData, err := weETHABI.Pack("getEETHByWeETH", oneToken)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode getEETHByWeETH: %v", err)), nil
		}
		convResult, err := eth.CallContract(ctx, callMsg(WeETHAddress, convData), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query exchange rate: %v", err)), nil
		}
		rateWei := new(big.Int).SetBytes(convResult)

		// Total pooled ETH
		totalData, err := liquidityPoolABI.Pack("getTotalPooledEther")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode getTotalPooledEther: %v", err)), nil
		}
		totalResult, err := eth.CallContract(ctx, callMsg(LiquidityPoolAddress, totalData), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query total pooled: %v", err)), nil
		}
		totalPooled := new(big.Int).SetBytes(totalResult)

		text := fmt.Sprintf(`Ether.fi Exchange Rates

weETH → eETH Rate: 1 weETH = %s eETH
Total Pooled ETH: %s ETH`,
			evmclient.FormatUnits(rateWei, 18),
			evmclient.FormatUnits(totalPooled, 18),
		)

		return mcp.NewToolResultText(text), nil
	}
}

// --- Helpers ---

func extractParams(ctx context.Context, req mcp.CallToolRequest, store *vault.Store) (amount, addr string, err error) {
	amount, err = req.RequireString("amount")
	if err != nil {
		return "", "", fmt.Errorf("missing amount")
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
