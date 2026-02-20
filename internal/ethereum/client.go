package ethereum

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	ethcommon "github.com/ethereum/go-ethereum/common"
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
	return formatWei(balance, 18), nil
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
	symbol, err := decodeABIString(symbolData)
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
		Balance:  formatWei(balance, int(decimals)),
		Symbol:   symbol,
		Decimals: decimals,
	}, nil
}

// formatWei formats a wei-denominated value into a decimal string.
func formatWei(wei *big.Int, decimals int) string {
	if wei.Sign() == 0 {
		return "0"
	}

	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	whole := new(big.Int).Div(wei, divisor)
	remainder := new(big.Int).Mod(wei, divisor)

	if remainder.Sign() == 0 {
		return whole.String()
	}

	// Pad remainder with leading zeros to `decimals` width, then trim trailing zeros.
	fracStr := fmt.Sprintf("%0*s", decimals, remainder.String())
	fracStr = strings.TrimRight(fracStr, "0")

	return whole.String() + "." + fracStr
}

// decodeABIString decodes an ABI-encoded string from contract return data.
// Handles both standard ABI encoding (offset+length+data) and non-standard
// encodings (e.g. bytes32 left-padded strings like MKR).
func decodeABIString(data []byte) (string, error) {
	if len(data) < 32 {
		return "", fmt.Errorf("data too short: %d bytes", len(data))
	}

	// Try standard ABI decoding: first 32 bytes = offset, then length, then data.
	offset := new(big.Int).SetBytes(data[:32])
	if offset.Cmp(big.NewInt(int64(len(data)))) < 0 && offset.Int64() >= 32 {
		off := int(offset.Int64())
		if off+32 <= len(data) {
			length := binary.BigEndian.Uint64(data[off+24 : off+32])
			if off+32+int(length) <= len(data) {
				return string(data[off+32 : off+32+int(length)]), nil
			}
		}
	}

	// Fallback: treat as bytes32 (null-terminated or right-padded).
	s := strings.TrimRight(string(data[:32]), "\x00")
	if isPrintable(s) && len(s) > 0 {
		return s, nil
	}

	return "", fmt.Errorf("unable to decode string from: 0x%s", hex.EncodeToString(data))
}

func isPrintable(s string) bool {
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}
