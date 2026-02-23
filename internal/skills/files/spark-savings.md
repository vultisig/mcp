---
name: Spark Savings
description: Deposit and withdraw stablecoins from Spark savings vaults (sUSDS, sDAI, sUSDC) across EVM chains
tags: [evm, spark, savings, defi, erc4626, usds, dai, usdc]
---

# Spark Savings

Interact with Spark savings vaults — ERC-4626 yield-bearing wrappers around stablecoins. Deposit an underlying stablecoin, receive vault shares that appreciate over time, and withdraw whenever you want.

## Addresses

### Ethereum (chain_id: 1)

| Contract | Address |
|----------|---------|
| sDAI vault | `0x83F20F44975D03b1b09e64809B757c47f942BEeA` |
| sUSDS vault | `0xa3931d71877C0E7a3148CB7Eb4463524FEc27fbD` |
| DAI (underlying for sDAI) | `0x6B175474E89094C44Da98b954EedeAC495271d0F` |
| USDS (underlying for sUSDS) | `0xdC035D45d973E3EC169d2276DDab16f1e407384F` |
| DAI ↔ USDS converter | `0x3225737a9Bbb6473CB4a45b7244ACa2BeFdB276A` |

### Base (chain_id: 8453)

| Contract | Address |
|----------|---------|
| sUSDS vault | `0x5875eEE11Cf8398102FdAd704C9E96607675467a` |
| sUSDC vault | `0x3128a0F7f0ea68E7B7c9B00AFa7E41045828e858` |
| USDS (underlying for sUSDS) | `0x820C137fa70C8691f0e44Dc420a5e53c168921Dc` |
| USDC (underlying for sUSDC) | `0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913` |

### Arbitrum (chain_id: 42161)

| Contract | Address |
|----------|---------|
| sUSDS vault | `0xdDb46999F8891663a8F2828d25298f70416d7610` |
| sUSDC vault | `0x940098b108fB7D0a7E374f6eDED7760787464609` |
| USDS (underlying for sUSDS) | `0x6491c05A82219b8D1479057361ff1654749b876b` |
| USDC (underlying for sUSDC) | `0xaf88d065e77c8cC2239327C5EDb3A432268e5831` |

### Optimism (chain_id: 10)

| Contract | Address |
|----------|---------|
| sUSDS vault | `0xb5B2dc7fd34C249F4be7fB1fCea07950784229e0` |
| sUSDC vault | `0xCF9326e24EBfFBEF22ce1050007A43A3c0B6DB55` |
| USDS (underlying for sUSDS) | `0x4F13a96EC5C4Cf34e442b46Bbd98a0791F20edC3` |
| USDC (underlying for sUSDC) | `0x0b2C639c533813f4Aa9D7837CAf62653d097Ff85` |

## Compatible tokens

Each vault accepts exactly one underlying token. To find which token a vault accepts, query:

```
evm_call(to: "<vault>", data: "0x38d52e0f", output_types: "address")
```

This calls `asset()` and returns the underlying token address.

- **sDAI** accepts **DAI** (Ethereum only)
- **sUSDS** accepts **USDS** (all chains listed above)
- **sUSDC** accepts **USDC** (Base, Arbitrum, Optimism)

On Ethereum, if the user holds DAI but wants to deposit into sUSDS, they must first convert DAI → USDS via the DAI ↔ USDS converter (`0x3225737a9Bbb6473CB4a45b7244ACa2BeFdB276A`) by calling `daiToUsds(address usr, uint256 wad)`.

## Read operations

All read operations use `evm_call`. No transaction needed.

### Check position value (assets owned)

```
evm_call(
  to: "<vault>",
  data: <abi_encode("assetsOf(address)", "<owner>")>,
  output_types: "uint256"
)
```

Returns the underlying asset amount the owner's shares are worth, in the asset's smallest unit. sDAI and sUSDS use 18 decimals; sUSDC uses 6 decimals (matches USDC).

### Check share balance

```
evm_call(
  to: "<vault>",
  data: <abi_encode("balanceOf(address)", "<owner>")>,
  output_types: "uint256"
)
```

### Preview a deposit (how many shares for a given asset amount)

```
evm_call(
  to: "<vault>",
  data: <abi_encode("previewDeposit(uint256)", "<asset_amount>")>,
  output_types: "uint256"
)
```

### Preview a withdrawal (how many shares will be burned)

```
evm_call(
  to: "<vault>",
  data: <abi_encode("previewWithdraw(uint256)", "<asset_amount>")>,
  output_types: "uint256"
)
```

### Check total assets in the vault

```
evm_call(
  to: "<vault>",
  data: "0x01e1d114",
  output_types: "uint256"
)
```

This calls `totalAssets()`.

### Convert between shares and assets

Shares → assets:
```
evm_call(
  to: "<vault>",
  data: <abi_encode("convertToAssets(uint256)", "<shares>")>,
  output_types: "uint256"
)
```

Assets → shares:
```
evm_call(
  to: "<vault>",
  data: <abi_encode("convertToShares(uint256)", "<assets>")>,
  output_types: "uint256"
)
```

## Deposit flow

Depositing requires two transactions: an ERC-20 approval followed by the vault deposit.

### Step 1 — Check the underlying token balance

```
get_token_balance(token_address: "<underlying_token>", address: "<sender>")
```

Verify the sender has enough of the underlying token.

### Step 2 — Approve the vault to spend the underlying token

Encode the approval calldata:

```
abi_encode(
  function: "approve(address,uint256)",
  args: "<vault_address>,<deposit_amount>"
)
```

Get transaction parameters and build:

```
evm_tx_info(address: "<sender>", to: "<underlying_token>", data: "<approve_calldata>", value: "0")
```

```
build_evm_tx(
  to: "<underlying_token>",
  value: "0",
  data: "<approve_calldata>",
  nonce: "<nonce>",
  gas_limit: "<estimated_gas>",
  max_fee_per_gas: "<suggested_max_fee>",
  max_priority_fee_per_gas: "<tip>"
)
```

### Step 3 — Deposit into the vault

Encode the deposit calldata:

```
abi_encode(
  function: "deposit(uint256,address)",
  args: "<deposit_amount>,<receiver_address>"
)
```

The receiver is typically the same as the sender. The deposit amount is in the underlying token's smallest unit.

Get transaction parameters (use nonce = previous nonce + 1):

```
evm_tx_info(address: "<sender>", to: "<vault>", data: "<deposit_calldata>", value: "0")
```

```
build_evm_tx(
  to: "<vault>",
  value: "0",
  data: "<deposit_calldata>",
  nonce: "<nonce>",
  gas_limit: "<estimated_gas>",
  max_fee_per_gas: "<suggested_max_fee>",
  max_priority_fee_per_gas: "<tip>"
)
```

## Withdraw flow

Withdraw a specific amount of underlying assets. The vault burns the required shares automatically.

### Step 1 — Check available balance

```
evm_call(
  to: "<vault>",
  data: <abi_encode("maxWithdraw(address)", "<owner>")>,
  output_types: "uint256"
)
```

This returns the maximum underlying asset amount the owner can withdraw.

### Step 2 — Build the withdraw transaction

```
abi_encode(
  function: "withdraw(uint256,address,address)",
  args: "<withdraw_amount>,<receiver>,<owner>"
)
```

The receiver gets the underlying tokens. The owner is the account whose shares are burned. For a normal self-withdrawal, receiver = owner = sender.

```
evm_tx_info(address: "<sender>", to: "<vault>", data: "<withdraw_calldata>", value: "0")
```

```
build_evm_tx(
  to: "<vault>",
  value: "0",
  data: "<withdraw_calldata>",
  nonce: "<nonce>",
  gas_limit: "<estimated_gas>",
  max_fee_per_gas: "<suggested_max_fee>",
  max_priority_fee_per_gas: "<tip>"
)
```

## Redeem flow (alternative to withdraw)

Use `redeem` when the user wants to exit a specific number of **shares** rather than a specific asset amount.

```
abi_encode(
  function: "redeem(uint256,address,address)",
  args: "<shares_amount>,<receiver>,<owner>"
)
```

To redeem all shares, first query `balanceOf(address)` to get the exact share count, then pass that as the shares amount.

## DO NOTs

- **DO NOT** set `value` to anything other than `"0"` when calling vault or token functions. These are not payable.
- **DO NOT** call `deposit` without first approving the vault to spend the underlying token. The transaction will revert.
- **DO NOT** confuse the vault address with the underlying token address. The `to` field for approval is the **underlying token**; the `to` field for deposit/withdraw/redeem is the **vault**.
- **DO NOT** use `mint` unless the user specifically asks for it. `deposit` is the standard entry point (specify assets in, receive shares). `mint` works in reverse (specify shares out, pay required assets).
- **DO NOT** call admin functions: `setDepositCap`, `setVsr`, `setVsrBounds`, `grantRole`, `revokeRole`, `take`, `drip`, `upgradeToAndCall`. These require privileged roles the user does not have.
- **DO NOT** mix up decimals. sDAI/sUSDS use 18 decimals; sUSDC uses 6 decimals. Always check with `convert_amount` or query `decimals()` if unsure.
- **DO NOT** deposit more than `maxDeposit(address)` returns for the receiver. The vault may have a deposit cap.
