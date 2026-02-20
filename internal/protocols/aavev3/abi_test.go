package aavev3

import (
	"math/big"
	"testing"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

func TestEncodeDecodeUserAccountData(t *testing.T) {
	// Build a mock response: 6 x uint256.
	data := make([]byte, 192)
	// totalCollateralBase = 1000_00000000 (1000 USD in 8 decimals)
	collateral := big.NewInt(100_000_000_000)
	copy(data[32-len(collateral.Bytes()):32], collateral.Bytes())
	// totalDebtBase = 500_00000000
	debt := big.NewInt(50_000_000_000)
	copy(data[64-len(debt.Bytes()):64], debt.Bytes())
	// availableBorrowsBase = 300_00000000
	avail := big.NewInt(30_000_000_000)
	copy(data[96-len(avail.Bytes()):96], avail.Bytes())
	// liquidationThreshold = 8000 (80%)
	liqThresh := big.NewInt(8000)
	copy(data[128-len(liqThresh.Bytes()):128], liqThresh.Bytes())
	// ltv = 7500 (75%)
	ltv := big.NewInt(7500)
	copy(data[160-len(ltv.Bytes()):160], ltv.Bytes())
	// healthFactor = 2e18 (2.0)
	hf := new(big.Int).Mul(big.NewInt(2), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	copy(data[192-len(hf.Bytes()):192], hf.Bytes())

	acct, err := DecodeUserAccountData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if acct.TotalCollateralBase.Cmp(collateral) != 0 {
		t.Errorf("TotalCollateralBase = %s, want %s", acct.TotalCollateralBase, collateral)
	}
	if acct.TotalDebtBase.Cmp(debt) != 0 {
		t.Errorf("TotalDebtBase = %s, want %s", acct.TotalDebtBase, debt)
	}
	if acct.AvailableBorrowsBase.Cmp(avail) != 0 {
		t.Errorf("AvailableBorrowsBase = %s, want %s", acct.AvailableBorrowsBase, avail)
	}
	if acct.CurrentLiquidationThreshold.Cmp(liqThresh) != 0 {
		t.Errorf("LiquidationThreshold = %s, want %s", acct.CurrentLiquidationThreshold, liqThresh)
	}
	if acct.LTV.Cmp(ltv) != 0 {
		t.Errorf("LTV = %s, want %s", acct.LTV, ltv)
	}
	if acct.HealthFactor.Cmp(hf) != 0 {
		t.Errorf("HealthFactor = %s, want %s", acct.HealthFactor, hf)
	}
}

func TestDecodeUserAccountDataTooShort(t *testing.T) {
	_, err := DecodeUserAccountData(make([]byte, 100))
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestEncodeDecodeReserveData(t *testing.T) {
	// Build mock: need at least 7 * 32 = 224 bytes.
	// liquidityRate at index 5 (offset 160), variableBorrowRate at index 6 (offset 192).
	data := make([]byte, 256) // extra padding
	// liquidityRate = 3.5e25 (3.5% supply APY)
	liqRate := new(big.Int).Mul(big.NewInt(35), new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil))
	copy(data[192-len(liqRate.Bytes()):192], liqRate.Bytes())
	// variableBorrowRate = 5e25 (5% borrow APY)
	varRate := new(big.Int).Mul(big.NewInt(5), new(big.Int).Exp(big.NewInt(10), big.NewInt(25), nil))
	copy(data[224-len(varRate.Bytes()):224], varRate.Bytes())

	rd, err := DecodeReserveData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rd.LiquidityRate.Cmp(liqRate) != 0 {
		t.Errorf("LiquidityRate = %s, want %s", rd.LiquidityRate, liqRate)
	}
	if rd.VariableBorrowRate.Cmp(varRate) != 0 {
		t.Errorf("VariableBorrowRate = %s, want %s", rd.VariableBorrowRate, varRate)
	}
}

func TestDecodeReserveConfigData(t *testing.T) {
	data := make([]byte, 288) // 9 * 32

	// decimals = 6
	data[31] = 6
	// ltv = 8000
	ltvBytes := big.NewInt(8000).Bytes()
	copy(data[64-len(ltvBytes):64], ltvBytes)
	// liquidationThreshold = 8500
	liqBytes := big.NewInt(8500).Bytes()
	copy(data[96-len(liqBytes):96], liqBytes)
	// liquidationBonus = 10500
	bonusBytes := big.NewInt(10500).Bytes()
	copy(data[128-len(bonusBytes):128], bonusBytes)
	// reserveFactor = 1000
	rfBytes := big.NewInt(1000).Bytes()
	copy(data[160-len(rfBytes):160], rfBytes)
	// usageAsCollateralEnabled = true
	data[191] = 1
	// borrowingEnabled = true
	data[223] = 1
	// isActive = true
	data[255] = 1
	// isFrozen = false (already zero)

	cfg, err := DecodeReserveConfigData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Decimals.Int64() != 6 {
		t.Errorf("Decimals = %d, want 6", cfg.Decimals.Int64())
	}
	if cfg.LTV.Int64() != 8000 {
		t.Errorf("LTV = %d, want 8000", cfg.LTV.Int64())
	}
	if cfg.LiquidationThreshold.Int64() != 8500 {
		t.Errorf("LiquidationThreshold = %d, want 8500", cfg.LiquidationThreshold.Int64())
	}
	if !cfg.UsageAsCollateralEnabled {
		t.Error("UsageAsCollateralEnabled should be true")
	}
	if !cfg.BorrowingEnabled {
		t.Error("BorrowingEnabled should be true")
	}
	if !cfg.IsActive {
		t.Error("IsActive should be true")
	}
	if cfg.IsFrozen {
		t.Error("IsFrozen should be false")
	}
}

func TestRayToAPY(t *testing.T) {
	tests := []struct {
		name string
		ray  *big.Int
		want float64
	}{
		{
			name: "zero",
			ray:  big.NewInt(0),
			want: 0,
		},
		{
			name: "3.5%",
			ray:  new(big.Int).Mul(big.NewInt(35), new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)),
			want: 3.5,
		},
		{
			name: "100% = 1e27",
			ray:  new(big.Int).Exp(big.NewInt(10), big.NewInt(27), nil),
			want: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RayToAPY(tt.ray)
			if got != tt.want {
				t.Errorf("RayToAPY = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		decimals int
		want     *big.Int
		wantErr  bool
	}{
		{
			name:     "integer",
			input:    "100",
			decimals: 6,
			want:     big.NewInt(100_000_000),
		},
		{
			name:     "decimal",
			input:    "0.5",
			decimals: 6,
			want:     big.NewInt(500_000),
		},
		{
			name:     "fractional with more precision",
			input:    "100.123456",
			decimals: 6,
			want:     big.NewInt(100_123_456),
		},
		{
			name:     "max",
			input:    "max",
			decimals: 18,
			want:     MaxUint256,
		},
		{
			name:     "MAX case insensitive",
			input:    "MAX",
			decimals: 6,
			want:     MaxUint256,
		},
		{
			name:     "18 decimals",
			input:    "1.5",
			decimals: 18,
			want:     new(big.Int).Mul(big.NewInt(15), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil)),
		},
		{
			name:     "truncates excess precision",
			input:    "1.1234567",
			decimals: 6,
			want:     big.NewInt(1_123_456),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAmount(tt.input, tt.decimals)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Cmp(tt.want) != 0 {
				t.Errorf("ParseAmount(%q, %d) = %s, want %s", tt.input, tt.decimals, got, tt.want)
			}
		})
	}
}

func TestABISelectors(t *testing.T) {
	// Verify selectors are the expected 4-byte values.
	tests := []struct {
		name     string
		selector []byte
		wantHex  string
	}{
		{"supply", SelectorSupply, "617ba037"},
		{"withdraw", SelectorWithdraw, "69328dec"},
		{"borrow", SelectorBorrow, "a415bcad"},
		{"repay", SelectorRepay, "573ade81"},
		{"getUserAccountData", SelectorGetUserAccountData, "bf92857c"},
		{"getReserveData", SelectorGetReserveData, "35ea6a75"},
		{"getReserveConfigurationData", SelectorGetReserveConfigData, "3e150141"},
		{"approve", SelectorApprove, "095ea7b3"},
		{"decimals", SelectorDecimals, "313ce567"},
		{"symbol", SelectorSymbol, "95d89b41"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ethcommon.Bytes2Hex(tt.selector)
			if got != tt.wantHex {
				t.Errorf("selector = %s, want %s", got, tt.wantHex)
			}
		})
	}
}

func TestEncodeApproveLength(t *testing.T) {
	data := EncodeApprove(ethcommon.HexToAddress("0x1234"), big.NewInt(1000))
	if len(data) != 68 { // 4 + 32 + 32
		t.Errorf("approve calldata length = %d, want 68", len(data))
	}
}

func TestEncodeSupplyLength(t *testing.T) {
	data := EncodeSupply(
		ethcommon.HexToAddress("0x1234"),
		big.NewInt(1000),
		ethcommon.HexToAddress("0x5678"),
	)
	if len(data) != 132 { // 4 + 32*4
		t.Errorf("supply calldata length = %d, want 132", len(data))
	}
}

func TestEncodeWithdrawLength(t *testing.T) {
	data := EncodeWithdraw(
		ethcommon.HexToAddress("0x1234"),
		big.NewInt(1000),
		ethcommon.HexToAddress("0x5678"),
	)
	if len(data) != 100 { // 4 + 32*3
		t.Errorf("withdraw calldata length = %d, want 100", len(data))
	}
}

func TestEncodeBorrowLength(t *testing.T) {
	data := EncodeBorrow(
		ethcommon.HexToAddress("0x1234"),
		big.NewInt(1000),
		ethcommon.HexToAddress("0x5678"),
	)
	if len(data) != 164 { // 4 + 32*5
		t.Errorf("borrow calldata length = %d, want 164", len(data))
	}
}

func TestEncodeRepayLength(t *testing.T) {
	data := EncodeRepay(
		ethcommon.HexToAddress("0x1234"),
		big.NewInt(1000),
		ethcommon.HexToAddress("0x5678"),
	)
	if len(data) != 132 { // 4 + 32*4
		t.Errorf("repay calldata length = %d, want 132", len(data))
	}
}
