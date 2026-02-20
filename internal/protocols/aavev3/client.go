package aavev3

import (
	"context"
	"fmt"
	"math/big"

	goeth "github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/vultisig/mcp/internal/ethereum"
)

// ProtocolClient wraps an ethereum.Client with Aave V3 deployment addresses.
type ProtocolClient struct {
	eth    *ethereum.Client
	deploy Deployment
}

// NewProtocolClient creates a new Aave V3 protocol client.
func NewProtocolClient(ethClient *ethereum.Client, deploy Deployment) *ProtocolClient {
	return &ProtocolClient{eth: ethClient, deploy: deploy}
}

// GetUserAccountData calls getUserAccountData(user) on the Pool contract.
func (c *ProtocolClient) GetUserAccountData(ctx context.Context, user ethcommon.Address) (*UserAccountData, error) {
	pool := c.deploy.Pool
	data, err := c.eth.CallContract(ctx, goeth.CallMsg{
		To:   &pool,
		Data: EncodeGetUserAccountData(user),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call getUserAccountData: %w", err)
	}
	return DecodeUserAccountData(data)
}

// GetReserveData calls getReserveData(asset) on the DataProvider contract.
func (c *ProtocolClient) GetReserveData(ctx context.Context, asset ethcommon.Address) (*ReserveData, error) {
	dp := c.deploy.DataProvider
	data, err := c.eth.CallContract(ctx, goeth.CallMsg{
		To:   &dp,
		Data: EncodeGetReserveData(asset),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call getReserveData: %w", err)
	}
	return DecodeReserveData(data)
}

// GetReserveConfigData calls getReserveConfigurationData(asset) on the DataProvider.
func (c *ProtocolClient) GetReserveConfigData(ctx context.Context, asset ethcommon.Address) (*ReserveConfigData, error) {
	dp := c.deploy.DataProvider
	data, err := c.eth.CallContract(ctx, goeth.CallMsg{
		To:   &dp,
		Data: EncodeGetReserveConfigData(asset),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("call getReserveConfigurationData: %w", err)
	}
	return DecodeReserveConfigData(data)
}

// GetTokenDecimals queries the decimals() of an ERC-20 token.
func (c *ProtocolClient) GetTokenDecimals(ctx context.Context, token ethcommon.Address) (uint8, error) {
	data, err := c.eth.CallContract(ctx, goeth.CallMsg{
		To:   &token,
		Data: SelectorDecimals,
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("call decimals(): %w", err)
	}
	if len(data) < 32 {
		return 0, fmt.Errorf("invalid decimals response: too short")
	}
	return uint8(new(big.Int).SetBytes(data).Uint64()), nil
}

// GetTokenSymbol queries the symbol() of an ERC-20 token.
func (c *ProtocolClient) GetTokenSymbol(ctx context.Context, token ethcommon.Address) (string, error) {
	data, err := c.eth.CallContract(ctx, goeth.CallMsg{
		To:   &token,
		Data: SelectorSymbol,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("call symbol(): %w", err)
	}
	return ethereum.DecodeABIString(data)
}

// PoolAddress returns the Pool contract address.
func (c *ProtocolClient) PoolAddress() ethcommon.Address {
	return c.deploy.Pool
}
