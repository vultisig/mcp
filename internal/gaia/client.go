package gaia

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcutil/bech32"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type AccountInfo struct {
	AccountNumber string
	Sequence      string
}

type TxStatus struct {
	Height  string
	Code    int
	GasUsed string
	TxHash  string
}

var ErrNotFound = fmt.Errorf("not found")

func (c *Client) get(ctx context.Context, path string, out any) error {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("gaia: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gaia: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gaia: unexpected status %d for %s", resp.StatusCode, path)
	}

	err = json.NewDecoder(resp.Body).Decode(out)
	if err != nil {
		return fmt.Errorf("gaia: decode response: %w", err)
	}

	return nil
}

type balanceResponse struct {
	Balance struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"balance"`
}

func (c *Client) GetBalance(ctx context.Context, address string) (string, error) {
	var resp balanceResponse
	path := fmt.Sprintf("/cosmos/bank/v1beta1/balances/%s/by_denom?denom=uatom", url.PathEscape(address))
	err := c.get(ctx, path, &resp)
	if err != nil {
		return "", fmt.Errorf("get balance: %w", err)
	}
	if resp.Balance.Amount == "" {
		return "0", nil
	}
	return resp.Balance.Amount, nil
}

type accountResponse struct {
	Account struct {
		Type          string `json:"@type"`
		AccountNumber string `json:"account_number"`
		Sequence      string `json:"sequence"`
	} `json:"account"`
}

func (c *Client) GetAccount(ctx context.Context, address string) (*AccountInfo, error) {
	var resp accountResponse
	path := fmt.Sprintf("/cosmos/auth/v1beta1/accounts/%s", url.PathEscape(address))
	err := c.get(ctx, path, &resp)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}
	return &AccountInfo{
		AccountNumber: resp.Account.AccountNumber,
		Sequence:      resp.Account.Sequence,
	}, nil
}

type txResponse struct {
	TxResponse struct {
		TxHash  string `json:"txhash"`
		Height  string `json:"height"`
		Code    int    `json:"code"`
		GasUsed string `json:"gas_used"`
	} `json:"tx_response"`
}

func (c *Client) GetTransactionStatus(ctx context.Context, txHash string) (*TxStatus, error) {
	var resp txResponse
	path := fmt.Sprintf("/cosmos/tx/v1beta1/txs/%s", url.PathEscape(txHash))
	err := c.get(ctx, path, &resp)
	if err != nil {
		return nil, err
	}
	return &TxStatus{
		TxHash:  resp.TxResponse.TxHash,
		Height:  resp.TxResponse.Height,
		Code:    resp.TxResponse.Code,
		GasUsed: resp.TxResponse.GasUsed,
	}, nil
}

func ValidateAddress(address string) error {
	hrp, _, err := bech32.Decode(address)
	if err != nil {
		return fmt.Errorf("invalid bech32 address: %w", err)
	}
	if hrp != "cosmos" {
		return fmt.Errorf("expected bech32 prefix \"cosmos\", got %q", hrp)
	}
	return nil
}

func FormatUATOM(uatom *big.Int) string {
	if uatom.Sign() == 0 {
		return "0.000000"
	}
	neg := uatom.Sign() < 0
	abs := new(big.Int).Abs(uatom)
	divisor := big.NewInt(1_000_000)
	whole := new(big.Int).Div(abs, divisor)
	remainder := new(big.Int).Mod(abs, divisor)
	if neg {
		return fmt.Sprintf("-%d.%06d", whole, remainder)
	}
	return fmt.Sprintf("%d.%06d", whole, remainder)
}
