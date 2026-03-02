package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"

	"github.com/vultisig/mcp/internal/evm"
)

// EVMRPCConfig holds RPC endpoint URLs for all supported EVM chains.
// Each field maps to an environment variable named EVM_{CHAIN}_URL
// (e.g. EVM_ETHEREUM_URL, EVM_BSC_URL, EVM_POLYGON_URL, …).
// If a variable is unset, the public-node default is used.
type EVMRPCConfig struct {
	Ethereum  RPCItem
	BSC       RPCItem
	Polygon   RPCItem
	Avalanche RPCItem
	Arbitrum  RPCItem
	Optimism  RPCItem
	Base      RPCItem
	Blast     RPCItem
	Mantle    RPCItem
	Zksync    RPCItem
}

type RPCItem struct {
	URL string
}

type Config struct {
	EVM           EVMRPCConfig
	BlockchairURL string `envconfig:"BLOCKCHAIR_API_URL" default:"https://api.vultisig.com/blockchair"`
	ThorchainURL  string `envconfig:"THORCHAIN_URL" default:"https://thornode.ninerealms.com"`
	MayachainURL  string `envconfig:"MAYACHAIN_URL" default:"https://mayanode.mayachain.info"`
	SolanaRPCURL  string `envconfig:"SOLANA_RPC_URL" default:"https://api.mainnet-beta.solana.com"`
	JupiterAPIURL string `envconfig:"JUPITER_API_URL" default:"https://api.jup.ag"`
	XrpRpcURL     string `envconfig:"XRP_RPC_URL" default:"https://s1.ripple.com:51234"`
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
