---
name: Wrap/Unwrap ETH (WETH)
description: Convert between native ETH and WETH (Wrapped Ether) on any EVM chain
tags: [evm, weth, wrap, unwrap, eth]
---

# Wrap/Unwrap ETH (WETH)

Convert between native ETH and WETH using the canonical WETH9 contract. Wrapping deposits native ETH and mints WETH; unwrapping burns WETH and returns native ETH.

## WETH Contract Addresses

| Chain     | WETH Address                                 |
|-----------|----------------------------------------------|
| Ethereum  | `0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2` |
| Arbitrum  | `0x82aF49447D8a07e3bd95BD0d56f35241523fBab1` |
| Optimism  | `0x4200000000000000000000000000000000000006` |
| Base      | `0x4200000000000000000000000000000000000006` |
| Polygon   | `0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270` |
| Avalanche | `0xB31f66AA3C1e785363F0875A1B74E27b85FD66c7` |
| BSC       | `0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c` |
| Blast     | `0x4300000000000000000000000000000000000004` |

Note: On Polygon, Avalanche, and BSC the "WETH" contract wraps the chain's native token (MATIC, AVAX, BNB), not ETH. The interface is identical.

## Wrap ETH → WETH

Wrapping calls the WETH `deposit()` function with a payable ETH value.

### 1. Get the sender's address

```
get_address(chain: "Ethereum")
```

### 2. Check native ETH balance

```
evm_get_balance(chain: "Ethereum", address: "<sender>")
```

Ensure sufficient balance for the wrap amount plus gas.

### 3. Encode the deposit calldata

```
abi_encode(
  signature: "deposit()"
)
```

The `deposit()` function takes no arguments — the amount to wrap is sent as the transaction `value`.

### 4. Get transaction parameters

```
evm_tx_info(
  chain: "Ethereum",
  address: "<sender>",
  to: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
  data: "<calldata_from_step_3>",
  value: "<amount_in_wei>"
)
```

### 5. Build the unsigned transaction

```
build_evm_tx(
  chain: "Ethereum",
  to: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
  value: "<amount_in_wei>",
  data: "<calldata_from_step_3>",
  nonce: "<nonce>",
  gas_limit: "<estimated_gas>",
  max_fee_per_gas: "<suggested_max_fee>",
  max_priority_fee_per_gas: "<max_priority_fee>"
)
```

The vault signs and broadcasts the transaction. Upon success, the sender receives an equal amount of WETH.

## Unwrap WETH → ETH

Unwrapping calls the WETH `withdraw(uint256)` function. No ETH value is sent — the WETH is burned and native ETH is returned.

### 1. Check WETH balance

```
evm_get_token_balance(
  chain: "Ethereum",
  address: "<sender>",
  token: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"
)
```

### 2. Encode the withdraw calldata

```
abi_encode(
  signature: "withdraw(uint256)",
  args: "<amount_in_wei>"
)
```

### 3. Get transaction parameters

```
evm_tx_info(
  chain: "Ethereum",
  address: "<sender>",
  to: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
  data: "<calldata_from_step_2>",
  value: "0"
)
```

### 4. Build the unsigned transaction

```
build_evm_tx(
  chain: "Ethereum",
  to: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
  value: "0",
  data: "<calldata_from_step_2>",
  nonce: "<nonce>",
  gas_limit: "<estimated_gas>",
  max_fee_per_gas: "<suggested_max_fee>",
  max_priority_fee_per_gas: "<max_priority_fee>"
)
```

## Notes

- `deposit()` gas is ~45,000. `withdraw(uint256)` gas is ~35,000. Always use the estimate from `evm_tx_info`.
- WETH is an ERC-20 token — after wrapping, the balance is queryable via `evm_get_token_balance`.
- Wrapping and unwrapping are 1:1 — no slippage, no fees beyond gas.
- The WETH contract is immutable and has no admin functions — it is one of the safest contracts on Ethereum.
- Use `convert_amount` if the user specifies an amount in ETH rather than wei (1 ETH = 10^18 wei).
