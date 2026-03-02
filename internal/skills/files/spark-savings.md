---
name: Spark Savings
description: Deposit and withdraw stablecoins from Spark savings vaults across EVM chains
tags: [evm, spark, savings, defi, erc4626, usds, dai, usdc, usdt]
---

# Spark Savings

Interact with Spark savings vaults — ERC-4626 yield-bearing wrappers around stablecoins and ETH. Deposit an underlying token, receive vault shares that appreciate over time, and withdraw whenever you want.

All vaults listed below share the same ERC-4626 interface. The deposit, withdraw, and redeem flows are identical regardless of vault or chain.

## Addresses

### Ethereum (chain_id: 1)

#### Savings vaults (SSR-based)

| Vault | Underlying | Vault address | Token address | Decimals |
|-------|-----------|---------------|---------------|----------|
| sDAI | DAI | `0x83F20F44975D03b1b09e64809B757c47f942BEeA` | `0x6B175474E89094C44Da98b954EedeAC495271d0F` | 18 |
| sUSDS | USDS | `0xa3931d71877C0E7a3148CB7Eb4463524FEc27fbD` | `0xdC035D45d973E3EC169d2276DDab16f1e407384F` | 18 |
| sUSDC | USDC | `0xBc65ad17c5C0a2A4D159fa5a503f4992c7B545FE` | `0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48` | 6 |

#### Spark Vault V2

| Vault | Underlying | Vault address | Token address | Decimals |
|-------|-----------|---------------|---------------|----------|
| spUSDC | USDC | `0x28B3a8fb53B741A8Fd78c0fb9A6B2393d896a43d` | `0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48` | 6 |
| spUSDT | USDT | `0xe2e7a17dFf93280dec073C995595155283e3C372` | `0xdAC17F958D2ee523a2206206994597C13D831ec7` | 6 |
| spPYUSD | PYUSD | `0x80128DbB9f07b93DDE62A6daeadb69ED14a7D354` | `0x6c3ea9036406852006290770BEdFcAbA0e23A0e8` | 6 |
| spETH | WETH | `0xfE6eb3b609a7C8352A241f7F3A21CEA4e9209B8f` | `0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2` | 18 |

#### Converters

| Contract | Address | Purpose |
|----------|---------|---------|
| DAI ↔ USDS | `0x3225737a9Bbb6473CB4a45b7244ACa2BeFdB276A` | Convert between DAI and USDS |

### Base (chain_id: 8453)

| Vault | Underlying | Vault address | Token address | Decimals |
|-------|-----------|---------------|---------------|----------|
| sUSDS | USDS | `0x5875eEE11Cf8398102FdAd704C9E96607675467a` | `0x820C137fa70C8691f0e44Dc420a5e53c168921Dc` | 18 |
| sUSDC | USDC | `0x3128a0F7f0ea68E7B7c9B00AFa7E41045828e858` | `0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913` | 6 |

### Arbitrum (chain_id: 42161)

| Vault | Underlying | Vault address | Token address | Decimals |
|-------|-----------|---------------|---------------|----------|
| sUSDS | USDS | `0xdDb46999F8891663a8F2828d25298f70416d7610` | `0x6491c05A82219b8D1479057361ff1654749b876b` | 18 |
| sUSDC | USDC | `0x940098b108fB7D0a7E374f6eDED7760787464609` | `0xaf88d065e77c8cC2239327C5EDb3A432268e5831` | 6 |

### Optimism (chain_id: 10)

| Vault | Underlying | Vault address | Token address | Decimals |
|-------|-----------|---------------|---------------|----------|
| sUSDS | USDS | `0xb5B2dc7fd34C249F4be7fB1fCea07950784229e0` | `0x4F13a96EC5C4Cf34e442b46Bbd98a0791F20edC3` | 18 |
| sUSDC | USDC | `0xCF9326e24EBfFBEF22ce1050007A43A3c0B6DB55` | `0x0b2C639c533813f4Aa9D7837CAf62653d097Ff85` | 6 |

### Unichain (chain_id: 130)

| Vault | Underlying | Vault address | Token address | Decimals |
|-------|-----------|---------------|---------------|----------|
| sUSDS | USDS | `0xA06b10Db9F390990364A3984C04FaDf1c13691b5` | `0x7E10036Acc4B56d4dFCa3b77810356CE52313F9C` | 18 |
| sUSDC | USDC | `0x14d9143BEcC348920b68D123687045db49a016C6` | `0x078D782b760474a361dDA0AF3839290b0EF57AD6` | 6 |

### Avalanche (chain_id: 43114)

| Vault | Underlying | Vault address | Token address | Decimals |
|-------|-----------|---------------|---------------|----------|
| spUSDC | USDC | `0x28B3a8fb53B741A8Fd78c0fb9A6B2393d896a43d` | `0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E` | 6 |

## Compatible tokens summary

Each vault accepts exactly one underlying token. To query dynamically:

```
evm_call(to: "<vault>", data: "0x38d52e0f", output_types: "address")
```

This calls `asset()` and returns the underlying token address.

| Vault family | Underlying | Chains |
|-------------|-----------|--------|
| sDAI | DAI | Ethereum |
| sUSDS | USDS | Ethereum, Base, Arbitrum, Optimism, Unichain |
| sUSDC | USDC | Ethereum, Base, Arbitrum, Optimism, Unichain |
| spUSDC | USDC | Ethereum, Avalanche |
| spUSDT | USDT | Ethereum |
| spPYUSD | PYUSD | Ethereum |
| spETH | WETH | Ethereum |

On Ethereum, if the user holds DAI but wants to deposit into sUSDS, they must first convert DAI → USDS via the converter (`0x3225737a9Bbb6473CB4a45b7244ACa2BeFdB276A`) by calling `daiToUsds(address usr, uint256 wad)`.

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

Returns the underlying asset amount the owner's shares are worth, in the token's smallest unit.

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
evm_get_token_balance(chain: "<chain>", contract_address: "<underlying_token>", address: "<sender>")
```

Verify the sender has enough of the underlying token.

### Step 2 — Approve the vault to spend the underlying token

**USDT special case (Ethereum only):** USDT requires setting the allowance to 0 before setting it to a new non-zero value. If depositing USDT into spUSDT, first send an `approve(vault, 0)` transaction, then send `approve(vault, amount)` as a separate transaction.

Encode the approval calldata:

```
abi_encode(
  signature: "approve(address,uint256)",
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
  signature: "deposit(uint256,address)",
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
  signature: "withdraw(uint256,address,address)",
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
  signature: "redeem(uint256,address,address)",
  args: "<shares_amount>,<receiver>,<owner>"
)
```

To redeem all shares, first query `balanceOf(address)` to get the exact share count, then pass that as the shares amount.

## DO NOTs

- **DO NOT** set `value` to anything other than `"0"` when calling vault or token functions. These are not payable (except spETH — but even spETH accepts WETH, not native ETH).
- **DO NOT** call `deposit` without first approving the vault to spend the underlying token. The transaction will revert.
- **DO NOT** confuse the vault address with the underlying token address. The `to` field for approval is the **underlying token**; the `to` field for deposit/withdraw/redeem is the **vault**.
- **DO NOT** approve USDT with a non-zero value if the current allowance is already non-zero. USDT on Ethereum requires resetting to 0 first.
- **DO NOT** use `mint` unless the user specifically asks for it. `deposit` is the standard entry point (specify assets in, receive shares). `mint` works in reverse (specify shares out, pay required assets).
- **DO NOT** call admin functions: `setDepositCap`, `setVsr`, `setVsrBounds`, `grantRole`, `revokeRole`, `take`, `drip`, `upgradeToAndCall`. These require privileged roles the user does not have.
- **DO NOT** mix up decimals. Use the Decimals column in the address tables above. When in doubt, query `decimals()` on the vault.
- **DO NOT** deposit more than `maxDeposit(address)` returns for the receiver. The vault may have a deposit cap.
- **DO NOT** use `search_token` to look up Spark vault tokens (spUSDT, spUSDC, etc.). They are not listed on CoinGecko. Use the addresses in this skill directly.
