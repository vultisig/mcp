---
name: UTXO Transfer
description: Build an unsigned UTXO transaction for Bitcoin, Litecoin, Dogecoin, and other UTXO chains
tags: [utxo, bitcoin, litecoin, dogecoin, transfer]
---

# UTXO Transfer

Build an unsigned UTXO transaction for any supported chain: Bitcoin, Litecoin, Dogecoin, Bitcoin-Cash, Dash, or Zcash.

## Prerequisites

- Vault set via `set_vault_info` (or an explicit sender address)
- Recipient address
- Amount in the chain's base unit (satoshis for BTC/LTC/BCH/DASH, koinus for DOGE, zatoshis for ZEC)

## Bitcoin (BTC)

### 1. Get the recommended fee rate

```
btc_fee_rate()
```

Returns the recommended sat/vB fee rate from THORChain.

### 2. Build the unsigned PSBT

```
build_btc_send(
  to_address: "<recipient>",
  amount: "<satoshis>",
  fee_rate: <sat_per_vb>
)
```

For THORChain swaps, add a `memo` parameter with the swap instruction. The tool automatically selects UTXOs, calculates fees, and handles change.

---

## Litecoin (LTC)

### 1. Get the recommended fee rate

```
ltc_fee_rate()
```

### 2. Build the unsigned PSBT

```
build_ltc_send(
  to_address: "<recipient>",
  amount: "<litoshis>",
  fee_rate: <sat_per_vb>
)
```

---

## Dogecoin (DOGE)

### 1. Get the recommended fee rate

```
doge_fee_rate()
```

### 2. Build the unsigned PSBT

```
build_doge_send(
  to_address: "<recipient>",
  amount: "<koinus>",
  fee_rate: <sat_per_vb>
)
```

Minimum output (dust limit) is 100,000,000 koinus (1 DOGE).

---

## Bitcoin-Cash (BCH)

### 1. Get the recommended fee rate

```
bch_fee_rate()
```

### 2. Build the unsigned PSBT

```
build_bch_send(
  to_address: "<recipient>",
  amount: "<satoshis>",
  fee_rate: <sat_per_vb>
)
```

BCH addresses can use either legacy (`1...`) or CashAddr (`bitcoincash:q...`) format.

---

## Dash (DASH)

### 1. Get the recommended fee rate

```
dash_fee_rate()
```

### 2. Build the unsigned PSBT

```
build_dash_send(
  to_address: "<recipient>",
  amount: "<duffs>",
  fee_rate: <sat_per_vb>
)
```

---

## Zcash (ZEC)

Zcash transactions use automatic ZIP-317 fee estimation — no fee rate parameter needed.

```
build_zec_send(
  to_address: "<recipient>",
  amount: "<zatoshis>"
)
```

The tool selects UTXOs, estimates the fee using ZIP-317, and returns a v4 transparent transaction with embedded signature hashes ready for ECDSA signing.

---

## Optional parameters (all chains except ZEC)

| Parameter | Description |
|-----------|-------------|
| `memo` | OP_RETURN memo (max 80 bytes). Used for THORChain/MayaChain swap instructions. |
| `address` | Override the sender address. Falls back to the vault-derived address if omitted. |

## Notes

- All tools require `set_vault_info` to be called first (unless `address` is overridden).
- Use `convert_amount` to convert human-readable amounts to base units if needed.
- The transaction result contains the unsigned hex, ready to be signed by the vault.
- BTC, LTC, BCH, DASH transactions are encoded as PSBT (`tx_encoding: "psbt"`).
- ZEC transactions are encoded as Zcash v4 (`tx_encoding: "zcash_v4"`).