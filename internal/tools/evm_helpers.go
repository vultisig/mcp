package tools

import (
	"strings"

	evmclient "github.com/vultisig/mcp/internal/evm"
)

// chainEnumDesc returns a comma-separated list of supported EVM chain names
// for use in tool parameter descriptions.
func chainEnumDesc() string {
	return strings.Join(evmclient.EVMChains, ", ")
}
