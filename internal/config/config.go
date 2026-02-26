package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"

	"github.com/vultisig/mcp/internal/evm"
)

type Config struct {
	EVM             EVMRPCConfig
	CoinGeckoAPIKey string `envconfig:"COINGECKO_API_KEY"`
	BlockchairURL   string `envconfig:"BLOCKCHAIR_API_URL" default:"https://api.vultisig.com/blockchair"`
	ThorchainURL    string `envconfig:"THORCHAIN_URL" default:"https://thornode.ninerealms.com"`
}

// ToURLMap converts the EVM RPC config to a chain-name → URL map,
// falling back to the built-in defaults for any URL that is empty.
func (e EVMRPCConfig) ToURLMap() map[string]string {
	m := map[string]string{
		"Ethereum":  e.Ethereum.URL,
		"BSC":       e.BSC.URL,
		"Polygon":   e.Polygon.URL,
		"Avalanche": e.Avalanche.URL,
		"Arbitrum":  e.Arbitrum.URL,
		"Optimism":  e.Optimism.URL,
		"Base":      e.Base.URL,
		"Blast":     e.Blast.URL,
		"Mantle":    e.Mantle.URL,
		"Zksync":    e.Zksync.URL,
	}
	defaults := evm.DefaultRPCURLs()
	for chain, url := range m {
		if url == "" {
			m[chain] = defaults[chain]
		}
	}
	return m
}

func Load() (Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("process env config: %w", err)
	}
	return cfg, nil
}
