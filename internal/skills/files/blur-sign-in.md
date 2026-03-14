---
name: Blur Sign-In
description: Authenticate with Blur NFT marketplace using EIP-712 wallet signature
tags: [evm, blur, nft, auth, eip712]
---

# Blur Sign-In

Authenticate a wallet with the Blur NFT marketplace by signing an EIP-712 challenge message.

## Overview

Blur requires wallets to sign an EIP-712 typed-data challenge to prove ownership before interacting with the platform (viewing bids, placing orders, etc.). This skill walks through the challenge-response flow.

## Prerequisites

- Ethereum address (explicit or via `set_vault_info`)
- The vault must support EIP-712 typed-data signing

## Steps

### 1. Request a sign-in challenge

Call the Blur auth API to get a nonce/challenge for the wallet address:

```
evm_call(
  chain: "Ethereum",
  to: "<blur_auth_endpoint>",
  data: "<challenge_request_for_address>"
)
```

The response contains an EIP-712 typed-data message that must be signed.

### 2. Sign the challenge

Present the EIP-712 typed-data payload to the vault for signing. The payload typically includes:

- **Domain**: `{ name: "Blur", version: "1", chainId: 1 }`
- **Primary type**: `Challenge`
- **Message**: `{ text: "Sign in to Blur", nonce: "<nonce_from_step_1>" }`

The vault signs this and returns the signature.

### 3. Submit the signature

Send the signed challenge back to the Blur auth endpoint to receive an access token:

```
evm_call(
  chain: "Ethereum",
  to: "<blur_auth_endpoint>",
  data: "<signed_challenge_submission>"
)
```

The response contains an auth token for subsequent Blur API calls.

## Notes

- The sign-in challenge expires after a short window — request and sign promptly.
- The auth token is typically valid for 24 hours.
- This flow is similar to "Sign-In with Ethereum" (EIP-4361) but uses Blur's custom EIP-712 schema.
- No on-chain transaction is required — this is purely an off-chain signature.
