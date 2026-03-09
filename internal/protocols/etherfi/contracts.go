package etherfi

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
	EETHAddress          = common.HexToAddress("0x35fA164735182de50811E8e2E824cFb9B6118ac2")
	WeETHAddress         = common.HexToAddress("0xCd5fE23C85820F7B72D0926FC9b05b43E359b7ee")
	LiquidityPoolAddress = common.HexToAddress("0x308861A430be4cce5502d0A12724771Fc6DaF216")
)

// ABI definitions for contract interactions.
var (
	// LiquidityPool: deposit() payable returns (uint256)
	liquidityPoolABI = mustParseABI(`[
		{"name":"deposit","type":"function","inputs":[],"outputs":[{"name":"","type":"uint256"}],"stateMutability":"payable"},
		{"name":"getTotalPooledEther","type":"function","inputs":[],"outputs":[{"name":"","type":"uint256"}]}
	]`)

	// eETH: rebasing liquid staking token
	eETHABI = mustParseABI(`[
		{"name":"balanceOf","type":"function","inputs":[{"name":"_account","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"approve","type":"function","inputs":[{"name":"_spender","type":"address"},{"name":"_amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]},
		{"name":"totalShares","type":"function","inputs":[],"outputs":[{"name":"","type":"uint256"}]}
	]`)

	// weETH: non-rebasing wrapped eETH
	weETHABI = mustParseABI(`[
		{"name":"wrap","type":"function","inputs":[{"name":"_eETHAmount","type":"uint256"}],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"unwrap","type":"function","inputs":[{"name":"_weETHAmount","type":"uint256"}],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"balanceOf","type":"function","inputs":[{"name":"_account","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"getEETHByWeETH","type":"function","inputs":[{"name":"_weETHAmount","type":"uint256"}],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"getWeETHByeETH","type":"function","inputs":[{"name":"_eETHAmount","type":"uint256"}],"outputs":[{"name":"","type":"uint256"}]}
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
