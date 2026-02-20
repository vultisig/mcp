package aavev3

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

// Function selectors (first 4 bytes of keccak256 of the function signature).
var (
	// Pool contract
	SelectorSupply             = ethcommon.Hex2Bytes("617ba037") // supply(address,uint256,address,uint16)
	SelectorWithdraw           = ethcommon.Hex2Bytes("69328dec") // withdraw(address,uint256,address)
	SelectorBorrow             = ethcommon.Hex2Bytes("a415bcad") // borrow(address,uint256,uint256,uint16,address)
	SelectorRepay              = ethcommon.Hex2Bytes("573ade81") // repay(address,uint256,uint256,address)
	SelectorGetUserAccountData = ethcommon.Hex2Bytes("bf92857c") // getUserAccountData(address)

	// DataProvider contract
	SelectorGetReserveData       = ethcommon.Hex2Bytes("35ea6a75") // getReserveData(address)
	SelectorGetReserveConfigData = ethcommon.Hex2Bytes("3e150141") // getReserveConfigurationData(address)

	// ERC-20
	SelectorApprove  = ethcommon.Hex2Bytes("095ea7b3") // approve(address,uint256)
	SelectorDecimals = ethcommon.Hex2Bytes("313ce567") // decimals()
	SelectorSymbol   = ethcommon.Hex2Bytes("95d89b41") // symbol()
)

// MaxUint256 is type(uint256).max, used for "max" withdrawals/repayments.
var MaxUint256 = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

// UserAccountData holds decoded output from getUserAccountData().
type UserAccountData struct {
	TotalCollateralBase    *big.Int // 8-decimal USD
	TotalDebtBase          *big.Int // 8-decimal USD
	AvailableBorrowsBase   *big.Int // 8-decimal USD
	CurrentLiquidationThreshold *big.Int // bps (10000 = 100%)
	LTV                    *big.Int // bps
	HealthFactor           *big.Int // 18-decimal WAD (1e18 = 1.0)
}

// ReserveData holds decoded output from getReserveData() (partial — we only need rates).
type ReserveData struct {
	LiquidityRate     *big.Int // RAY (1e27 = 100%)
	VariableBorrowRate *big.Int // RAY
}

// ReserveConfigData holds decoded output from getReserveConfigurationData().
type ReserveConfigData struct {
	Decimals               *big.Int
	LTV                    *big.Int // bps
	LiquidationThreshold   *big.Int // bps
	LiquidationBonus       *big.Int // bps (e.g. 10500 = 5% bonus)
	ReserveFactor          *big.Int // bps
	UsageAsCollateralEnabled bool
	BorrowingEnabled       bool
	IsActive               bool
	IsFrozen               bool
}

// --- Encoding helpers ---

func padAddress(addr ethcommon.Address) []byte {
	b := make([]byte, 32)
	copy(b[12:], addr.Bytes())
	return b
}

func padUint256(v *big.Int) []byte {
	b := make([]byte, 32)
	if v != nil {
		vBytes := v.Bytes()
		copy(b[32-len(vBytes):], vBytes)
	}
	return b
}

// EncodeApprove encodes approve(spender, amount).
func EncodeApprove(spender ethcommon.Address, amount *big.Int) []byte {
	data := make([]byte, 4+64)
	copy(data, SelectorApprove)
	copy(data[4:], padAddress(spender))
	copy(data[36:], padUint256(amount))
	return data
}

// EncodeSupply encodes supply(asset, amount, onBehalfOf, referralCode=0).
func EncodeSupply(asset ethcommon.Address, amount *big.Int, onBehalfOf ethcommon.Address) []byte {
	data := make([]byte, 4+128)
	copy(data, SelectorSupply)
	copy(data[4:], padAddress(asset))
	copy(data[36:], padUint256(amount))
	copy(data[68:], padAddress(onBehalfOf))
	// referralCode = 0 (last 32 bytes already zero)
	return data
}

// EncodeWithdraw encodes withdraw(asset, amount, to).
func EncodeWithdraw(asset ethcommon.Address, amount *big.Int, to ethcommon.Address) []byte {
	data := make([]byte, 4+96)
	copy(data, SelectorWithdraw)
	copy(data[4:], padAddress(asset))
	copy(data[36:], padUint256(amount))
	copy(data[68:], padAddress(to))
	return data
}

// EncodeBorrow encodes borrow(asset, amount, interestRateMode=2, referralCode=0, onBehalfOf).
func EncodeBorrow(asset ethcommon.Address, amount *big.Int, onBehalfOf ethcommon.Address) []byte {
	data := make([]byte, 4+160)
	copy(data, SelectorBorrow)
	copy(data[4:], padAddress(asset))
	copy(data[36:], padUint256(amount))
	copy(data[68:], padUint256(big.NewInt(2))) // variable rate
	// referralCode = 0 (bytes 100-131 already zero)
	copy(data[132:], padAddress(onBehalfOf))
	return data
}

// EncodeRepay encodes repay(asset, amount, interestRateMode=2, onBehalfOf).
func EncodeRepay(asset ethcommon.Address, amount *big.Int, onBehalfOf ethcommon.Address) []byte {
	data := make([]byte, 4+128)
	copy(data, SelectorRepay)
	copy(data[4:], padAddress(asset))
	copy(data[36:], padUint256(amount))
	copy(data[68:], padUint256(big.NewInt(2))) // variable rate
	copy(data[100:], padAddress(onBehalfOf))
	return data
}

// EncodeGetUserAccountData encodes getUserAccountData(user).
func EncodeGetUserAccountData(user ethcommon.Address) []byte {
	data := make([]byte, 4+32)
	copy(data, SelectorGetUserAccountData)
	copy(data[4:], padAddress(user))
	return data
}

// EncodeGetReserveData encodes getReserveData(asset).
func EncodeGetReserveData(asset ethcommon.Address) []byte {
	data := make([]byte, 4+32)
	copy(data, SelectorGetReserveData)
	copy(data[4:], padAddress(asset))
	return data
}

// EncodeGetReserveConfigData encodes getReserveConfigurationData(asset).
func EncodeGetReserveConfigData(asset ethcommon.Address) []byte {
	data := make([]byte, 4+32)
	copy(data, SelectorGetReserveConfigData)
	copy(data[4:], padAddress(asset))
	return data
}

// --- Decoding helpers ---

// DecodeUserAccountData decodes the return value of getUserAccountData().
// Returns 6 x uint256: totalCollateralBase, totalDebtBase, availableBorrowsBase,
// currentLiquidationThreshold, ltv, healthFactor.
func DecodeUserAccountData(data []byte) (*UserAccountData, error) {
	if len(data) < 192 { // 6 * 32
		return nil, fmt.Errorf("getUserAccountData response too short: %d bytes", len(data))
	}
	return &UserAccountData{
		TotalCollateralBase:    new(big.Int).SetBytes(data[0:32]),
		TotalDebtBase:          new(big.Int).SetBytes(data[32:64]),
		AvailableBorrowsBase:   new(big.Int).SetBytes(data[64:96]),
		CurrentLiquidationThreshold: new(big.Int).SetBytes(data[96:128]),
		LTV:                    new(big.Int).SetBytes(data[128:160]),
		HealthFactor:           new(big.Int).SetBytes(data[160:192]),
	}, nil
}

// DecodeReserveData decodes the return value of getReserveData() from the
// DataProvider. The full return has many fields; we extract the ones we need:
//
//	(uint256 unbacked, uint256 accruedToTreasuryScaled, uint256 totalAToken,
//	 uint256 totalStableDebt, uint256 totalVariableDebt, uint256 liquidityRate,
//	 uint256 variableBorrowRate, uint256 stableBorrowRate, ...)
//
// liquidityRate is at index 5 (offset 160), variableBorrowRate at index 6 (offset 192).
func DecodeReserveData(data []byte) (*ReserveData, error) {
	if len(data) < 224 { // need at least 7 * 32
		return nil, fmt.Errorf("getReserveData response too short: %d bytes", len(data))
	}
	return &ReserveData{
		LiquidityRate:      new(big.Int).SetBytes(data[160:192]),
		VariableBorrowRate: new(big.Int).SetBytes(data[192:224]),
	}, nil
}

// DecodeReserveConfigData decodes getReserveConfigurationData() return:
//
//	(uint256 decimals, uint256 ltv, uint256 liquidationThreshold,
//	 uint256 liquidationBonus, uint256 reserveFactor,
//	 bool usageAsCollateralEnabled, bool borrowingEnabled,
//	 bool isActive, bool isFrozen)
func DecodeReserveConfigData(data []byte) (*ReserveConfigData, error) {
	if len(data) < 288 { // 9 * 32
		return nil, fmt.Errorf("getReserveConfigurationData response too short: %d bytes", len(data))
	}
	return &ReserveConfigData{
		Decimals:               new(big.Int).SetBytes(data[0:32]),
		LTV:                    new(big.Int).SetBytes(data[32:64]),
		LiquidationThreshold:   new(big.Int).SetBytes(data[64:96]),
		LiquidationBonus:       new(big.Int).SetBytes(data[96:128]),
		ReserveFactor:          new(big.Int).SetBytes(data[128:160]),
		UsageAsCollateralEnabled: new(big.Int).SetBytes(data[160:192]).Sign() != 0,
		BorrowingEnabled:       new(big.Int).SetBytes(data[192:224]).Sign() != 0,
		IsActive:               new(big.Int).SetBytes(data[224:256]).Sign() != 0,
		IsFrozen:               new(big.Int).SetBytes(data[256:288]).Sign() != 0,
	}, nil
}

// --- Utility functions ---

// RayToAPY converts a RAY value (1e27 = 100%) to a percentage float.
// E.g., 3.5e25 → 3.5%.
func RayToAPY(ray *big.Int) float64 {
	// ray / 1e25 gives percentage
	f := new(big.Float).SetInt(ray)
	divisor := new(big.Float).SetFloat64(1e25)
	pct, _ := new(big.Float).Quo(f, divisor).Float64()
	return math.Round(pct*100) / 100 // round to 2 decimal places
}

// ParseAmount parses a human-readable amount string (e.g. "100.5" or "max")
// into a *big.Int in the token's smallest unit.
func ParseAmount(s string, decimals int) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "max") {
		return new(big.Int).Set(MaxUint256), nil
	}

	// Split on decimal point.
	parts := strings.SplitN(s, ".", 2)
	if len(parts) == 0 || parts[0] == "" && (len(parts) < 2 || parts[1] == "") {
		return nil, fmt.Errorf("invalid amount: %q", s)
	}

	wholePart := parts[0]
	if wholePart == "" {
		wholePart = "0"
	}

	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}

	// Truncate or pad fractional part to match decimals.
	if len(fracPart) > decimals {
		fracPart = fracPart[:decimals]
	}
	for len(fracPart) < decimals {
		fracPart += "0"
	}

	combined := wholePart + fracPart
	// Remove leading zeros but keep at least "0".
	combined = strings.TrimLeft(combined, "0")
	if combined == "" {
		combined = "0"
	}

	result, ok := new(big.Int).SetString(combined, 10)
	if !ok {
		return nil, fmt.Errorf("invalid amount: %q", s)
	}

	return result, nil
}
