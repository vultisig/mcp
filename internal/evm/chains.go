package evm

import "math/big"

// EVMChains is the ordered list of supported EVM chain names.
// These names match the values used in get_address and vultisig-go.
var EVMChains = []string{
	"Ethereum",
	"BSC",
	"Polygon",
	"Avalanche",
	"Arbitrum",
	"Optimism",
	"Base",
	"Blast",
	"Mantle",
	"Zksync",
}

type chainConfig struct {
	defaultRPCURL string
	chainID       int64
	ticker        string
}

var chainDefaults = map[string]chainConfig{
	"Ethereum": {
		defaultRPCURL: "https://ethereum-rpc.publicnode.com",
		chainID:       1,
		ticker:        "ETH",
	},
	"BSC": {
		defaultRPCURL: "https://bsc-rpc.publicnode.com",
		chainID:       56,
		ticker:        "BNB",
	},
	"Polygon": {
		defaultRPCURL: "https://polygon-bor-rpc.publicnode.com",
		chainID:       137,
		ticker:        "POL",
	},
	"Avalanche": {
		defaultRPCURL: "https://avalanche-c-chain-rpc.publicnode.com",
		chainID:       43114,
		ticker:        "AVAX",
	},
	"Arbitrum": {
		defaultRPCURL: "https://arbitrum-one-rpc.publicnode.com",
		chainID:       42161,
		ticker:        "ETH",
	},
	"Optimism": {
		defaultRPCURL: "https://optimism-rpc.publicnode.com",
		chainID:       10,
		ticker:        "ETH",
	},
	"Base": {
		defaultRPCURL: "https://base-rpc.publicnode.com",
		chainID:       8453,
		ticker:        "ETH",
	},
	"Blast": {
		defaultRPCURL: "https://blast-rpc.publicnode.com",
		chainID:       81457,
		ticker:        "ETH",
	},
	"Mantle": {
		defaultRPCURL: "https://mantle-rpc.publicnode.com",
		chainID:       5000,
		ticker:        "MNT",
	},
	"Zksync": {
		defaultRPCURL: "https://mainnet.era.zksync.io",
		chainID:       324,
		ticker:        "ETH",
	},
}

// DefaultRPCURLs returns the default RPC URL map for all supported EVM chains.
func DefaultRPCURLs() map[string]string {
	m := make(map[string]string, len(chainDefaults))
	for name, cfg := range chainDefaults {
		m[name] = cfg.defaultRPCURL
	}
	return m
}

// ChainIDByName returns the chain ID for a known EVM chain name.
func ChainIDByName(chainName string) (*big.Int, bool) {
	cfg, ok := chainDefaults[chainName]
	if !ok {
		return nil, false
	}
	return big.NewInt(cfg.chainID), true
}

// NativeTicker returns the native coin ticker symbol for a chain.
func NativeTicker(chainName string) string {
	cfg, ok := chainDefaults[chainName]
	if !ok {
		return "ETH"
	}
	return cfg.ticker
}
