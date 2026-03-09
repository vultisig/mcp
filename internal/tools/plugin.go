package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/vultisig/mcp/internal/verifier"
)

const billingPluginID = "vultisig-fees-feee"

func newGetRecipeSchemaTool() mcp.Tool {
	return mcp.NewTool("get_recipe_schema",
		mcp.WithDescription(
			"Fetch the configuration schema and examples for a plugin. "+
				"Returns supported resources, parameter constraints, and configuration examples. "+
				"Call this before suggest_policy to understand what the plugin accepts.",
		),
		mcp.WithString("plugin_id",
			mcp.Description("Plugin identifier (e.g. 'vultisig-dca')"),
			mcp.Required(),
		),
	)
}

func handleGetRecipeSchema(vc *verifier.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pluginID, err := req.RequireString("plugin_id")
		if err != nil {
			return mcp.NewToolResultError("plugin_id is required"), nil
		}

		schema, err := vc.GetRecipeSchema(ctx, pluginID)
		if err != nil {
			return mcp.NewToolResultError("get recipe schema: " + err.Error()), nil
		}

		data, err := json.Marshal(schema)
		if err != nil {
			return mcp.NewToolResultError("marshal schema: " + err.Error()), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func newSuggestPolicyTool() mcp.Tool {
	return mcp.NewTool("suggest_policy",
		mcp.WithDescription(
			"Validate a plugin configuration and get the resulting policy rules. "+
				"Call after get_recipe_schema to confirm the configuration is valid and "+
				"to obtain the rules that will govern plugin execution. "+
				"Pass configuration as a JSON-encoded string.",
		),
		mcp.WithString("plugin_id",
			mcp.Description("Plugin identifier"),
			mcp.Required(),
		),
		mcp.WithString("configuration",
			mcp.Description("Plugin configuration as a JSON object string matching the recipe schema"),
			mcp.Required(),
		),
	)
}

func handleSuggestPolicy(vc *verifier.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pluginID, err := req.RequireString("plugin_id")
		if err != nil {
			return mcp.NewToolResultError("plugin_id is required"), nil
		}

		cfgStr, err := req.RequireString("configuration")
		if err != nil {
			return mcp.NewToolResultError("configuration is required"), nil
		}
		var cfg map[string]any
		if err := json.Unmarshal([]byte(cfgStr), &cfg); err != nil {
			return mcp.NewToolResultError("configuration must be valid JSON: " + err.Error()), nil
		}

		policy, err := vc.GetPolicySuggest(ctx, pluginID, cfg)
		if err != nil {
			return mcp.NewToolResultError("suggest policy: " + err.Error()), nil
		}

		result := map[string]any{
			"plugin_id":      pluginID,
			"configuration":  cfg,
			"policy_suggest": policy,
		}
		data, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError("marshal result: " + err.Error()), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func newCheckPluginInstalledTool() mcp.Tool {
	return mcp.NewTool("check_plugin_installed",
		mcp.WithDescription(
			"Check whether a plugin is installed for a vault. "+
				"Uses the vault's ECDSA public key to identify the user.",
		),
		mcp.WithString("plugin_id",
			mcp.Description("Plugin identifier to check"),
			mcp.Required(),
		),
		mcp.WithString("public_key",
			mcp.Description("Vault ECDSA public key (hex) identifying the user"),
			mcp.Required(),
		),
	)
}

func handleCheckPluginInstalled(vc *verifier.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pluginID, err := req.RequireString("plugin_id")
		if err != nil {
			return mcp.NewToolResultError("plugin_id is required"), nil
		}

		publicKey, err := req.RequireString("public_key")
		if err != nil {
			return mcp.NewToolResultError("public_key is required"), nil
		}

		installed, err := vc.IsPluginInstalled(ctx, publicKey, pluginID)
		if err != nil {
			return mcp.NewToolResultError("check plugin installed: " + err.Error()), nil
		}

		data, _ := json.Marshal(map[string]any{
			"installed": installed,
			"plugin_id": pluginID,
		})
		return mcp.NewToolResultText(string(data)), nil
	}
}

func newCheckBillingStatusTool() mcp.Tool {
	return mcp.NewTool("check_billing_status",
		mcp.WithDescription(
			"Check whether a vault's billing is active (free trial or billing plugin installed). "+
				"Uses the vault's ECDSA public key to identify the user.",
		),
		mcp.WithString("public_key",
			mcp.Description("Vault ECDSA public key (hex) identifying the user"),
			mcp.Required(),
		),
	)
}

func handleCheckBillingStatus(vc *verifier.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		publicKey, err := req.RequireString("public_key")
		if err != nil {
			return mcp.NewToolResultError("public_key is required"), nil
		}

		feeStatus, err := vc.GetFeeStatus(ctx, publicKey)
		if err != nil {
			return mcp.NewToolResultError("check billing status: " + err.Error()), nil
		}

		if feeStatus.IsTrialActive {
			data, _ := json.Marshal(map[string]any{
				"billing_ok":      true,
				"is_trial_active": true,
				"trial_remaining": feeStatus.TrialRemaining,
			})
			return mcp.NewToolResultText(string(data)), nil
		}

		billingInstalled, err := vc.IsPluginInstalled(ctx, publicKey, billingPluginID)
		if err != nil {
			return mcp.NewToolResultError("check billing app: " + err.Error()), nil
		}

		data, _ := json.Marshal(map[string]any{
			"billing_ok":        billingInstalled,
			"is_trial_active":   false,
			"billing_installed": billingInstalled,
			"billing_plugin_id": billingPluginID,
		})
		return mcp.NewToolResultText(string(data)), nil
	}
}
