package lido

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
	StETHAddress           = common.HexToAddress("0xae7ab96520DE3A18E5e111B5EaAb095312D7fE84")
	WstETHAddress          = common.HexToAddress("0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0")
	WithdrawalQueueAddress = common.HexToAddress("0x889edC2eDab5f40e902b864aD4d7AdE8E412F9B1")
)

// ABI definitions for contract interactions.
var (
	// stETH: submit(address _referral) payable returns (uint256)
	stETHABI = mustParseABI(`[
		{"name":"submit","type":"function","inputs":[{"name":"_referral","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"balanceOf","type":"function","inputs":[{"name":"_account","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"approve","type":"function","inputs":[{"name":"_spender","type":"address"},{"name":"_amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]},
		{"name":"decimals","type":"function","inputs":[],"outputs":[{"name":"","type":"uint8"}]}
	]`)

	// wstETH: wrap/unwrap + conversion
	wstETHABI = mustParseABI(`[
		{"name":"wrap","type":"function","inputs":[{"name":"_stETHAmount","type":"uint256"}],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"unwrap","type":"function","inputs":[{"name":"_wstETHAmount","type":"uint256"}],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"balanceOf","type":"function","inputs":[{"name":"_account","type":"address"}],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"getStETHByWstETH","type":"function","inputs":[{"name":"_wstETHAmount","type":"uint256"}],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"getWstETHByStETH","type":"function","inputs":[{"name":"_stETHAmount","type":"uint256"}],"outputs":[{"name":"","type":"uint256"}]}
	]`)

	// Withdrawal queue: requestWithdrawals
	withdrawalQueueABI = mustParseABI(`[
		{"name":"requestWithdrawals","type":"function","inputs":[{"name":"_amounts","type":"uint256[]"},{"name":"_owner","type":"address"}],"outputs":[{"name":"requestIds","type":"uint256[]"}]}
	]`)
)

// ParseAmount converts a human-readable amount string to wei (18 decimals).
func ParseAmount(amount string) (*big.Int, error) {
	// Handle integer amounts
	if !strings.Contains(amount, ".") {
		wei := new(big.Int)
		wei.SetString(amount, 10)
		if wei.Sign() <= 0 {
			return nil, ErrInvalidAmount
		}
		exp := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
		return wei.Mul(wei, exp), nil
	}

	// Handle decimal amounts
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
