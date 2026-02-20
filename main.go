package main

//go:generate abigen --abi ./internal/ethereum/contracts/aave-v3/atoken.abi --pkg aavev3contracts --type AToken --out ./internal/ethereum/contracts/aave-v3/atoken.go
//go:generate abigen --abi ./internal/ethereum/contracts/aave-v3/wallet_balance.abi --pkg aavev3contracts --type WalletBalance --out ./internal/ethereum/contracts/aave-v3/wallet_balance.go
func main() {}
