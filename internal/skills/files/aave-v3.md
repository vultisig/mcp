---
name: Aave V3
description: Supply, withdraw, borrow, and repay tokens on Aave V3 lending protocol across EVM chains
tags: [evm, aave, lending, defi, borrow, supply, collateral]
---

# Aave V3

Interact with Aave V3 lending pools — supply tokens as collateral, borrow against them, repay debt, and withdraw. All operations use the generic EVM tools (`evm_call`, `abi_encode`, `evm_tx_info`, `build_evm_tx`).

## Contract Addresses

### Pool Contract

The Pool contract handles all lending operations (supply, withdraw, borrow, repay).

| Chain | Chain ID | Pool Address |
|-------|----------|-------------|
| Ethereum | 1 | `0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2` |
| Optimism | 10 | `0x794a61358D6845594F94dc1DB02A252b5b4814aD` |
| BSC | 56 | `0x6807dc923806fE8Fd134338EABCA509979a7e0cB` |
| Polygon | 137 | `0x794a61358D6845594F94dc1DB02A252b5b4814aD` |
| zkSync | 324 | `0x78e30497a3c7527d953c6B1E3541b021A98Ac43c` |
| Mantle | 5000 | `0xCFbFa83332bB1A3154FA4BA4febedf5c94bDA7c0` |
| Base | 8453 | `0xA238Dd80C259a72e81d7e4664a9801593F98d1c5` |
| Arbitrum | 42161 | `0x794a61358D6845594F94dc1DB02A252b5b4814aD` |
| Avalanche | 43114 | `0x794a61358D6845594F94dc1DB02A252b5b4814aD` |

### Pool Data Provider Contract

The Data Provider exposes read-only reserve configuration data.

| Chain | Chain ID | Data Provider Address |
|-------|----------|-----------------------|
| Ethereum | 1 | `0x0a16f2FCC0D44FaE41cc54e079281D84A363bECD` |
| Optimism | 10 | `0x243Aa95cAC2a25651eda86e80bEe66114413c43b` |
| BSC | 56 | `0xc90Df74A7c16245c5F5C5870327Ceb38Fe5d5328` |
| Polygon | 137 | `0x243Aa95cAC2a25651eda86e80bEe66114413c43b` |
| zkSync | 324 | `0x9057ac7b2D35606F8AD5aE2FCBafcD94E58D9927` |
| Mantle | 5000 | `0x487c5c669D9eee6057C44973207101276cf73b68` |
| Base | 8453 | `0x0F43731EB8d45A581f4a36DD74F5f358bc90C73A` |
| Arbitrum | 42161 | `0x243Aa95cAC2a25651eda86e80bEe66114413c43b` |
| Avalanche | 43114 | `0x243Aa95cAC2a25651eda86e80bEe66114413c43b` |

## Read Operations

All read operations use `evm_call`. No transaction needed.

### Get account summary (collateral, debt, health factor)

```
evm_call(
  chain: "<chain>",
  to: "<pool_address>",
  data: <abi_encode("getUserAccountData(address)", "<user_address>")>,
  output_types: "uint256,uint256,uint256,uint256,uint256,uint256"
)
```

Returns (all uint256):
1. `totalCollateralBase` — total collateral in USD (8 decimals, divide by 1e8)
2. `totalDebtBase` — total debt in USD (8 decimals)
3. `availableBorrowsBase` — remaining borrow capacity in USD (8 decimals)
4. `currentLiquidationThreshold` — weighted liquidation threshold (basis points, divide by 100 for %)
5. `ltv` — weighted loan-to-value ratio (basis points)
6. `healthFactor` — health factor (18 decimals, divide by 1e18; > 1.0 is safe, < 1.0 is liquidatable)

If `totalDebtBase` is 0, health factor is effectively infinite (no liquidation risk).

### Get reserve rates and configuration

Query supply APY and borrow APY from the Pool:

```
evm_call(
  chain: "<chain>",
  to: "<pool_address>",
  data: <abi_encode("getReserveData(address)", "<asset_address>")>,
  output_types: "uint256,uint128,uint128,uint128,uint128,uint128,uint40,uint16,address,address,address,address,uint128"
)
```

Key fields from the return tuple:
- Index 2: `currentLiquidityRate` — supply rate in RAY (27 decimals). APY% = value / 1e25
- Index 4: `currentVariableBorrowRate` — borrow rate in RAY. APY% = value / 1e25

Query reserve configuration from the Data Provider:

```
evm_call(
  chain: "<chain>",
  to: "<data_provider_address>",
  data: <abi_encode("getReserveConfigurationData(address)", "<asset_address>")>,
  output_types: "uint256,uint256,uint256,uint256,uint256,bool,bool,bool,bool,bool"
)
```

Returns:
1. `decimals`
2. `ltv` (basis points)
3. `liquidationThreshold` (basis points)
4. `liquidationBonus` (10000 + bonus in basis points; subtract 10000 and divide by 100 for %)
5. `reserveFactor`
6. `usageAsCollateralEnabled`
7. `borrowingEnabled`
8. `stableBorrowRateEnabled` (deprecated, always false on v3)
9. `isActive`
10. `isFrozen`

### Check aToken balance (supplied position)

Each supplied asset has a corresponding aToken. Query the aToken balance to see how much of a token a user has supplied:

```
evm_call(
  chain: "<chain>",
  to: "<atoken_address>",
  data: <abi_encode("balanceOf(address)", "<user_address>")>,
  output_types: "uint256"
)
```

To find the aToken address for a reserve:

```
evm_call(
  chain: "<chain>",
  to: "<data_provider_address>",
  data: <abi_encode("getReserveTokensAddresses(address)", "<asset_address>")>,
  output_types: "address,address,address"
)
```

Returns: `(aTokenAddress, stableDebtTokenAddress, variableDebtTokenAddress)`

## Supply Flow (Deposit)

Supplying tokens to Aave V3 requires two transactions: an ERC-20 approval followed by the pool supply call.

### Step 1 — Check token balance

```
evm_get_token_balance(chain: "<chain>", contract_address: "<asset_address>", address: "<sender>")
```

Verify the sender has enough of the token to supply.

### Step 2 — Approve the Pool to spend the token

Encode the approval calldata:

```
abi_encode(
  signature: "approve(address,uint256)",
  args: "<pool_address>,<supply_amount_in_wei>"
)
```

**USDT special case (Ethereum only):** USDT requires setting the allowance to 0 before setting a new non-zero value. If supplying USDT, first send `approve(pool, 0)`, then `approve(pool, amount)` as separate transactions.

Get transaction parameters and build:

```
evm_tx_info(chain: "<chain>", address: "<sender>", to: "<asset_address>", data: "<approve_calldata>", value: "0")
```

```
build_evm_tx(
  chain: "<chain>",
  to: "<asset_address>",
  value: "0",
  data: "<approve_calldata>",
  nonce: "<nonce>",
  gas_limit: "<estimated_gas>",
  max_fee_per_gas: "<suggested_max_fee>",
  max_priority_fee_per_gas: "<tip>"
)
```

### Step 3 — Supply to the pool

Encode the supply calldata:

```
abi_encode(
  signature: "supply(address,uint256,address,uint16)",
  args: "<asset_address>,<supply_amount_in_wei>,<sender_address>,0"
)
```

The last parameter is `referralCode` — always use `0`.

Get transaction parameters (use nonce = previous nonce + 1):

```
evm_tx_info(chain: "<chain>", address: "<sender>", to: "<pool_address>", data: "<supply_calldata>", value: "0")
```

```
build_evm_tx(
  chain: "<chain>",
  to: "<pool_address>",
  value: "0",
  data: "<supply_calldata>",
  nonce: "<nonce + 1>",
  gas_limit: "<estimated_gas>",
  max_fee_per_gas: "<suggested_max_fee>",
  max_priority_fee_per_gas: "<tip>"
)
```

Supply gas is typically ~250,000–300,000. Always use the estimate from `evm_tx_info`.

## Withdraw Flow

### Step 1 — Check available balance

Query `getUserAccountData` (see Read Operations) to verify the user has supplied collateral. To check the exact amount of a specific token supplied, query the aToken balance.

### Step 2 — Build the withdraw transaction

```
abi_encode(
  signature: "withdraw(address,uint256,address)",
  args: "<asset_address>,<withdraw_amount_in_wei>,<sender_address>"
)
```

Use `type(uint256).max` (`115792089237316195423570985008687907853269984665640564039457584007913129639935`) as the amount to withdraw the full balance.

```
evm_tx_info(chain: "<chain>", address: "<sender>", to: "<pool_address>", data: "<withdraw_calldata>", value: "0")
```

```
build_evm_tx(
  chain: "<chain>",
  to: "<pool_address>",
  value: "0",
  data: "<withdraw_calldata>",
  nonce: "<nonce>",
  gas_limit: "<estimated_gas>",
  max_fee_per_gas: "<suggested_max_fee>",
  max_priority_fee_per_gas: "<tip>"
)
```

## Borrow Flow

### Step 1 — Check borrow capacity

Query `getUserAccountData` to verify `availableBorrowsBase > 0` and check the health factor impact.

### Step 2 — Build the borrow transaction

```
abi_encode(
  signature: "borrow(address,uint256,uint256,uint16,address)",
  args: "<asset_address>,<borrow_amount_in_wei>,2,0,<sender_address>"
)
```

- Interest rate mode `2` = variable rate (always use variable; stable rate is deprecated on v3)
- Referral code `0`

```
evm_tx_info(chain: "<chain>", address: "<sender>", to: "<pool_address>", data: "<borrow_calldata>", value: "0")
```

```
build_evm_tx(
  chain: "<chain>",
  to: "<pool_address>",
  value: "0",
  data: "<borrow_calldata>",
  nonce: "<nonce>",
  gas_limit: "<estimated_gas>",
  max_fee_per_gas: "<suggested_max_fee>",
  max_priority_fee_per_gas: "<tip>"
)
```

## Repay Flow

Repaying requires two transactions: an ERC-20 approval followed by the pool repay call.

### Step 1 — Check debt balance

Query `getUserAccountData` to see total debt, or query the variable debt token balance for a specific asset.

### Step 2 — Approve the Pool to spend the token

Same as the supply approval flow — encode `approve(pool, amount)` for the asset token.

**USDT special case** applies here too.

### Step 3 — Repay to the pool

```
abi_encode(
  signature: "repay(address,uint256,uint256,address)",
  args: "<asset_address>,<repay_amount_in_wei>,2,<sender_address>"
)
```

- Interest rate mode `2` = variable rate
- Use `type(uint256).max` as the amount to repay the full debt

```
evm_tx_info(chain: "<chain>", address: "<sender>", to: "<pool_address>", data: "<repay_calldata>", value: "0")
```

```
build_evm_tx(
  chain: "<chain>",
  to: "<pool_address>",
  value: "0",
  data: "<repay_calldata>",
  nonce: "<nonce + 1>",
  gas_limit: "<estimated_gas>",
  max_fee_per_gas: "<suggested_max_fee>",
  max_priority_fee_per_gas: "<tip>"
)
```

Repay gas is typically ~250,000–300,000. Always use the estimate from `evm_tx_info`.

## DO NOTs

- **DO NOT** set `value` to anything other than `"0"` for pool or token calls. All Aave V3 functions are non-payable (use WETH for ETH positions).
- **DO NOT** call `supply` without first approving the Pool to spend the asset token. The transaction will revert.
- **DO NOT** borrow more than `availableBorrowsBase` — the transaction will revert.
- **DO NOT** withdraw collateral that would push the health factor below 1.0 — this enables liquidation.
- **DO NOT** use interest rate mode `1` (stable rate) — it is deprecated on Aave V3 and will revert on most markets.
- **DO NOT** approve USDT with a non-zero value if the current allowance is already non-zero on Ethereum. Reset to 0 first.
- **DO NOT** confuse the Pool address with the asset address. The `to` field for approval is the **asset token**; the `to` field for supply/withdraw/borrow/repay is the **Pool**.
- **DO NOT** use a pool address from one chain on a different chain — each chain has its own deployment.
- **DO NOT** forget to convert human-readable amounts to wei using the token's decimals. Use `convert_amount` if needed.
