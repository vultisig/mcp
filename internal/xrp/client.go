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
	Transaction string `json:"transaction,omitempty"`
}

type rpcResponse struct {
	Result rpcResult `json:"result"`
}

type rpcResult struct {
	Status       string      `json:"status,omitempty"`
	AccountData  accountData `json:"account_data,omitempty"`
	LedgerIndex  json.Number `json:"ledger_index,omitempty"`
	Drops        feeDrops    `json:"drops,omitempty"`
	Error        string      `json:"error,omitempty"`
	ErrorMessage string      `json:"error_message,omitempty"`
	// Fields for "tx" method response.
	Validated bool       `json:"validated,omitempty"`
	Fee       string     `json:"Fee,omitempty"`
	Meta      txMeta     `json:"meta,omitempty"`
	Hash      string     `json:"hash,omitempty"`
}

type txMeta struct {
	TransactionResult string `json:"TransactionResult,omitempty"`
}

type accountData struct {
	Account  string      `json:"Account"`
	Balance  string      `json:"Balance"`
	Sequence json.Number `json:"Sequence"`
}

type feeDrops struct {
	BaseFee string `json:"base_fee"`
}

// ErrTxNotFound is returned when a transaction hash cannot be found.
var ErrTxNotFound = fmt.Errorf("transaction not found")

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

	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()

	var rpcResp rpcResponse
	err = dec.Decode(&rpcResp)
	if err != nil {
		return nil, fmt.Errorf("xrp: decode response: %w", err)
	}

	if rpcResp.Result.Error != "" {
		if rpcResp.Result.Error == "txnNotFound" {
			return nil, ErrTxNotFound
		}
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

	seq, err := strconv.ParseUint(result.AccountData.Sequence.String(), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("xrp: parse sequence: %w", err)
	}

	return &AccountInfo{
		Sequence: uint32(seq),
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

	idx, err := strconv.ParseUint(result.LedgerIndex.String(), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("xrp: parse ledger index: %w", err)
	}
	return uint32(idx), nil
}

func (c *Client) GetBaseFee(ctx context.Context) (uint64, error) {
	result, err := c.do(ctx, "fee", rpcParam{})
	if err != nil {
		return 0, fmt.Errorf("get base fee: %w", err)
	}

	if result.Drops.BaseFee == "" {
		return 0, fmt.Errorf("xrp: fee response missing drops.base_fee")
	}

	fee, err := strconv.ParseUint(result.Drops.BaseFee, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("xrp: parse base fee %q: %w", result.Drops.BaseFee, err)
	}

	if fee < 12 {
		fee = 12
	}

	return fee, nil
}

// TxStatus holds XRP transaction confirmation info.
type TxStatus struct {
	Validated bool
	Fee       string
	Result    string // e.g. "tesSUCCESS"
	Ledger    int64
}

// GetTransactionStatus fetches the status of an XRP transaction by hash.
func (c *Client) GetTransactionStatus(ctx context.Context, txHash string) (*TxStatus, error) {
	result, err := c.do(ctx, "tx", rpcParam{
		Transaction: txHash,
	})
	if err != nil {
		return nil, err
	}

	var ledger int64
	if result.LedgerIndex.String() != "" {
		l, err := strconv.ParseInt(result.LedgerIndex.String(), 10, 64)
		if err == nil {
			ledger = l
		}
	}

	return &TxStatus{
		Validated: result.Validated,
		Fee:       result.Fee,
		Result:    result.Meta.TransactionResult,
		Ledger:    ledger,
	}, nil
}
