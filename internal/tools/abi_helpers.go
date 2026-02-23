package tools

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// parseABITypes parses a comma-separated list of Solidity types into abi.Arguments.
func parseABITypes(types string) (abi.Arguments, error) {
	types = strings.TrimSpace(types)
	if types == "" {
		return nil, nil
	}
	parts := strings.Split(types, ",")
	args := make(abi.Arguments, len(parts))
	for i, p := range parts {
		p = strings.TrimSpace(p)
		t, err := abi.NewType(p, "", nil)
		if err != nil {
			return nil, fmt.Errorf("invalid ABI type %q: %w", p, err)
		}
		args[i] = abi.Argument{Type: t}
	}
	return args, nil
}

// convertStringArg converts a string argument to the Go type expected by the ABI encoder.
func convertStringArg(s string, typ abi.Type) (any, error) {
	switch typ.T {
	case abi.AddressTy:
		if !common.IsHexAddress(s) {
			return nil, fmt.Errorf("invalid address: %s", s)
		}
		return common.HexToAddress(s), nil

	case abi.UintTy:
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("invalid uint: %s", s)
		}
		if n.Sign() < 0 {
			return nil, fmt.Errorf("negative value for uint: %s", s)
		}
		if typ.Size <= 64 {
			switch typ.Size {
			case 8:
				return uint8(n.Uint64()), nil
			case 16:
				return uint16(n.Uint64()), nil
			case 32:
				return uint32(n.Uint64()), nil
			case 64:
				return n.Uint64(), nil
			}
		}
		return n, nil

	case abi.IntTy:
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			return nil, fmt.Errorf("invalid int: %s", s)
		}
		if typ.Size <= 64 {
			switch typ.Size {
			case 8:
				return int8(n.Int64()), nil
			case 16:
				return int16(n.Int64()), nil
			case 32:
				return int32(n.Int64()), nil
			case 64:
				return n.Int64(), nil
			}
		}
		return n, nil

	case abi.BoolTy:
		switch strings.ToLower(s) {
		case "true", "1":
			return true, nil
		case "false", "0":
			return false, nil
		default:
			return nil, fmt.Errorf("invalid bool: %s", s)
		}

	case abi.StringTy:
		return s, nil

	case abi.BytesTy:
		return hexToBytes(s)

	case abi.FixedBytesTy:
		b, err := hexToBytes(s)
		if err != nil {
			return nil, err
		}
		if len(b) > typ.Size {
			return nil, fmt.Errorf("bytes too long: got %d, max %d", len(b), typ.Size)
		}
		arr := reflect.New(typ.GetType()).Elem()
		reflect.Copy(arr, reflect.ValueOf(b))
		return arr.Interface(), nil

	default:
		return nil, fmt.Errorf("unsupported ABI type: %s", typ.String())
	}
}

// formatABIValue converts an ABI output value to a JSON-safe representation.
func formatABIValue(val any) any {
	switch v := val.(type) {
	case *big.Int:
		return v.String()
	case common.Address:
		return v.Hex()
	case []byte:
		return "0x" + hex.EncodeToString(v)
	case bool, string:
		return v
	default:
		rv := reflect.ValueOf(val)
		// Handle fixed-size byte arrays [N]byte.
		if rv.Kind() == reflect.Array && rv.Type().Elem().Kind() == reflect.Uint8 {
			b := make([]byte, rv.Len())
			for i := range b {
				b[i] = byte(rv.Index(i).Uint())
			}
			return "0x" + hex.EncodeToString(b)
		}
		// Fallback for small integer types (uint8, int32, etc.).
		return fmt.Sprintf("%v", v)
	}
}

// hexToBytes decodes a hex string (with or without 0x prefix) into bytes.
func hexToBytes(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	if len(s)%2 != 0 {
		s = "0" + s
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %s", s)
	}
	return b, nil
}
