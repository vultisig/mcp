package evm

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/vultisig/recipes/sdk/evm/codegen/erc20"
)

var erc20Codec = erc20.NewErc20()

// TokenBalance holds the result of a token balance query.
type TokenBalance struct {
	Balance  string
	Symbol   string
	Decimals uint8
}

// Client wraps an Ethereum JSON-RPC client.
type Client struct {
	eth    *ethclient.Client
	rawRPC *rpc.Client
}

func NewClient(rpcURL string) (*Client, error) {
	eth, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial evm rpc: %w", err)
	}
	return &Client{eth: eth, rawRPC: eth.Client()}, nil
}

func (c *Client) Close() {
	c.eth.Close()
}

// ETH returns the underlying ethclient for use by the EVM SDK.
func (c *Client) ETH() *ethclient.Client {
	return c.eth
}

// RawRPC returns the underlying rpc.Client for use by the EVM SDK.
func (c *Client) RawRPC() *rpc.Client {
	return c.rawRPC
}

// GetNativeBalance returns the native coin balance formatted in human-readable units.
func (c *Client) GetNativeBalance(ctx context.Context, addr string) (string, error) {
	address := ethcommon.HexToAddress(addr)
	balance, err := c.eth.BalanceAt(ctx, address, nil)
	if err != nil {
		return "", fmt.Errorf("get native balance: %w", err)
	}
	return FormatUnits(balance, 18), nil
}

// GetTokenBalance returns the ERC-20 token balance, symbol, and decimals.
func (c *Client) GetTokenBalance(ctx context.Context, tokenAddr, holderAddr string) (*TokenBalance, error) {
	token := ethcommon.HexToAddress(tokenAddr)
	holder := ethcommon.HexToAddress(holderAddr)

	decimalsData, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: erc20Codec.PackDecimals(),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call decimals(): %w", err)
	}
	decimals, err := erc20Codec.UnpackDecimals(decimalsData)
	if err != nil {
		return nil, fmt.Errorf("decode decimals: %w", err)
	}

	symbolData, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: erc20Codec.PackSymbol(),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call symbol(): %w", err)
	}
	symbol, err := DecodeABIString(symbolData)
	if err != nil {
		return nil, fmt.Errorf("decode symbol: %w", err)
	}

	balanceData, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: erc20Codec.PackBalanceOf(holder),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call balanceOf(): %w", err)
	}
	balance, err := erc20Codec.UnpackBalanceOf(balanceData)
	if err != nil {
		return nil, fmt.Errorf("decode balanceOf: %w", err)
	}

	return &TokenBalance{
		Balance:  FormatUnits(balance, int(decimals)),
		Symbol:   symbol,
		Decimals: decimals,
	}, nil
}

// GetAllowance returns the ERC-20 token allowance for a spender.
func (c *Client) GetAllowance(ctx context.Context, tokenAddr, ownerAddr, spenderAddr string) (*big.Int, uint8, string, error) {
	token := ethcommon.HexToAddress(tokenAddr)
	owner := ethcommon.HexToAddress(ownerAddr)
	spender := ethcommon.HexToAddress(spenderAddr)

	allowanceData, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: erc20Codec.PackAllowance(owner, spender),
	}, nil)
	if err != nil {
		return nil, 0, "", fmt.Errorf("call allowance(): %w", err)
	}
	allowance, err := erc20Codec.UnpackAllowance(allowanceData)
	if err != nil {
		return nil, 0, "", fmt.Errorf("decode allowance: %w", err)
	}

	decimalsData, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: erc20Codec.PackDecimals(),
	}, nil)
	if err != nil {
		return nil, 0, "", fmt.Errorf("call decimals(): %w", err)
	}
	decimals, err := erc20Codec.UnpackDecimals(decimalsData)
	if err != nil {
		return nil, 0, "", fmt.Errorf("decode decimals: %w", err)
	}

	symbolData, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &token,
		Data: erc20Codec.PackSymbol(),
	}, nil)
	if err != nil {
		return nil, 0, "", fmt.Errorf("call symbol(): %w", err)
	}
	symbol, err := DecodeABIString(symbolData)
	if err != nil {
		return nil, 0, "", fmt.Errorf("decode symbol: %w", err)
	}

	return allowance, decimals, symbol, nil
}

// ChainID returns the chain ID of the connected network.
func (c *Client) ChainID(ctx context.Context) (*big.Int, error) {
	return c.eth.ChainID(ctx)
}

// CallContract executes a read-only contract call.
func (c *Client) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	return c.eth.CallContract(ctx, msg, block)
}

// PendingNonce returns the next nonce for the given address (pending state).
func (c *Client) PendingNonce(ctx context.Context, addr ethcommon.Address) (uint64, error) {
	return c.eth.PendingNonceAt(ctx, addr)
}

// SuggestGasTipCap returns the suggested gas tip cap (maxPriorityFeePerGas).
func (c *Client) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return c.eth.SuggestGasTipCap(ctx)
}

// LatestBaseFee returns the base fee from the latest block header.
func (c *Client) LatestBaseFee(ctx context.Context) (*big.Int, error) {
	header, err := c.eth.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("get latest header: %w", err)
	}
	if header.BaseFee == nil {
		return nil, fmt.Errorf("chain does not support EIP-1559 (no base fee)")
	}
	return header.BaseFee, nil
}

// EstimateGas estimates the gas needed for the given call message.
func (c *Client) EstimateGas(ctx context.Context, msg ethereum.CallMsg) (uint64, error) {
	return c.eth.EstimateGas(ctx, msg)
}

// IsApprovedForAll checks ERC-1155 isApprovedForAll(owner, operator).
func (c *Client) IsApprovedForAll(ctx context.Context, contractAddr, ownerAddr, operatorAddr string) (bool, error) {
	contract := ethcommon.HexToAddress(contractAddr)
	owner := ethcommon.HexToAddress(ownerAddr)
	operator := ethcommon.HexToAddress(operatorAddr)

	// isApprovedForAll(address,address) selector = 0xe985e9c5
	data := make([]byte, 4+32+32)
	data[0] = 0xe9
	data[1] = 0x85
	data[2] = 0xe9
	data[3] = 0xc5
	copy(data[4+12:4+32], owner.Bytes())
	copy(data[4+32+12:4+64], operator.Bytes())

	result, err := c.eth.CallContract(ctx, ethereum.CallMsg{
		To:   &contract,
		Data: data,
	}, nil)
	if err != nil {
		return false, fmt.Errorf("call isApprovedForAll(): %w", err)
	}
	if len(result) < 32 {
		return false, fmt.Errorf("unexpected isApprovedForAll result length: %d", len(result))
	}
	return new(big.Int).SetBytes(result).Sign() != 0, nil
}
