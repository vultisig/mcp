package ethereum

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ABI function selectors.
var (
	selectorBalanceOf = ethcommon.Hex2Bytes("70a08231") // balanceOf(address)
	selectorDecimals  = ethcommon.Hex2Bytes("313ce567") // decimals()
	selectorSymbol    = ethcommon.Hex2Bytes("95d89b41") // symbol()
)

// TokenBalance holds the result of a token balance query.
type TokenBalance struct {
	Balance  string
	Symbol   string
	Decimals uint8
}

// Client wraps an Ethereum JSON-RPC client.
type Client struct {
	eth *ethclient.Client
}

func NewClient(rpcURL string) (*Client, error) {
	eth, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial ethereum rpc: %w", err)
	}
	return &Client{eth: eth}, nil
}

func (c *Client) Close() {
	c.eth.Close()
}

// GetETHBalance returns the native ETH balance formatted in ETH units.
func (c *Client) GetETHBalance(ctx context.Context, addr string) (string, error) {
	address := ethcommon.HexToAddress(addr)
	balance, err := c.eth.BalanceAt(ctx, address, nil)
	if err != nil {
		return "", fmt.Errorf("get eth balance: %w", err)
	}
	return FormatUnits(balance, 18), nil
}

// GetTokenBalance returns the ERC-20 token balance, symbol, and decimals.
func (c *Client) GetTokenBalance(ctx context.Context, tokenAddr, holderAddr string) (*TokenBalance, error) {
	token := ethcommon.HexToAddress(tokenAddr)
	holder := ethcommon.HexToAddress(holderAddr)

	// Query decimals.
	decimalsData, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: selectorDecimals,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call decimals(): %w", err)
	}
	if len(decimalsData) < 32 {
		return nil, fmt.Errorf("invalid decimals response: too short")
	}
	decimals := uint8(new(big.Int).SetBytes(decimalsData).Uint64())

	// Query symbol.
	symbolData, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: selectorSymbol,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call symbol(): %w", err)
	}
	symbol, err := DecodeABIString(symbolData)
	if err != nil {
		return nil, fmt.Errorf("decode symbol: %w", err)
	}

	// Query balanceOf.
	callData := make([]byte, 4+32)
	copy(callData, selectorBalanceOf)
	copy(callData[4+12:], holder.Bytes()) // left-pad address to 32 bytes

	balanceData, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: callData,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call balanceOf(): %w", err)
	}
	if len(balanceData) < 32 {
		return nil, fmt.Errorf("invalid balanceOf response: too short")
	}
	balance := new(big.Int).SetBytes(balanceData)

	return &TokenBalance{
		Balance:  FormatUnits(balance, int(decimals)),
		Symbol:   symbol,
		Decimals: decimals,
	}, nil
}

// ChainID returns the chain ID of the connected network.
func (c *Client) ChainID(ctx context.Context) (*big.Int, error) {
	return c.eth.ChainID(ctx)
}

// CallContract executes a read-only contract call.
func (c *Client) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	return c.eth.CallContract(ctx, msg, block)
}

// PendingNonceAt returns the next nonce for the account at the pending state.
func (c *Client) PendingNonceAt(ctx context.Context, account ethcommon.Address) (uint64, error) {
	return c.eth.PendingNonceAt(ctx, account)
}

// SuggestGasPrice returns the currently suggested gas price.
func (c *Client) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	return c.eth.SuggestGasPrice(ctx)
}

// EstimateGas estimates the gas needed to execute a transaction.
func (c *Client) EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	return c.eth.EstimateGas(ctx, msg)
}

// BuildUnsignedTx constructs a fully populated unsigned transaction, estimating
// gas and querying gas price from the chain. Returns the RLP-encoded hex string.
//
// If gasOverride is non-zero it is used directly as the gas limit (no
// estimation). This is needed for multi-tx flows where a later transaction
// depends on state created by an earlier one (e.g. supply after approve) —
// eth_estimateGas would revert because the approve hasn't executed yet.
func (c *Client) BuildUnsignedTx(ctx context.Context, from, to ethcommon.Address, data []byte, value *big.Int, chainID *big.Int, nonce uint64, gasOverride uint64) (string, *types.Transaction, error) {
	gasPrice, err := c.SuggestGasPrice(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("suggest gas price: %w", err)
	}

	gas := gasOverride
	if gas == 0 {
		gas, err = c.EstimateGas(ctx, ethereum.CallMsg{
			From:     from,
			To:       &to,
			Data:     data,
			Value:    value,
			GasPrice: gasPrice,
		})
		if err != nil {
			return "", nil, fmt.Errorf("estimate gas: %w", err)
		}
		// Apply 20% safety margin to estimated gas only.
		gas = gas * 120 / 100
	}

	tx := types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      gas,
		To:       &to,
		Value:    value,
		Data:     data,
	})

	rawBytes, err := tx.MarshalBinary()
	if err != nil {
		return "", nil, fmt.Errorf("marshal tx: %w", err)
	}

	return "0x" + fmt.Sprintf("%x", rawBytes), tx, nil
}

