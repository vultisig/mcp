---
name: UTXO Transfer
description: Build an unsigned UTXO transaction for Bitcoin, Litecoin, Dogecoin, and other UTXO chains
tags: [utxo, bitcoin, litecoin, dogecoin, transfer]
---

# UTXO Transfer

Build an unsigned UTXO transaction for any supported chain (Bitcoin, Litecoin, Dogecoin, Dash, Bitcoin-Cash, Zcash).

## Prerequisites

- Sender address (explicit or via `set_vault_info`)
- Recipient address
- Amount to send (in the chain's base unit, e.g. satoshis for Bitcoin)

## Steps

### 1. Check the sender's balance

```
get_utxo_balance(chain: "bitcoin", address: "<sender>")
```

Verify the balance is sufficient for the transfer plus fees.

### 2. List available UTXOs

```
list_utxos(chain: "bitcoin", address: "<sender>")
```

This returns unspent outputs with `txid`, `vout`, and `value` (in satoshis).

### 3. Select inputs and calculate fee

Select UTXOs whose total value covers the send amount plus the desired fee. A typical fee calculation:

- Estimate transaction size: `~10 + (148 * num_inputs) + (34 * num_outputs) + 10` bytes (for legacy P2PKH)
- Multiply by the desired fee rate (satoshis per byte)
- For segwit transactions, the weight calculation differs

Ensure: `sum(input_values) >= send_amount + fee`

The change (if any) goes back to the sender:
`change = sum(input_values) - send_amount - fee`

### 4. Build the unsigned transaction

```
build_utxo_tx(
  chain: "bitcoin",
  inputs: '[{"txid":"<txid1>","vout":0,"value":50000},{"txid":"<txid2>","vout":1,"value":30000}]',
  outputs: '[{"address":"<recipient>","amount":70000},{"address":"<sender>","amount":8000}]'
)
```

The fee is implicit: `fee = sum(inputs) - sum(outputs)`.

The result contains the serialized unsigned transaction hex, ready to be signed by the vault.

## Notes

- Always include a change output back to the sender unless the inputs exactly cover the amount + fee.
- The fee is **not** an explicit parameter — it's the difference between total inputs and total outputs.
- Use `convert_amount` to convert between human-readable amounts and satoshis if needed.
- Supported chains: `bitcoin`, `litecoin`, `dogecoin`, `dash`, `bitcoin-cash`, `zcash`.
