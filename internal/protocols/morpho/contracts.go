package morpho

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

var (
	ErrInvalidAmount   = errors.New("invalid amount: must be a positive number")
	ErrInvalidMarketID = errors.New("invalid market_id: must be 0x-prefixed 64 hex chars")
)

// Ethereum mainnet — Morpho Blue singleton contract.
var MorphoAddress = common.HexToAddress("0xBBBBBbbBBb9cC5e90e3b3Af64bdAF62C37EEFFCb")

// ABI definitions for Morpho Blue contract.
var (
	// MarketParams is a tuple: (address loanToken, address collateralToken, address oracle, address irm, uint256 lltv)
	morphoABI = mustParseABI(`[
		{"name":"supply","type":"function","inputs":[{"name":"marketParams","type":"tuple","components":[{"name":"loanToken","type":"address"},{"name":"collateralToken","type":"address"},{"name":"oracle","type":"address"},{"name":"irm","type":"address"},{"name":"lltv","type":"uint256"}]},{"name":"assets","type":"uint256"},{"name":"shares","type":"uint256"},{"name":"onBehalf","type":"address"},{"name":"data","type":"bytes"}],"outputs":[{"name":"assetsSupplied","type":"uint256"},{"name":"sharesSupplied","type":"uint256"}]},
		{"name":"withdraw","type":"function","inputs":[{"name":"marketParams","type":"tuple","components":[{"name":"loanToken","type":"address"},{"name":"collateralToken","type":"address"},{"name":"oracle","type":"address"},{"name":"irm","type":"address"},{"name":"lltv","type":"uint256"}]},{"name":"assets","type":"uint256"},{"name":"shares","type":"uint256"},{"name":"onBehalf","type":"address"},{"name":"receiver","type":"address"}],"outputs":[{"name":"assetsWithdrawn","type":"uint256"},{"name":"sharesWithdrawn","type":"uint256"}]},
		{"name":"borrow","type":"function","inputs":[{"name":"marketParams","type":"tuple","components":[{"name":"loanToken","type":"address"},{"name":"collateralToken","type":"address"},{"name":"oracle","type":"address"},{"name":"irm","type":"address"},{"name":"lltv","type":"uint256"}]},{"name":"assets","type":"uint256"},{"name":"shares","type":"uint256"},{"name":"onBehalf","type":"address"},{"name":"receiver","type":"address"}],"outputs":[{"name":"assetsBorrowed","type":"uint256"},{"name":"sharesBorrowed","type":"uint256"}]},
		{"name":"repay","type":"function","inputs":[{"name":"marketParams","type":"tuple","components":[{"name":"loanToken","type":"address"},{"name":"collateralToken","type":"address"},{"name":"oracle","type":"address"},{"name":"irm","type":"address"},{"name":"lltv","type":"uint256"}]},{"name":"assets","type":"uint256"},{"name":"shares","type":"uint256"},{"name":"onBehalf","type":"address"},{"name":"data","type":"bytes"}],"outputs":[{"name":"assetsRepaid","type":"uint256"},{"name":"sharesRepaid","type":"uint256"}]},
		{"name":"position","type":"function","inputs":[{"name":"id","type":"bytes32"},{"name":"user","type":"address"}],"outputs":[{"name":"supplyShares","type":"uint256"},{"name":"borrowShares","type":"uint128"},{"name":"collateral","type":"uint128"}]},
		{"name":"market","type":"function","inputs":[{"name":"id","type":"bytes32"}],"outputs":[{"name":"totalSupplyAssets","type":"uint128"},{"name":"totalSupplyShares","type":"uint128"},{"name":"totalBorrowAssets","type":"uint128"},{"name":"totalBorrowShares","type":"uint128"},{"name":"lastUpdate","type":"uint128"},{"name":"fee","type":"uint128"}]},
		{"name":"idToMarketParams","type":"function","inputs":[{"name":"id","type":"bytes32"}],"outputs":[{"name":"loanToken","type":"address"},{"name":"collateralToken","type":"address"},{"name":"oracle","type":"address"},{"name":"irm","type":"address"},{"name":"lltv","type":"uint256"}]}
	]`)

	erc20ABI = mustParseABI(`[
		{"name":"approve","type":"function","inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"outputs":[{"name":"","type":"bool"}]},
		{"name":"decimals","type":"function","inputs":[],"outputs":[{"name":"","type":"uint8"}]},
		{"name":"symbol","type":"function","inputs":[],"outputs":[{"name":"","type":"string"}]}
	]`)
)

// MarketParams for ABI encoding.
type MarketParams struct {
	LoanToken       common.Address
	CollateralToken common.Address
	Oracle          common.Address
	Irm             common.Address
	Lltv            *big.Int
}

// ParseMarketID parses a 0x-prefixed 66-char hex string into [32]byte.
func ParseMarketID(id string) ([32]byte, error) {
	var result [32]byte
	id = strings.TrimPrefix(id, "0x")
	if len(id) != 64 {
		return result, ErrInvalidMarketID
	}
	b, err := hexDecode(id)
	if err != nil {
		return result, ErrInvalidMarketID
	}
	copy(result[:], b)
	return result, nil
}

func hexDecode(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("odd length hex string")
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		hi := unhex(s[i])
		lo := unhex(s[i+1])
		if hi == 0xff || lo == 0xff {
			return nil, fmt.Errorf("invalid hex char")
		}
		b[i/2] = hi<<4 | lo
	}
	return b, nil
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0xff
	}
}

// ParseAmount converts a human-readable amount string to token units.
func ParseAmount(amount string, decimals int) (*big.Int, error) {
	decBig := big.NewInt(int64(decimals))

	if !strings.Contains(amount, ".") {
		wei := new(big.Int)
		wei.SetString(amount, 10)
		if wei.Sign() <= 0 {
			return nil, ErrInvalidAmount
		}
		exp := new(big.Int).Exp(big.NewInt(10), decBig, nil)
		return wei.Mul(wei, exp), nil
	}

	parts := strings.SplitN(amount, ".", 2)
	whole := new(big.Int)
	whole.SetString(parts[0], 10)

	fracStr := parts[1]
	if len(fracStr) > decimals {
		fracStr = fracStr[:decimals]
	}
	for len(fracStr) < decimals {
		fracStr += "0"
	}
	frac := new(big.Int)
	frac.SetString(fracStr, 10)

	exp := new(big.Int).Exp(big.NewInt(10), decBig, nil)
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
