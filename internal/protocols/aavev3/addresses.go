package aavev3

import (
	"math/big"

	ethcommon "github.com/ethereum/go-ethereum/common"
)

// Deployment holds the Aave V3 contract addresses for a specific chain.
type Deployment struct {
	Pool         ethcommon.Address
	DataProvider ethcommon.Address
}

// deployments maps chain ID to Aave V3 deployment addresses.
// Sources: https://github.com/bgd-labs/aave-address-book
var deployments = map[uint64]Deployment{
	// Ethereum Mainnet
	1: {
		Pool:         ethcommon.HexToAddress("0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2"),
		DataProvider: ethcommon.HexToAddress("0x7B4EB56E7CD4b454BA8ff71E4518426c9B8bFe4B"),
	},
	// Optimism
	10: {
		Pool:         ethcommon.HexToAddress("0x794a61358D6845594F94dc1DB02A252b5b4814aD"),
		DataProvider: ethcommon.HexToAddress("0x69FA688f1Dc47d4B5d8029D5a35FB7a548310654"),
	},
	// BSC
	56: {
		Pool:         ethcommon.HexToAddress("0x6807dc923806fE8Fd134338EABCA509979a7e0cB"),
		DataProvider: ethcommon.HexToAddress("0x41585C50524fb8c3899B43D7D797d9486AAc94DB"),
	},
	// Polygon
	137: {
		Pool:         ethcommon.HexToAddress("0x794a61358D6845594F94dc1DB02A252b5b4814aD"),
		DataProvider: ethcommon.HexToAddress("0x69FA688f1Dc47d4B5d8029D5a35FB7a548310654"),
	},
	// ZkSync Era
	324: {
		Pool:         ethcommon.HexToAddress("0x78e30497a3c7527d953c6B1E3541b021A98Ac43c"),
		DataProvider: ethcommon.HexToAddress("0x8A48E34A62fBA5E47f2C8EC51b03BB436E509E6B"),
	},
	// Mantle
	5000: {
		Pool:         ethcommon.HexToAddress("0xCFbFa83332bB1A3154FA4BA4febedf5c94bDA7c0"),
		DataProvider: ethcommon.HexToAddress("0xa99a1dCA4FbA6C4c277Ee756bD98A01C0E521c78"),
	},
	// Base
	8453: {
		Pool:         ethcommon.HexToAddress("0xA238Dd80C259a72e81d7e4664a9801593F98d1c5"),
		DataProvider: ethcommon.HexToAddress("0xd82a47fdebB5bf5329b09441C3DaB4b5df2153Ad"),
	},
	// Arbitrum One
	42161: {
		Pool:         ethcommon.HexToAddress("0x794a61358D6845594F94dc1DB02A252b5b4814aD"),
		DataProvider: ethcommon.HexToAddress("0x69FA688f1Dc47d4B5d8029D5a35FB7a548310654"),
	},
	// Avalanche C-Chain
	43114: {
		Pool:         ethcommon.HexToAddress("0x794a61358D6845594F94dc1DB02A252b5b4814aD"),
		DataProvider: ethcommon.HexToAddress("0x69FA688f1Dc47d4B5d8029D5a35FB7a548310654"),
	},
}

// GetDeployment returns the Aave V3 deployment for the given chain ID, or false
// if Aave V3 is not deployed on that chain.
func GetDeployment(chainID *big.Int) (Deployment, bool) {
	d, ok := deployments[chainID.Uint64()]
	return d, ok
}

// SupportedChainIDs returns all chain IDs where Aave V3 is deployed.
func SupportedChainIDs() []uint64 {
	ids := make([]uint64, 0, len(deployments))
	for id := range deployments {
		ids = append(ids, id)
	}
	return ids
}
