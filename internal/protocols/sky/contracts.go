package sky

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
	// Legacy MakerDAO savings
	DAIAddress  = common.HexToAddress("0x6B175474E89094C44Da98b954EedeAC495271d0F")
	SDAIAddress = common.HexToAddress("0x83F20F44975D03b1b09e64809B757c47f942BEeA")

	// New Sky savings
	USDSAddress  = common.HexToAddress("0xdC035D45d973E3EC169d2276DDab16f1e407384F")
	SUSDSAddress = common.HexToAddress("0xa3931d71877C0E7a3148CB7Eb4463524FEc27fbD")
)

// VaultConfig maps vault name to token/vault addresses.
type VaultConfig struct {
	UnderlyingAddress common.Address
	VaultAddress      common.Address
	UnderlyingSymbol  string
	VaultSymbol       string
	Decimals          int
}

var Vaults = map[string]VaultConfig{
	"sdai": {
		UnderlyingAddress: DAIAddress,
		VaultAddress:      SDAIAddress,
		UnderlyingSymbol:  "DAI",
		VaultSymbol:       "sDAI",
		Decimals:          18,
	},
	"susds": {
		UnderlyingAddress: USDSAddress,
		VaultAddress:      SUSDSAddress,
		UnderlyingSymbol:  "USDS",
		VaultSymbol:       "sUSDS",
		Decimals:          18,
	},
}

// ABI definitions — ERC-4626 + ERC-20 approve/balanceOf.
var (
	erc20ABI = mustParseABI(`[
		{"name":"approve","type":"function","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]},
		{"name":"balanceOf","type":"function","inputs":[{"name":"account","type":"address"}],"outputs":[{"name":"","type":"uint256"}]}
	]`)

	erc4626ABI = mustParseABI(`[
		{"name":"deposit","type":"function","inputs":[{"name":"assets","type":"uint256"},{"name":"receiver","type":"address"}],"outputs":[{"name":"shares","type":"uint256"}]},
		{"name":"withdraw","type":"function","inputs":[{"name":"assets","type":"uint256"},{"name":"receiver","type":"address"},{"name":"owner","type":"address"}],"outputs":[{"name":"shares","type":"uint256"}]},
		{"name":"redeem","type":"function","inputs":[{"name":"shares","type":"uint256"},{"name":"receiver","type":"address"},{"name":"owner","type":"address"}],"outputs":[{"name":"assets","type":"uint256"}]},
		{"name":"convertToAssets","type":"function","inputs":[{"name":"shares","type":"uint256"}],"outputs":[{"name":"assets","type":"uint256"}]},
		{"name":"convertToShares","type":"function","inputs":[{"name":"assets","type":"uint256"}],"outputs":[{"name":"shares","type":"uint256"}]},
		{"name":"totalAssets","type":"function","inputs":[],"outputs":[{"name":"","type":"uint256"}]},
		{"name":"balanceOf","type":"function","inputs":[{"name":"account","type":"address"}],"outputs":[{"name":"","type":"uint256"}]}
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
