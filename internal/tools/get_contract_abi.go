package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/etherscan"
)

var addressRegex = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

func newGetContractABITool() mcp.Tool {
	return mcp.NewTool("get_contract_abi",
		mcp.WithDescription(
			"Fetch the verified ABI for a smart contract from Etherscan. "+
				"Returns the full ABI JSON which can be used with abi_encode to build "+
				"contract calls or with abi_decode to interpret return data. "+
				"Automatically detects and resolves proxy contracts to return the "+
				"implementation ABI. Supports all EVM chains.",
		),
		mcp.WithString("chain",
			mcp.Description("EVM chain name. One of: Ethereum, BSC, Polygon, Arbitrum, Optimism, Base, Avalanche, Blast, Mantle, Zksync."),
			mcp.DefaultString("Ethereum"),
		),
		mcp.WithString("address",
			mcp.Description("Contract address (0x-prefixed, 42 characters)."),
			mcp.Required(),
		),
	)
}

func handleGetContractABI(esClient *etherscan.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chain := req.GetString("chain", "Ethereum")
		address, err := req.RequireString("address")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if !addressRegex.MatchString(address) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid address format: %q. Expected 0x-prefixed 40-character hex string.", address)), nil
		}

		if _, ok := etherscan.ChainIDs[chain]; !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unsupported chain: %q. Supported: Ethereum, BSC, Polygon, Arbitrum, Optimism, Base, Avalanche, Blast, Mantle, Zksync.", chain)), nil
		}

		// Fetch source code info (includes proxy detection)
		src, err := esClient.GetSourceCode(ctx, chain, address)
		if err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "not verified") || strings.Contains(errMsg, "Contract source code not verified") {
				return mcp.NewToolResultError(fmt.Sprintf("contract at %s is not verified on Etherscan. ABI is only available for verified contracts.", address)), nil
			}
			if strings.Contains(errMsg, "API key required") {
				return mcp.NewToolResultError(errMsg), nil
			}
			return nil, fmt.Errorf("etherscan get source code: %w", err)
		}

		if src.ABI == "" || src.ABI == "Contract source code not verified" {
			return mcp.NewToolResultError(fmt.Sprintf("contract at %s is not verified on Etherscan. ABI is only available for verified contracts.", address)), nil
		}

		var sb strings.Builder
		contractName := src.ContractName
		if contractName == "" {
			contractName = "Unknown"
		}
		truncAddr := address[:6] + "..." + address[len(address)-4:]
		fmt.Fprintf(&sb, "Contract: %s (%s)\n", contractName, truncAddr)
		fmt.Fprintf(&sb, "Chain: %s\n", chain)

		abiJSON := src.ABI

		// Check for proxy and resolve implementation
		if src.Proxy == "1" && src.Implementation != "" {
			fmt.Fprintf(&sb, "Type: Proxy → Implementation: %s\n", src.Implementation)

			implABI, err := esClient.GetContractABI(ctx, chain, src.Implementation)
			if err == nil && implABI != "" {
				abiJSON = implABI
			}
		}

		if src.CompilerVersion != "" {
			fmt.Fprintf(&sb, "Compiler: %s\n", src.CompilerVersion)
		}

		// Parse ABI and extract function signatures
		funcs := extractFunctions(abiJSON)
		if len(funcs) > 0 {
			fmt.Fprintf(&sb, "\nFunctions (%d):\n", len(funcs))
			for _, f := range funcs {
				fmt.Fprintf(&sb, "  - %s\n", f)
			}
		}

		fmt.Fprintf(&sb, "\nABI:\n%s\n", abiJSON)

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// abiEntry is a minimal struct for parsing ABI JSON.
type abiEntry struct {
	Type            string     `json:"type"`
	Name            string     `json:"name"`
	Inputs          []abiParam `json:"inputs"`
	Outputs         []abiParam `json:"outputs"`
	StateMutability string     `json:"stateMutability"`
}

type abiParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func extractFunctions(abiJSON string) []string {
	var entries []abiEntry
	if err := json.Unmarshal([]byte(abiJSON), &entries); err != nil {
		return nil
	}

	var funcs []string
	for _, e := range entries {
		if e.Type != "function" {
			continue
		}
		inputs := make([]string, len(e.Inputs))
		for i, p := range e.Inputs {
			inputs[i] = p.Type
		}
		outputs := make([]string, len(e.Outputs))
		for i, p := range e.Outputs {
			outputs[i] = p.Type
		}

		sig := fmt.Sprintf("%s(%s)", e.Name, strings.Join(inputs, ","))
		if len(outputs) > 0 {
			sig += " → " + strings.Join(outputs, ",")
		}
		funcs = append(funcs, sig)
	}
	return funcs
}
