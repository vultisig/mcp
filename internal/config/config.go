package config

import "os"

const defaultETHRPCURL = "https://ethereum-rpc.publicnode.com"

type Config struct {
	ETHRPCURL string
}

func Load() Config {
	rpcURL := os.Getenv("ETH_RPC_URL")
	if rpcURL == "" {
		rpcURL = defaultETHRPCURL
	}
	return Config{
		ETHRPCURL: rpcURL,
	}
}
