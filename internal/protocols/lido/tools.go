package lido

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
	gasLimitStake      = 150_000
	gasLimitWrap       = 100_000
	gasLimitUnwrap     = 100_000
	gasLimitWithdrawal = 300_000
	gasLimitApprove    = 60_000
)

// Protocol implements the protocols.Protocol interface for Lido.
type Protocol struct{}

func (p *Protocol) Name() string { return "lido" }

func (p *Protocol) SupportsChain(chainID *big.Int) bool {
	return chainID.Int64() == 1 // Ethereum mainnet only
}

func (p *Protocol) Register(s *server.MCPServer, store *vault.Store, ethClient *evmclient.Client, evmSDK *evmsdk.SDK, chainID *big.Int) {
	toolmeta.Register(s, newStakeTool(), handleStake(store, evmSDK, ethClient, chainID), "lido")
	toolmeta.Register(s, newWrapTool(), handleWrap(store, evmSDK, ethClient, chainID), "lido")
	toolmeta.Register(s, newUnwrapTool(), handleUnwrap(store, evmSDK, ethClient, chainID), "lido")
	toolmeta.Register(s, newRequestWithdrawalTool(), handleRequestWithdrawal(store, evmSDK, ethClient, chainID), "lido")
	toolmeta.Register(s, newGetBalancesTool(), handleGetBalances(store, ethClient), "lido")
}

// --- Tool definitions ---

func newStakeTool() mcp.Tool {
	return mcp.NewTool("lido_stake",
		mcp.WithDescription("Build an unsigned transaction to stake ETH with Lido and receive stETH. Sends ETH to the Lido staking contract."),
		mcp.WithString("amount", mcp.Description("Amount of ETH to stake in human-readable units (e.g. \"1.5\")"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Staker's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newWrapTool() mcp.Tool {
	return mcp.NewTool("lido_wrap",
		mcp.WithDescription("Build unsigned transactions to wrap stETH into wstETH. Returns an approve tx and a wrap tx. wstETH is non-rebasing and better for DeFi integrations."),
		mcp.WithString("amount", mcp.Description("Amount of stETH to wrap in human-readable units (e.g. \"10.0\")"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Owner's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newUnwrapTool() mcp.Tool {
	return mcp.NewTool("lido_unwrap",
		mcp.WithDescription("Build an unsigned transaction to unwrap wstETH back into stETH. No approval needed."),
		mcp.WithString("amount", mcp.Description("Amount of wstETH to unwrap in human-readable units"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Owner's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newRequestWithdrawalTool() mcp.Tool {
	return mcp.NewTool("lido_request_withdrawal",
		mcp.WithDescription("Build unsigned transactions to request an stETH withdrawal from Lido. Returns an approve tx and a withdrawal request tx. After the request is finalized (1-5 days), ETH can be claimed."),
		mcp.WithString("amount", mcp.Description("Amount of stETH to withdraw in human-readable units"), mcp.Required()),
		mcp.WithString("address", mcp.Description("Requester's Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

func newGetBalancesTool() mcp.Tool {
	return mcp.NewTool("lido_get_balances",
		mcp.WithDescription("Query Lido staking balances: stETH balance, wstETH balance, and the equivalent stETH value of wstETH holdings."),
		mcp.WithString("address", mcp.Description("Ethereum address (0x-prefixed). Optional if vault info is set.")),
	)
}

// --- Handlers ---

func handleStake(store *vault.Store, evmSDK *evmsdk.SDK, ethClient *evmclient.Client, chainID *big.Int) server.ToolHandlerFunc {
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

		// submit(address _referral) with ETH value
		calldata, err := stETHABI.Pack("submit", ethcommon.Address{}) // zero address referral
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode submit: %v", err)), nil
		}

		unsignedTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, StETHAddress, weiAmount, calldata, 0, gasLimitStake)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build stake tx: %v", err)), nil
		}

		nonce, gasLimit, maxFee, maxPriorityFee, err := decodeTxFields(unsignedTx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode tx: %v", err)), nil
		}

		dets := txDetails(StETHAddress.Hex(), weiAmount.String(), nonce, gasLimit, maxFee, maxPriorityFee, calldata, "Stake ETH with Lido to receive stETH", "Lido stETH")
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

func handleWrap(store *vault.Store, evmSDK *evmsdk.SDK, ethClient *evmclient.Client, chainID *big.Int) server.ToolHandlerFunc {
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

		// 1. Approve stETH spending by wstETH contract
		approveData, err := stETHABI.Pack("approve", WstETHAddress, weiAmount)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode approve: %v", err)), nil
		}

		approveTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, StETHAddress, big.NewInt(0), approveData, 0, gasLimitApprove)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build approve tx: %v", err)), nil
		}

		// 2. Wrap stETH to wstETH
		wrapData, err := wstETHABI.Pack("wrap", weiAmount)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode wrap: %v", err)), nil
		}

		wrapTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, WstETHAddress, big.NewInt(0), wrapData, 1, gasLimitWrap)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build wrap tx: %v", err)), nil
		}

		return buildMultiTxResult(chainID, []unsignedTxInfo{
			{approveTx, approveData, StETHAddress, "approve", "Approve stETH spending for wstETH wrapping", "stETH Token", big.NewInt(0)},
			{wrapTx, wrapData, WstETHAddress, "wrap", "Wrap stETH into wstETH", "wstETH", big.NewInt(0)},
		}, "stETH", StETHAddress.Hex(), amountStr, weiAmount.String(), "18")
	}
}

func handleUnwrap(store *vault.Store, evmSDK *evmsdk.SDK, ethClient *evmclient.Client, chainID *big.Int) server.ToolHandlerFunc {
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

		unwrapData, err := wstETHABI.Pack("unwrap", weiAmount)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode unwrap: %v", err)), nil
		}

		unwrapTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, WstETHAddress, big.NewInt(0), unwrapData, 0, gasLimitUnwrap)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build unwrap tx: %v", err)), nil
		}

		nonce, gasLimit, maxFee, maxPriorityFee, err := decodeTxFields(unwrapTx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("decode tx: %v", err)), nil
		}

		dets := txDetails(WstETHAddress.Hex(), "0", nonce, gasLimit, maxFee, maxPriorityFee, unwrapData, "Unwrap wstETH back to stETH", "wstETH")
		dets["token_symbol"] = "wstETH"
		dets["token_address"] = WstETHAddress.Hex()
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

func handleRequestWithdrawal(store *vault.Store, evmSDK *evmsdk.SDK, ethClient *evmclient.Client, chainID *big.Int) server.ToolHandlerFunc {
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

		// 1. Approve stETH spending by withdrawal queue
		approveData, err := stETHABI.Pack("approve", WithdrawalQueueAddress, weiAmount)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode approve: %v", err)), nil
		}

		approveTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, StETHAddress, big.NewInt(0), approveData, 0, gasLimitApprove)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build approve tx: %v", err)), nil
		}

		// 2. Request withdrawal
		amounts := []*big.Int{weiAmount}
		withdrawData, err := withdrawalQueueABI.Pack("requestWithdrawals", amounts, user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode requestWithdrawals: %v", err)), nil
		}

		withdrawTx, err := evmSDK.MakeTxWithGasLimit(ctx, user, WithdrawalQueueAddress, big.NewInt(0), withdrawData, 1, gasLimitWithdrawal)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("build withdrawal tx: %v", err)), nil
		}

		return buildMultiTxResult(chainID, []unsignedTxInfo{
			{approveTx, approveData, StETHAddress, "approve", "Approve stETH for Lido withdrawal queue", "stETH Token", big.NewInt(0)},
			{withdrawTx, withdrawData, WithdrawalQueueAddress, "request_withdrawal", "Request stETH withdrawal from Lido (1-5 days)", "Lido Withdrawal Queue", big.NewInt(0)},
		}, "stETH", StETHAddress.Hex(), amountStr, weiAmount.String(), "18")
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

		// Query stETH balance
		stETHBalData, err := stETHABI.Pack("balanceOf", user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode stETH balanceOf: %v", err)), nil
		}
		stETHResult, err := eth.CallContract(ctx, callMsg(StETHAddress, stETHBalData), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query stETH balance: %v", err)), nil
		}
		stETHBal := new(big.Int).SetBytes(stETHResult)

		// Query wstETH balance
		wstETHBalData, err := wstETHABI.Pack("balanceOf", user)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encode wstETH balanceOf: %v", err)), nil
		}
		wstETHResult, err := eth.CallContract(ctx, callMsg(WstETHAddress, wstETHBalData), nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query wstETH balance: %v", err)), nil
		}
		wstETHBal := new(big.Int).SetBytes(wstETHResult)

		// Convert wstETH to stETH equivalent
		var wstETHInStETH *big.Int
		if wstETHBal.Sign() > 0 {
			convData, err := wstETHABI.Pack("getStETHByWstETH", wstETHBal)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("encode conversion: %v", err)), nil
			}
			convResult, err := eth.CallContract(ctx, callMsg(WstETHAddress, convData), nil)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("query conversion rate: %v", err)), nil
			}
			wstETHInStETH = new(big.Int).SetBytes(convResult)
		} else {
			wstETHInStETH = big.NewInt(0)
		}

		totalStETH := new(big.Int).Add(stETHBal, wstETHInStETH)

		result := fmt.Sprintf(`Lido Staking Balances
Address: %s

stETH Balance: %s stETH
wstETH Balance: %s wstETH (≈ %s stETH)

Total Staked (stETH equivalent): %s stETH`,
			addr,
			evmclient.FormatUnits(stETHBal, 18),
			evmclient.FormatUnits(wstETHBal, 18),
			evmclient.FormatUnits(wstETHInStETH, 18),
			evmclient.FormatUnits(totalStETH, 18),
		)

		return mcp.NewToolResultText(result), nil
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
