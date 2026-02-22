package config

import "os"

const defaultETHRPCURL = "https://ethereum-rpc.publicnode.com"

type Config struct {
	ETHRPCURL        string
	CoinGeckoAPIKey  string
}

func Load() Config {
	rpcURL := os.Getenv("ETH_RPC_URL")
	if rpcURL == "" {
		rpcURL = defaultETHRPCURL
	}
	return Config{
		ETHRPCURL:       rpcURL,
		CoinGeckoAPIKey: os.Getenv("COINGECKO_API_KEY"),
	}
}
