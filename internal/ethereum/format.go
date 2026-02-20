package ethereum

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// FormatUnits formats a wei-denominated value into a decimal string with
// the given number of decimals (e.g. 18 for ETH, 6 for USDC).
func FormatUnits(wei *big.Int, decimals int) string {
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

// DecodeABIString decodes an ABI-encoded string from contract return data.
// Handles both standard ABI encoding (offset+length+data) and non-standard
// encodings (e.g. bytes32 left-padded strings like MKR).
func DecodeABIString(data []byte) (string, error) {
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
