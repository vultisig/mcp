package tools

import (
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/bech32"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil/base58"
	bchcfg "github.com/gcash/bchd/chaincfg"
	"github.com/gcash/bchutil"
)

// utxoChainParams holds the chain-specific parameters needed for address decoding
// and pkScript generation.
type utxoChainParams struct {
	// addressToPkScript converts an address string to a pkScript for the chain.
	addressToPkScript func(addr string) ([]byte, error)
	// txVersion is the transaction version for this chain.
	txVersion int32
}

var utxoChains = map[string]utxoChainParams{
	"Bitcoin": {
		addressToPkScript: btcAddrToPkScript(&chaincfg.MainNetParams),
		txVersion:         2,
	},
	"Litecoin": {
		addressToPkScript: ltcAddrToPkScript,
		txVersion:         2,
	},
	"Dogecoin": {
		addressToPkScript: btcAddrToPkScript(&chaincfg.Params{
			PubKeyHashAddrID: 0x1e,
			ScriptHashAddrID: 0x16,
		}),
		txVersion: 1,
	},
	"Dash": {
		addressToPkScript: btcAddrToPkScript(&chaincfg.Params{
			PubKeyHashAddrID: 0x4c,
			ScriptHashAddrID: 0x10,
		}),
		txVersion: 1,
	},
	"Bitcoin-Cash": {
		addressToPkScript: bchAddrToPkScript,
		txVersion:         2,
	},
	"Zcash": {
		addressToPkScript: zcashAddrToPkScript,
		txVersion:         4,
	},
}

// btcAddrToPkScript returns a function that decodes a btcutil-compatible address
// and produces a pkScript using btcd's txscript.
func btcAddrToPkScript(params *chaincfg.Params) func(string) ([]byte, error) {
	return func(addr string) ([]byte, error) {
		decoded, err := btcutil.DecodeAddress(addr, params)
		if err != nil {
			return nil, fmt.Errorf("decode address %q: %w", addr, err)
		}
		return txscript.PayToAddrScript(decoded)
	}
}

// ltcAddrToPkScript decodes a Litecoin address (bech32 segwit or legacy P2PKH/P2SH)
// and produces a pkScript. btcutil.DecodeAddress requires network registration to
// handle bech32 prefixes, so segwit addresses are decoded directly.
func ltcAddrToPkScript(addr string) ([]byte, error) {
	if strings.HasPrefix(strings.ToLower(addr), "ltc1") {
		_, data, err := bech32.Decode(strings.ToLower(addr))
		if err != nil {
			return nil, fmt.Errorf("decode LTC bech32 address %q: %w", addr, err)
		}
		if len(data) < 2 {
			return nil, fmt.Errorf("LTC bech32 address too short: %q", addr)
		}
		witnessVersion := data[0]
		if witnessVersion > 16 {
			return nil, fmt.Errorf("invalid LTC witness version %d: %q", witnessVersion, addr)
		}
		witnessProgram, err := bech32.ConvertBits(data[1:], 5, 8, false)
		if err != nil {
			return nil, fmt.Errorf("decode LTC bech32 bits %q: %w", addr, err)
		}
		if witnessVersion == 0 {
			if len(witnessProgram) != 20 && len(witnessProgram) != 32 {
				return nil, fmt.Errorf("invalid LTC v0 witness program length %d: %q", len(witnessProgram), addr)
			}
		} else {
			if len(witnessProgram) < 2 || len(witnessProgram) > 40 {
				return nil, fmt.Errorf("invalid LTC witness program length %d: %q", len(witnessProgram), addr)
			}
		}
		var witnessOp byte
		if witnessVersion == 0 {
			witnessOp = txscript.OP_0
		} else {
			witnessOp = txscript.OP_1 - 1 + witnessVersion
		}
		builder := txscript.NewScriptBuilder()
		builder.AddOp(witnessOp)
		builder.AddData(witnessProgram)
		return builder.Script()
	}
	return btcAddrToPkScript(&chaincfg.Params{
		PubKeyHashAddrID: 0x30,
		ScriptHashAddrID: 0x32,
	})(addr)
}

// bchAddrToPkScript decodes a Bitcoin Cash CashAddr and produces a pkScript.
func bchAddrToPkScript(addr string) ([]byte, error) {
	decoded, err := bchutil.DecodeAddress(addr, &bchcfg.MainNetParams)
	if err != nil {
		return nil, fmt.Errorf("decode BCH address %q: %w", addr, err)
	}

	switch a := decoded.(type) {
	case *bchutil.AddressPubKeyHash:
		hash := a.Hash160()
		return p2pkhScript(hash[:]), nil
	case *bchutil.AddressScriptHash:
		hash := a.Hash160()
		return p2shScript(hash[:]), nil
	default:
		return nil, fmt.Errorf("unsupported BCH address type: %T", decoded)
	}
}

// zcashAddrToPkScript decodes a Zcash transparent address (t-addr) with its
// 2-byte version prefix and produces a pkScript.
func zcashAddrToPkScript(addr string) ([]byte, error) {
	decoded, version, err := base58.CheckDecode(addr)
	if err != nil {
		return nil, fmt.Errorf("decode Zcash address %q: %w", addr, err)
	}

	// Zcash t-addresses use a 2-byte prefix encoded as: first byte = version (from CheckDecode),
	// second byte = first byte of decoded payload. The actual hash is the remaining 20 bytes.
	// P2PKH: prefix 0x1c, 0xb8  (version=0x1c from CheckDecode, decoded[0]=0xb8)
	// P2SH:  prefix 0x1c, 0xbd  (version=0x1c from CheckDecode, decoded[0]=0xbd)
	if len(decoded) != 21 {
		return nil, fmt.Errorf("invalid Zcash address payload length: %d", len(decoded))
	}

	secondByte := decoded[0]
	hash := decoded[1:21]

	switch {
	case version == 0x1c && secondByte == 0xb8: // t1... P2PKH
		return p2pkhScript(hash), nil
	case version == 0x1c && secondByte == 0xbd: // t3... P2SH
		return p2shScript(hash), nil
	default:
		return nil, fmt.Errorf("unsupported Zcash address prefix: 0x%02x%02x", version, secondByte)
	}
}

// p2pkhScript builds OP_DUP OP_HASH160 <hash> OP_EQUALVERIFY OP_CHECKSIG.
func p2pkhScript(hash []byte) []byte {
	script := make([]byte, 25)
	script[0] = txscript.OP_DUP
	script[1] = txscript.OP_HASH160
	script[2] = 0x14 // push 20 bytes
	copy(script[3:23], hash)
	script[23] = txscript.OP_EQUALVERIFY
	script[24] = txscript.OP_CHECKSIG
	return script
}

// p2shScript builds OP_HASH160 <hash> OP_EQUAL.
func p2shScript(hash []byte) []byte {
	script := make([]byte, 23)
	script[0] = txscript.OP_HASH160
	script[1] = 0x14 // push 20 bytes
	copy(script[2:22], hash)
	script[22] = txscript.OP_EQUAL
	return script
}
