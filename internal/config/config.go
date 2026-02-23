package config

import "os"

const (
	defaultETHRPCURL    = "https://ethereum-rpc.publicnode.com"
	defaultBlockchairURL = "https://api.vultisig.com/blockchair"
)

type Config struct {
	ETHRPCURL       string
	CoinGeckoAPIKey string
	BlockchairURL   string
}

func Load() Config {
	rpcURL := os.Getenv("ETH_RPC_URL")
	if rpcURL == "" {
		rpcURL = defaultETHRPCURL
	}
	blockchairURL := os.Getenv("BLOCKCHAIR_API_URL")
	if blockchairURL == "" {
		blockchairURL = defaultBlockchairURL
	}
	return Config{
		ETHRPCURL:       rpcURL,
		CoinGeckoAPIKey: os.Getenv("COINGECKO_API_KEY"),
		BlockchairURL:   blockchairURL,
	}
}
