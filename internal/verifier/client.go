package verifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

var validPluginID = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// validPublicKey matches a compressed secp256k1 ECDSA public key: 33 bytes = 66 hex chars.
var validPublicKey = regexp.MustCompile(`^[0-9a-fA-F]{66}$`)

// Client calls the verifier service for plugin-related operations.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new verifier client.
// apiKey is used for server-to-server calls (X-Service-Key header).
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// HasAPIKey reports whether the client has a service API key configured.
// Tools that require X-Service-Key authentication should not be registered without one.
func (c *Client) HasAPIKey() bool {
	return c.apiKey != ""
}

// RecipeSchema represents a plugin's recipe specification.
type RecipeSchema struct {
	SupportedResources   []SupportedResource `json:"supported_resources"`
	Configuration        map[string]any      `json:"configuration,omitempty"`
	ConfigurationExample []map[string]any    `json:"configuration_example,omitempty"`
}

// SupportedResource represents a supported resource in a recipe schema.
type SupportedResource struct {
	ResourcePath         ResourcePath          `json:"resource_path"`
	ParameterConstraints []ParameterConstraint `json:"parameter_constraints"`
}

// ResourcePath identifies a resource type.
type ResourcePath struct {
	FunctionID   string `json:"function_id"`
	ResourceType string `json:"resource_type"`
}

// ParameterConstraint defines constraints on a parameter.
type ParameterConstraint struct {
	ParameterName string     `json:"parameterName"`
	Constraint    Constraint `json:"constraint"`
}

// Constraint defines the constraint value.
type Constraint struct {
	Type       string `json:"type,omitempty"`
	FixedValue string `json:"fixedValue,omitempty"`
	Required   bool   `json:"required,omitempty"`
}

type recipeSpecResponse struct {
	Code int          `json:"code"`
	Data RecipeSchema `json:"data"`
}

// GetRecipeSchema fetches the recipe specification for a plugin.
func (c *Client) GetRecipeSchema(ctx context.Context, pluginID string) (*RecipeSchema, error) {
	if !validPluginID.MatchString(pluginID) {
		return nil, fmt.Errorf("invalid plugin ID: %q", pluginID)
	}

	url := fmt.Sprintf("%s/plugins/%s/recipe-specification", c.baseURL, pluginID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp recipeSpecResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &apiResp.Data, nil
}

// PolicySuggest represents the policy suggestion from the plugin.
type PolicySuggest struct {
	Rules           []Rule `json:"rules"`
	RateLimitWindow int    `json:"rateLimitWindow,omitempty"`
	MaxTxsPerWindow int    `json:"maxTxsPerWindow,omitempty"`
}

// Rule represents a policy rule.
type Rule struct {
	Resource             string                `json:"resource"`
	Effect               string                `json:"effect,omitempty"`
	Target               *Target               `json:"target,omitempty"`
	ParameterConstraints []ParameterConstraint `json:"parameterConstraints,omitempty"`
}

// Target represents a rule target.
type Target struct {
	Address string `json:"address,omitempty"`
}

type policySuggestResponse struct {
	Data PolicySuggest `json:"data"`
}

type suggestRequest struct {
	Configuration map[string]any `json:"configuration"`
}

// GetPolicySuggest calls the plugin's suggest endpoint to build a policy.
func (c *Client) GetPolicySuggest(ctx context.Context, pluginID string, configuration map[string]any) (*PolicySuggest, error) {
	if !validPluginID.MatchString(pluginID) {
		return nil, fmt.Errorf("invalid plugin ID: %q", pluginID)
	}

	url := fmt.Sprintf("%s/plugins/%s/recipe-specification/suggest", c.baseURL, pluginID)

	body, err := json.Marshal(suggestRequest{Configuration: configuration})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp policySuggestResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &apiResp.Data, nil
}

// FeeStatus represents the billing status for a user.
type FeeStatus struct {
	IsTrialActive  bool  `json:"is_trial_active"`
	TrialRemaining int64 `json:"trial_remaining"`
	UnpaidAmount   int64 `json:"unpaid_amount"`
}

// GetFeeStatus fetches billing status for a vault identified by its ECDSA public key.
// Calls GET /service/fee/status?public_key={hex} with X-Service-Key auth.
func (c *Client) GetFeeStatus(ctx context.Context, publicKey string) (*FeeStatus, error) {
	if !validPublicKey.MatchString(publicKey) {
		return nil, fmt.Errorf("invalid public key: must be 66 hex characters")
	}
	reqURL := fmt.Sprintf("%s/service/fee/status?public_key=%s", c.baseURL, url.QueryEscape(publicKey))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("X-Service-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var status FeeStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &status, nil
}

// installedPluginsResponse is the response from GET /plugins/installed.
type installedPluginsResponse struct {
	Code int `json:"code"`
	Data struct {
		Plugins []struct {
			ID string `json:"id"`
		} `json:"plugins"`
	} `json:"data"`
}

// IsPluginInstalled checks if a plugin is installed for the vault identified by publicKey.
// Calls GET /service/plugins/installed?public_key={hex} with X-Service-Key auth.
func (c *Client) IsPluginInstalled(ctx context.Context, publicKey, pluginID string) (bool, error) {
	if !validPublicKey.MatchString(publicKey) {
		return false, fmt.Errorf("invalid public key: must be 66 hex characters")
	}
	reqURL := fmt.Sprintf("%s/service/plugins/installed?public_key=%s", c.baseURL, url.QueryEscape(publicKey))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false, fmt.Errorf("create request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("X-Service-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp installedPluginsResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return false, fmt.Errorf("decode response: %w", err)
	}

	for _, p := range apiResp.Data.Plugins {
		if p.ID == pluginID {
			return true, nil
		}
	}

	return false, nil
}
