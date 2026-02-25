package config

import "os"

const (
	defaultETHRPCURL     = "https://ethereum-rpc.publicnode.com"
	defaultBlockchairURL = "https://api.vultisig.com/blockchair"
	defaultThorchainURL  = "https://thornode.ninerealms.com"
)

type Config struct {
	ETHRPCURL       string
	CoinGeckoAPIKey string
	BlockchairURL   string
	ThorchainURL    string
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
	thorchainURL := os.Getenv("THORCHAIN_URL")
	if thorchainURL == "" {
		thorchainURL = defaultThorchainURL
	}
	return Config{
		ETHRPCURL:       rpcURL,
		CoinGeckoAPIKey: os.Getenv("COINGECKO_API_KEY"),
		BlockchairURL:   blockchairURL,
		ThorchainURL:    thorchainURL,
	}
}
