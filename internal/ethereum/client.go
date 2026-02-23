package ethereum

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
		return nil, fmt.Errorf("dial ethereum rpc: %w", err)
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

// ChainID returns the chain ID of the connected network.
func (c *Client) ChainID(ctx context.Context) (*big.Int, error) {
	return c.eth.ChainID(ctx)
}

// CallContract executes a read-only contract call.
func (c *Client) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	return c.eth.CallContract(ctx, msg, block)
}
