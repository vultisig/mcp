package eigenlayer

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

var ErrInvalidAmount = errors.New("invalid amount: must be a positive number")

// Ethereum mainnet contract addresses.
var (
	StrategyManagerAddress   = common.HexToAddress("0x858646372CC42E1A627fcE94aa7A7033e7CF075A")
	DelegationManagerAddress = common.HexToAddress("0x39053D51B77DC0d36036Fc1fCc8Cb819df8EF37A")
)

// StrategyInfo describes a known EigenLayer strategy.
type StrategyInfo struct {
	Strategy common.Address
	Token    common.Address
	Symbol   string
	Decimals int
}

// Well-known strategies on Ethereum mainnet.
var Strategies = map[string]StrategyInfo{
	"stETH": {
		Strategy: common.HexToAddress("0x93c4b944D05dfe6df7645A86cd2206016c51564D"),
		Token:    common.HexToAddress("0xae7ab96520DE3A18E5e111B5EaAb095312D7fE84"),
		Symbol:   "stETH",
		Decimals: 18,
	},
	"cbETH": {
		Strategy: common.HexToAddress("0x54945180dB7943c0ed0FEE7EdaB2Bd24620256bc"),
		Token:    common.HexToAddress("0xBe9895146f7AF43049ca1c1AE358B0541Ea49704"),
		Symbol:   "cbETH",
		Decimals: 18,
	},
	"rETH": {
		Strategy: common.HexToAddress("0x1BeE69b7dFFfA4E2d53C2a2Df135C388AD25dCD2"),
		Token:    common.HexToAddress("0xae78736Cd615f374D3085123A210448E74Fc6393"),
		Symbol:   "rETH",
		Decimals: 18,
	},
}

// ABI definitions.
var (
	strategyManagerABI = mustParseABI(`[
		{"name":"depositIntoStrategy","type":"function","inputs":[{"name":"strategy","type":"address"},{"name":"token","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"shares","type":"uint256"}]},
		{"name":"stakerStrategyShares","type":"function","inputs":[{"name":"staker","type":"address"},{"name":"strategy","type":"address"}],"outputs":[{"name":"","type":"uint256"}]}
	]`)

	delegationManagerABI = mustParseABI(`[
		{"name":"delegatedTo","type":"function","inputs":[{"name":"staker","type":"address"}],"outputs":[{"name":"","type":"address"}]},
		{"name":"isDelegated","type":"function","inputs":[{"name":"staker","type":"address"}],"outputs":[{"name":"","type":"bool"}]}
	]`)

	strategyABI = mustParseABI(`[
		{"name":"sharesToUnderlyingView","type":"function","inputs":[{"name":"amountShares","type":"uint256"}],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"underlyingToSharesView","type":"function","inputs":[{"name":"amountUnderlying","type":"uint256"}],"outputs":[{"name":"","type":"uint256"}]}
	]`)

	erc20ABI = mustParseABI(`[
		{"name":"approve","type":"function","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]}
	]`)
)

// ParseAmount converts a human-readable amount string to wei (18 decimals).
func ParseAmount(amount string) (*big.Int, error) {
	if !strings.Contains(amount, ".") {
		wei := new(big.Int)
		wei.SetString(amount, 10)
		if wei.Sign() <= 0 {
			return nil, ErrInvalidAmount
		}
		exp := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
		return wei.Mul(wei, exp), nil
	}

	parts := strings.SplitN(amount, ".", 2)
	whole := new(big.Int)
	whole.SetString(parts[0], 10)

	fracStr := parts[1]
	if len(fracStr) > 18 {
		fracStr = fracStr[:18]
	}
	for len(fracStr) < 18 {
		fracStr += "0"
	}
	frac := new(big.Int)
	frac.SetString(fracStr, 10)

	exp := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	result := new(big.Int).Mul(whole, exp)
	result.Add(result, frac)

	if result.Sign() <= 0 {
		return nil, ErrInvalidAmount
	}
	return result, nil
}

func mustParseABI(jsonABI string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(jsonABI))
	if err != nil {
		panic(fmt.Sprintf("failed to parse ABI: %v", err))
	}
	return parsed
}
