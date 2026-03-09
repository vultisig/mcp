package tron

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcutil/base58"
)

const ABIWordHexLen = 64

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type AccountInfo struct {
	Address string `json:"address"`
	Balance int64  `json:"balance"`
}

type AccountResource struct {
	FreeNetUsed  int64 `json:"freeNetUsed"`
	FreeNetLimit int64 `json:"freeNetLimit"`
	EnergyUsed   int64 `json:"EnergyUsed"`
	EnergyLimit  int64 `json:"EnergyLimit"`
	NetUsed      int64 `json:"NetUsed"`
	NetLimit     int64 `json:"NetLimit"`
}

type TxInfo struct {
	ID             string    `json:"id"`
	BlockNumber    int64     `json:"blockNumber"`
	Fee            int64     `json:"fee"`
	Result         string    `json:"result"`
	ContractResult []string  `json:"contractResult"`
	Receipt        TxReceipt `json:"receipt"`
}

type TxReceipt struct {
	Result           string `json:"result"`
	EnergyUsageTotal int64  `json:"energy_usage_total"`
	NetUsage         int64  `json:"net_usage"`
}

type ConstantResult struct {
	Result         map[string]any `json:"result"`
	ConstantResult []string       `json:"constant_result"`
	EnergyUsed     int64          `json:"energy_used"`
}

func (c *Client) do(ctx context.Context, path string, reqBody any) (json.RawMessage, error) {
	var bodyBytes []byte
	if reqBody != nil {
		var err error
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("tron: marshal request: %w", err)
		}
	} else {
		bodyBytes = []byte("{}")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("tron: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tron: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tron: unexpected status %d", resp.StatusCode)
	}

	var raw json.RawMessage
	err = json.NewDecoder(resp.Body).Decode(&raw)
	if err != nil {
		return nil, fmt.Errorf("tron: decode response: %w", err)
	}

	return raw, nil
}

func (c *Client) GetAccount(ctx context.Context, address string) (*AccountInfo, error) {
	body := map[string]any{
		"address": address,
		"visible": true,
	}

	raw, err := c.do(ctx, "/wallet/getaccount", body)
	if err != nil {
		return nil, fmt.Errorf("get account: %w", err)
	}

	var info AccountInfo
	err = json.Unmarshal(raw, &info)
	if err != nil {
		return nil, fmt.Errorf("tron: unmarshal account: %w", err)
	}

	return &info, nil
}

func (c *Client) GetAccountResource(ctx context.Context, address string) (*AccountResource, error) {
	body := map[string]any{
		"address": address,
		"visible": true,
	}

	raw, err := c.do(ctx, "/wallet/getaccountresource", body)
	if err != nil {
		return nil, fmt.Errorf("get account resource: %w", err)
	}

	var res AccountResource
	err = json.Unmarshal(raw, &res)
	if err != nil {
		return nil, fmt.Errorf("tron: unmarshal account resource: %w", err)
	}

	return &res, nil
}

var ErrTxNotFound = fmt.Errorf("transaction not found")

func (c *Client) GetTransactionInfoByID(ctx context.Context, txID string) (*TxInfo, error) {
	body := map[string]any{
		"value": txID,
	}

	raw, err := c.do(ctx, "/wallet/gettransactioninfobyid", body)
	if err != nil {
		return nil, fmt.Errorf("get transaction info: %w", err)
	}

	rawStr := strings.TrimSpace(string(raw))
	if rawStr == "{}" || rawStr == "" {
		return nil, ErrTxNotFound
	}

	var info TxInfo
	err = json.Unmarshal(raw, &info)
	if err != nil {
		return nil, fmt.Errorf("tron: unmarshal tx info: %w", err)
	}

	if info.ID == "" {
		return nil, ErrTxNotFound
	}

	return &info, nil
}

func (c *Client) TriggerConstantContract(ctx context.Context, ownerAddr, contractAddr, functionSelector, parameter string) (*ConstantResult, error) {
	body := map[string]any{
		"owner_address":     ownerAddr,
		"contract_address":  contractAddr,
		"function_selector": functionSelector,
		"parameter":         parameter,
		"visible":           true,
	}

	raw, err := c.do(ctx, "/wallet/triggerconstantcontract", body)
	if err != nil {
		return nil, fmt.Errorf("trigger constant contract: %w", err)
	}

	var result ConstantResult
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return nil, fmt.Errorf("tron: unmarshal constant result: %w", err)
	}

	if result.Result != nil {
		if r, ok := result.Result["result"].(bool); ok && !r {
			msg := "unknown error"
			if m, ok := result.Result["message"].(string); ok {
				decoded, decErr := hex.DecodeString(m)
				if decErr == nil {
					msg = string(decoded)
				} else {
					msg = m
				}
			}
			return nil, fmt.Errorf("tron: contract call failed: %s", msg)
		}
	}

	return &result, nil
}

func ValidateAddress(address string) error {
	if len(address) == 0 {
		return fmt.Errorf("empty address")
	}
	if !strings.HasPrefix(address, "T") {
		return fmt.Errorf("tron address must start with 'T'")
	}
	if len(address) != 34 {
		return fmt.Errorf("tron address must be 34 characters, got %d", len(address))
	}

	decoded := base58.Decode(address)
	if len(decoded) != 25 {
		return fmt.Errorf("invalid base58 encoding")
	}

	payload := decoded[:21]
	checksum := decoded[21:]

	hash1 := sha256.Sum256(payload)
	hash2 := sha256.Sum256(hash1[:])

	if hash2[0] != checksum[0] || hash2[1] != checksum[1] ||
		hash2[2] != checksum[2] || hash2[3] != checksum[3] {
		return fmt.Errorf("invalid checksum")
	}

	if payload[0] != 0x41 {
		return fmt.Errorf("invalid version byte: expected 0x41, got 0x%02x", payload[0])
	}

	return nil
}

func AddressToHex(address string) (string, error) {
	decoded := base58.Decode(address)
	if len(decoded) < 21 {
		return "", fmt.Errorf("invalid tron address")
	}
	return hex.EncodeToString(decoded[1:21]), nil
}

func FormatSUN(sun *big.Int) string {
	divisor := big.NewInt(1_000_000)
	whole := new(big.Int).Div(sun, divisor)
	frac := new(big.Int).Mod(sun, divisor)
	frac.Abs(frac)
	return fmt.Sprintf("%s.%06s", whole, frac)
}

func DecodeTRC20Balance(hexData string) (*big.Int, error) {
	data, err := hex.DecodeString(hexData)
	if err != nil {
		return nil, fmt.Errorf("decode hex: %w", err)
	}
	if len(data) < 32 {
		padded := make([]byte, 32)
		copy(padded[32-len(data):], data)
		data = padded
	}
	return new(big.Int).SetBytes(data[:32]), nil
}

func DecodeTRC20Decimals(hexData string) (uint8, error) {
	balance, err := DecodeTRC20Balance(hexData)
	if err != nil {
		return 0, err
	}
	return uint8(balance.Uint64()), nil
}

func DecodeTRC20Symbol(hexData string) (string, error) {
	data, err := hex.DecodeString(hexData)
	if err != nil {
		return "", fmt.Errorf("decode hex: %w", err)
	}
	if len(data) < 64 {
		return strings.TrimRight(string(data), "\x00"), nil
	}
	dataLen := uint64(len(data))
	offset := new(big.Int).SetBytes(data[:32]).Uint64()
	if offset > dataLen {
		return "", fmt.Errorf("invalid ABI string offset")
	}
	remaining := dataLen - offset
	if remaining < 32 {
		return "", fmt.Errorf("invalid ABI string offset")
	}
	length := new(big.Int).SetBytes(data[offset : offset+32]).Uint64()
	if length > remaining-32 {
		return "", fmt.Errorf("invalid ABI string length")
	}
	return string(data[offset+32 : offset+32+length]), nil
}

func FormatTokenBalance(balance *big.Int, decimals uint8) string {
	if decimals == 0 {
		return balance.String()
	}
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	whole := new(big.Int).Div(balance, divisor)
	frac := new(big.Int).Mod(balance, divisor)

	fracStr := fmt.Sprintf("%0*d", int(decimals), frac)
	fracStr = strings.TrimRight(fracStr, "0")
	if fracStr == "" {
		return whole.String()
	}
	return whole.String() + "." + fracStr
}
