---
name: EVM Contract Call
description: Read from or write to any EVM smart contract
tags: [evm, contract, abi, call]
---

# EVM Contract Call

Interact with any EVM smart contract — read-only calls or state-changing transactions.

## Read-Only Call (no transaction needed)

### 1. Encode the calldata

```
abi_encode(
  function: "balanceOf(address)",
  args: "0x1234..."
)
```

### 2. Execute the call

```
evm_call(
  to: "<contract_address>",
  data: "<calldata>",
  output_types: "uint256"
)
```

The `output_types` parameter decodes the raw return data. Common patterns:

| Function | output_types |
|----------|-------------|
| `balanceOf(address)` | `uint256` |
| `name()` | `string` |
| `decimals()` | `uint8` |
| `getReserves()` | `uint112,uint112,uint32` |
| `owner()` | `address` |

## State-Changing Transaction

### 1. Encode the calldata

```
abi_encode(
  function: "approve(address,uint256)",
  args: "<spender>,<amount>"
)
```

### 2. Get transaction parameters

```
evm_tx_info(
  address: "<sender>",
  to: "<contract_address>",
  data: "<calldata>",
  value: "0"
)
```

Set `value` to the wei amount if the function is payable.

### 3. Build the unsigned transaction

```
build_evm_tx(
  to: "<contract_address>",
  value: "0",
  data: "<calldata>",
  nonce: "<nonce>",
  gas_limit: "<estimated_gas>",
  max_fee_per_gas: "<suggested_max_fee_per_gas>",
  max_priority_fee_per_gas: "<max_priority_fee_per_gas>"
)
```

## Decoding Return Data

If you already have raw hex output from a contract call, decode it:

```
abi_decode(
  types: "uint256,address,bool",
  data: "0x..."
)
```

## Notes

- Use `evm_call` for read-only operations — these are free and don't require signing.
- Use `build_evm_tx` for state-changing operations — these produce an unsigned transaction for signing.
- The `abi_encode` function selector is derived from the function signature automatically.
- For functions with no arguments, use an empty `args` or omit it.
