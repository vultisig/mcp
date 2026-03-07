package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/blockchair"
	evmclient "github.com/vultisig/mcp/internal/evm"
	solanaclient "github.com/vultisig/mcp/internal/solana"
	xrpclient "github.com/vultisig/mcp/internal/xrp"
)

var (
	evmTxHashRE  = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)
	utxoTxHashRE = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

func newGetTxStatusTool() mcp.Tool {
	// Derive chain lists from canonical sources.
	allChains := make([]string, 0, len(evmclient.EVMChains)+len(blockchair.SupportedChains)+2)
	allChains = append(allChains, evmclient.EVMChains...)
	for c := range blockchair.SupportedChains {
		allChains = append(allChains, c)
	}
	allChains = append(allChains, "Solana", "Ripple", "XRP")
	sort.Strings(allChains)

	return mcp.NewTool("get_tx_status",
		mcp.WithDescription(
			"Check the confirmation status of a transaction by its hash. "+
				"Returns status (confirmed/pending/failed/not_found), block number, confirmations, and fee. "+
				"Supported chains: "+strings.Join(allChains, ", "),
		),
		mcp.WithString("chain",
			mcp.Description("Chain where the transaction was submitted."),
			mcp.Required(),
		),
		mcp.WithString("tx_hash",
			mcp.Description("Transaction hash (0x-prefixed for EVM, base58 for Solana, hex for UTXO/XRP)."),
			mcp.Required(),
		),
	)
}

type txStatusResult struct {
	Chain         string `json:"chain"`
	TxHash        string `json:"tx_hash"`
	Status        string `json:"status"`
	Success       bool   `json:"success"`
	BlockNumber   int64  `json:"block_number,omitempty"`
	Confirmations uint64 `json:"confirmations,omitempty"`
	Fee           string `json:"fee,omitempty"`
	From          string `json:"from,omitempty"`
	To            string `json:"to,omitempty"`
}

func handleGetTxStatus(pool *evmclient.Pool, bcClient *blockchair.Client, solClient *solanaclient.Client, xrpClient *xrpclient.Client) server.ToolHandlerFunc {
	// Build lookup sets from canonical sources at init time.
	evmChainSet := make(map[string]bool, len(evmclient.EVMChains))
	for _, c := range evmclient.EVMChains {
		evmChainSet[c] = true
	}

	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chain, err := req.RequireString("chain")
		if err != nil {
			return mcp.NewToolResultError("chain parameter is required"), nil
		}
		txHash, err := req.RequireString("tx_hash")
		if err != nil {
			return mcp.NewToolResultError("tx_hash parameter is required"), nil
		}

		var result *txStatusResult

		switch {
		case evmChainSet[chain]:
			result, err = getEVMTxStatus(ctx, pool, chain, txHash)
		case blockchair.SupportedChains[chain] != (blockchair.ChainInfo{}):
			result, err = getUTXOTxStatus(ctx, bcClient, chain, txHash)
		case chain == "Solana":
			result, err = getSolanaTxStatus(ctx, solClient, txHash)
		case chain == "Ripple" || chain == "XRP":
			result, err = getXRPTxStatus(ctx, xrpClient, txHash)
		default:
			return mcp.NewToolResultError(fmt.Sprintf("unsupported chain %q for tx status lookup", chain)), nil
		}

		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		data, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal tx status: %w", marshalErr)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

func getEVMTxStatus(ctx context.Context, pool *evmclient.Pool, chain, txHash string) (*txStatusResult, error) {
	if !evmTxHashRE.MatchString(txHash) {
		return nil, fmt.Errorf("invalid EVM transaction hash: %s (expected 0x + 64 hex chars)", txHash)
	}

	client, _, err := pool.Get(ctx, chain)
	if err != nil {
		return nil, fmt.Errorf("chain %s unavailable: %v", chain, err)
	}

	hash := ethcommon.HexToHash(txHash)
	receipt, err := client.ETH().TransactionReceipt(ctx, hash)
	if err != nil {
		if err == ethereum.NotFound {
			// Check if the tx is pending in the mempool.
			tx, isPending, txErr := client.ETH().TransactionByHash(ctx, hash)
			if txErr == nil && tx != nil && isPending {
				res := &txStatusResult{
					Chain:  chain,
					TxHash: txHash,
					Status: "pending",
				}
				if tx.To() != nil {
					res.To = tx.To().Hex()
				}
				return res, nil
			}
			return &txStatusResult{
				Chain:  chain,
				TxHash: txHash,
				Status: "not_found",
			}, nil
		}
		return nil, fmt.Errorf("failed to get receipt: %v", err)
	}

	ticker := evmclient.NativeTicker(chain)

	// Calculate confirmations (guard against underflow from reorgs).
	latestBlock, blockErr := client.ETH().BlockNumber(ctx)
	var confirmations uint64
	if blockErr == nil && receipt.BlockNumber != nil && latestBlock >= receipt.BlockNumber.Uint64() {
		confirmations = latestBlock - receipt.BlockNumber.Uint64()
	}

	// Calculate fee in human-readable units.
	var feeStr string
	if receipt.EffectiveGasPrice != nil {
		gasUsed := new(big.Int).SetUint64(receipt.GasUsed)
		feeBig := new(big.Int).Mul(gasUsed, receipt.EffectiveGasPrice)
		feeStr = evmclient.FormatUnits(feeBig, 18) + " " + ticker
	}

	status := "confirmed"
	success := receipt.Status == 1
	if !success {
		status = "failed"
	}

	result := &txStatusResult{
		Chain:         chain,
		TxHash:        txHash,
		Status:        status,
		Success:       success,
		Confirmations: confirmations,
		Fee:           feeStr,
	}

	if receipt.BlockNumber != nil {
		result.BlockNumber = receipt.BlockNumber.Int64()
	}

	// Get from/to from the transaction itself.
	tx, _, err := client.ETH().TransactionByHash(ctx, hash)
	if err == nil && tx != nil {
		if tx.To() != nil {
			result.To = tx.To().Hex()
		}
		// From requires a signer, use the receipt logs or skip for simplicity.
	}

	return result, nil
}

func getUTXOTxStatus(ctx context.Context, bcClient *blockchair.Client, chain, txHash string) (*txStatusResult, error) {
	if !utxoTxHashRE.MatchString(txHash) {
		return nil, fmt.Errorf("invalid UTXO transaction hash: %s (expected 64 hex chars)", txHash)
	}

	info, ok := blockchair.SupportedChains[chain]
	if !ok {
		return nil, fmt.Errorf("unsupported UTXO chain: %s", chain)
	}

	tx, err := bcClient.GetTxDashboard(ctx, chain, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get tx status from Blockchair: %v", err)
	}
	if tx == nil {
		return &txStatusResult{
			Chain:  chain,
			TxHash: txHash,
			Status: "not_found",
		}, nil
	}

	status := "confirmed"
	success := true
	if tx.BlockID == -1 {
		status = "pending"
		success = false
	}

	feeFormatted := blockchair.FormatSatoshis(tx.Fee, info.Decimals) + " " + info.Ticker

	res := &txStatusResult{
		Chain:   chain,
		TxHash:  txHash,
		Status:  status,
		Success: success,
		Fee:     feeFormatted,
	}
	if tx.BlockID >= 0 {
		res.BlockNumber = tx.BlockID
	}
	if tx.Confirmations > 0 {
		res.Confirmations = uint64(tx.Confirmations)
	}
	return res, nil
}

func getSolanaTxStatus(ctx context.Context, solClient *solanaclient.Client, txHash string) (*txStatusResult, error) {
	status, err := solClient.GetTransactionStatus(ctx, txHash)
	if err != nil {
		if errors.Is(err, solanaclient.ErrTxNotFound) {
			return &txStatusResult{
				Chain:  "Solana",
				TxHash: txHash,
				Status: "not_found",
			}, nil
		}
		return nil, fmt.Errorf("failed to get Solana tx status: %v", err)
	}

	result := &txStatusResult{
		Chain:   "Solana",
		TxHash:  txHash,
		Status:  status.Status,
		Success: status.Status == "confirmed" || status.Status == "finalized",
	}
	if status.Confirmations != nil {
		result.Confirmations = *status.Confirmations
	}
	if status.Slot > 0 {
		result.BlockNumber = int64(status.Slot)
	}
	return result, nil
}

func getXRPTxStatus(ctx context.Context, xrpClient *xrpclient.Client, txHash string) (*txStatusResult, error) {
	status, err := xrpClient.GetTransactionStatus(ctx, txHash)
	if err != nil {
		if errors.Is(err, xrpclient.ErrTxNotFound) {
			return &txStatusResult{
				Chain:  "Ripple",
				TxHash: txHash,
				Status: "not_found",
			}, nil
		}
		return nil, fmt.Errorf("failed to get XRP tx status: %v", err)
	}

	txStatus := "confirmed"
	success := status.Result == "tesSUCCESS"
	if !success {
		txStatus = "failed"
	}
	if !status.Validated {
		txStatus = "pending"
	}

	result := &txStatusResult{
		Chain:       "Ripple",
		TxHash:      txHash,
		Status:      txStatus,
		Success:     success,
		BlockNumber: status.Ledger,
	}
	if status.Fee != "" {
		result.Fee = status.Fee + " drops"
	}
	return result, nil
}
