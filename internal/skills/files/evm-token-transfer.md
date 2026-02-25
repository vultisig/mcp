---
name: EVM Token Transfer
description: Build an ERC-20 token transfer transaction on any EVM chain
tags: [evm, transfer, erc20, token]
---

# EVM Token Transfer

Build an unsigned ERC-20 token transfer transaction, ready for signing.

## Prerequisites

- Sender address (explicit or via `set_vault_info`)
- Token contract address and decimals
- Recipient address and amount

## Steps

### 1. Identify the token

If you only have a token name or symbol, look it up:

```
search_token(query: "USDC")
```

Note the `contract_address` and `decimals` from the result.

### 2. Check the sender's token balance

```
get_token_balance(token_address: "<contract>", address: "<sender>")
```

Verify the balance is sufficient for the transfer.

### 3. Encode the transfer calldata

Use `abi_encode` to build the ERC-20 `transfer(address,uint256)` calldata:

```
abi_encode(
  function: "transfer(address,uint256)",
  args: "<recipient>,<amount_in_smallest_unit>"
)
```

The amount must be in the token's smallest unit (e.g. for USDC with 6 decimals, 1 USDC = 1000000). Use `convert_amount` if needed:

```
convert_amount(amount: "1.5", decimals: 6, direction: "to_smallest")
```

### 4. Get transaction parameters

```
evm_tx_info(
  address: "<sender>",
  to: "<contract>",
  data: "<calldata_from_step_3>",
  value: "0"
)
```

This returns nonce, gas estimate, and fee suggestions.

### 5. Build the unsigned transaction

```
build_evm_tx(
  to: "<contract>",
  value: "0",
  data: "<calldata_from_step_3>",
  nonce: "<nonce>",
  gas_limit: "<estimated_gas>",
  max_fee_per_gas: "<suggested_max_fee_per_gas>",
  max_priority_fee_per_gas: "<max_priority_fee_per_gas>"
)
```

The result contains the RLP-encoded unsigned transaction hex, ready to be signed by the vault.

## Notes

- The `to` field in `build_evm_tx` is the **token contract address**, not the recipient.
- The `value` is `"0"` because the transfer amount is encoded in the calldata.
- Always verify the token balance before building the transaction.
