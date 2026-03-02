package xrp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	rpcURL     string
	httpClient *http.Client
}

func NewClient(rpcURL string) *Client {
	return &Client{
		rpcURL: rpcURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type rpcRequest struct {
	Method string     `json:"method"`
	Params []rpcParam `json:"params"`
}

type rpcParam struct {
	Account     string `json:"account,omitempty"`
	LedgerIndex string `json:"ledger_index,omitempty"`
	Strict      bool   `json:"strict,omitempty"`
}

type rpcResponse struct {
	Result rpcResult `json:"result"`
}

type rpcResult struct {
	Status       string      `json:"status,omitempty"`
	AccountData  accountData `json:"account_data,omitempty"`
	LedgerIndex  interface{} `json:"ledger_index,omitempty"`
	Info         serverInfo  `json:"info,omitempty"`
	Error        string      `json:"error,omitempty"`
	ErrorMessage string      `json:"error_message,omitempty"`
}

type accountData struct {
	Account  string      `json:"Account"`
	Balance  string      `json:"Balance"`
	Sequence interface{} `json:"Sequence"`
}

type serverInfo struct {
	ValidatedLedger validatedLedger `json:"validated_ledger,omitempty"`
	BaseFee         interface{}     `json:"base_fee,omitempty"`
}

type validatedLedger struct {
	Seq interface{} `json:"seq,omitempty"`
}

func (c *Client) do(ctx context.Context, method string, param rpcParam) (*rpcResult, error) {
	body, err := json.Marshal(rpcRequest{
		Method: method,
		Params: []rpcParam{param},
	})
	if err != nil {
		return nil, fmt.Errorf("xrp: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("xrp: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xrp: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("xrp: unexpected status %d", resp.StatusCode)
	}

	var rpcResp rpcResponse
	err = json.NewDecoder(resp.Body).Decode(&rpcResp)
	if err != nil {
		return nil, fmt.Errorf("xrp: decode response: %w", err)
	}

	if rpcResp.Result.Error != "" {
		return nil, fmt.Errorf("xrp: %s — %s", rpcResp.Result.Error, rpcResp.Result.ErrorMessage)
	}

	return &rpcResp.Result, nil
}

type AccountInfo struct {
	Sequence uint32
	Balance  string
}

func (c *Client) GetAccountInfo(ctx context.Context, address string) (*AccountInfo, error) {
	result, err := c.do(ctx, "account_info", rpcParam{
		Account:     address,
		Strict:      true,
		LedgerIndex: "validated",
	})
	if err != nil {
		return nil, fmt.Errorf("get account info: %w", err)
	}

	var sequence uint32
	switch v := result.AccountData.Sequence.(type) {
	case float64:
		sequence = uint32(v)
	case string:
		seq, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("xrp: parse sequence: %w", err)
		}
		sequence = uint32(seq)
	default:
		return nil, fmt.Errorf("xrp: unexpected sequence type: %T", v)
	}

	return &AccountInfo{
		Sequence: sequence,
		Balance:  result.AccountData.Balance,
	}, nil
}

func (c *Client) GetCurrentLedger(ctx context.Context) (uint32, error) {
	result, err := c.do(ctx, "ledger", rpcParam{
		LedgerIndex: "validated",
	})
	if err != nil {
		return 0, fmt.Errorf("get current ledger: %w", err)
	}

	switch v := result.LedgerIndex.(type) {
	case float64:
		return uint32(v), nil
	case string:
		idx, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("xrp: parse ledger index: %w", err)
		}
		return uint32(idx), nil
	default:
		return 0, fmt.Errorf("xrp: unexpected ledger index type: %T", v)
	}
}

func (c *Client) GetBaseFee(ctx context.Context) (uint64, error) {
	result, err := c.do(ctx, "server_info", rpcParam{})
	if err != nil {
		return 0, fmt.Errorf("get base fee: %w", err)
	}

	var baseFee uint64 = 12
	if result.Info.BaseFee != nil {
		switch v := result.Info.BaseFee.(type) {
		case float64:
			baseFee = uint64(v)
		case string:
			fee, err := strconv.ParseUint(v, 10, 64)
			if err == nil {
				baseFee = fee
			}
		}
	}

	if baseFee < 12 {
		baseFee = 12
	}

	return baseFee, nil
}
