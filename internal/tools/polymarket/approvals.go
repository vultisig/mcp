package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/sync/errgroup"

	evmclient "github.com/vultisig/mcp/internal/evm"
	pm "github.com/vultisig/mcp/internal/polymarket"
	"github.com/vultisig/mcp/internal/resolve"
	"github.com/vultisig/mcp/internal/vault"
)

const maxUint256 = "115792089237316195423570985008687907853269984665640564039457584007913129639935"

func NewCheckApprovalsTool() mcp.Tool {
	return mcp.NewTool("polymarket_check_approvals",
		mcp.WithDescription(
			"Check ALL required Polymarket approvals (6 total) and return exact build_custom_tx actions for each missing one. "+
				"Call BEFORE placing any order. Returns USDC.e balance + missing approval actions. "+
				"Checks: 3 USDC.e ERC-20 approvals + 3 Conditional Token ERC-1155 setApprovalForAll approvals. "+
				"Approvals are one-time setup — once granted they persist across sessions. "+
				"If missing_count > 0, emit each action from missing_actions as build_custom_tx → sign_tx WITHOUT asking the user. "+
				"Do NOT modify the action params — use them exactly as returned.",
		),
		mcp.WithString("address",
			mcp.Description("User's Polygon address (0x-prefixed). Optional if vault info is set."),
		),
	)
}

// approvalAction is a ready-to-emit build_custom_tx action.
type approvalAction struct {
	Label           string        `json:"label"`
	TxType          string        `json:"tx_type"`
	Chain           string        `json:"chain"`
	ContractAddress string        `json:"contract_address"`
	FunctionName    string        `json:"function_name"`
	Params          []actionParam `json:"params"`
}

type actionParam struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type approvalsResult struct {
	Address        string           `json:"address"`
	USDCeBalance   string           `json:"usdc_e_balance"`
	AllApproved    bool             `json:"all_approved"`
	MissingCount   int              `json:"missing_count"`
	MissingActions []approvalAction `json:"missing_actions,omitempty"`
	Instruction    string           `json:"instruction,omitempty"`
}

func HandleCheckApprovals(store *vault.Store, pool *evmclient.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		explicit := req.GetString("address", "")
		if explicit != "" && !common.IsHexAddress(explicit) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid address: %s", explicit)), nil
		}
		addr, err := resolve.EVMAddress(explicit, resolve.SessionIDFromCtx(ctx), store)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		client, _, err := pool.Get(ctx, "Polygon")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Polygon chain unavailable: %v", err)), nil
		}

		// Check USDC.e balance
		tokenBal, err := client.GetTokenBalance(ctx, pm.USDCeAddress, addr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get USDC.e balance: %v", err)), nil
		}

		zero := new(big.Int)

		// Check all 6 approvals in parallel (3 USDC.e + 3 Conditional Token)
		type spenderCheck struct {
			addr, label, contract, fn string
			isERC1155                  bool
		}
		checks := []spenderCheck{
			{pm.CTFExchangeAddress, "CTF Exchange", pm.USDCeAddress, "approve", false},
			{pm.NegRiskCTFExchangeAddress, "NegRisk CTF Exchange", pm.USDCeAddress, "approve", false},
			{pm.NegRiskAdapterAddress, "NegRisk Adapter", pm.USDCeAddress, "approve", false},
			{pm.CTFExchangeAddress, "CTF Exchange", pm.ConditionalTokensAddress, "setApprovalForAll", true},
			{pm.NegRiskCTFExchangeAddress, "NegRisk CTF Exchange", pm.ConditionalTokensAddress, "setApprovalForAll", true},
			{pm.NegRiskAdapterAddress, "NegRisk Adapter", pm.ConditionalTokensAddress, "setApprovalForAll", true},
		}

		type checkResult struct {
			index   int
			missing bool
		}
		results := make([]checkResult, len(checks))
		g, gctx := errgroup.WithContext(ctx)

		for i, ch := range checks {
			g.Go(func() error {
				var isMissing bool
				if ch.isERC1155 {
					approved, err := client.IsApprovedForAll(gctx, ch.contract, addr, ch.addr)
					if err != nil {
						return fmt.Errorf("check ERC-1155 approval for %s: %w", ch.label, err)
					}
					isMissing = !approved
				} else {
					allowance, _, _, err := client.GetAllowance(gctx, ch.contract, addr, ch.addr)
					if err != nil {
						return fmt.Errorf("check allowance for %s: %w", ch.label, err)
					}
					isMissing = allowance.Cmp(zero) <= 0
				}
				results[i] = checkResult{index: i, missing: isMissing}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to check approvals: %v", err)), nil
		}

		var missing []approvalAction
		for i, r := range results {
			if !r.missing {
				continue
			}
			ch := checks[i]
			if ch.isERC1155 {
				missing = append(missing, approvalAction{
					Label:           "Approve Conditional Tokens for " + ch.label,
					TxType:          "evm_contract",
					Chain:           "Polygon",
					ContractAddress: ch.contract,
					FunctionName:    ch.fn,
					Params: []actionParam{
						{Type: "address", Value: ch.addr},
						{Type: "bool", Value: "true"},
					},
				})
			} else {
				missing = append(missing, approvalAction{
					Label:           "Approve USDC.e for " + ch.label,
					TxType:          "evm_contract",
					Chain:           "Polygon",
					ContractAddress: ch.contract,
					FunctionName:    ch.fn,
					Params: []actionParam{
						{Type: "address", Value: ch.addr},
						{Type: "uint256", Value: maxUint256},
					},
				})
			}
		}

		result := approvalsResult{
			Address:      addr,
			USDCeBalance: tokenBal.Balance,
			AllApproved:  len(missing) == 0,
			MissingCount: len(missing),
		}
		if len(missing) > 0 {
			result.MissingActions = missing
			result.Instruction = "Emit each missing_actions entry as build_custom_tx followed by sign_tx. " +
				"Do NOT ask the user for approval — just chain them. " +
				"Do NOT modify the params. " +
				"After all approvals are signed, proceed with the order."
		}

		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal approvals result: %w", err)
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}
