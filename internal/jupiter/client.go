package jupiter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const (
	DefaultSlippageBps = 100
	DefaultMaxAccounts = 30
)

type Client struct {
	apiURL     string
	httpClient *http.Client
	rpcClient  *rpc.Client
}

func NewClient(apiURL string, rpcClient *rpc.Client) *Client {
	return &Client{
		apiURL: apiURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		rpcClient: rpcClient,
	}
}

type SwapResult struct {
	TxBytes       []byte
	OutAmount     *big.Int
	MinimumOutput *big.Int
	PriceImpact   string
	InputMint     string
	OutputMint    string
}

func (c *Client) BuildSwapTransaction(
	ctx context.Context,
	userAddress string,
	inputMint string,
	outputMint string,
	amount *big.Int,
	slippageBps int,
) (*SwapResult, error) {
	if inputMint == "" {
		inputMint = solana.SolMint.String()
	}
	if outputMint == "" {
		outputMint = solana.SolMint.String()
	}
	if slippageBps <= 0 {
		slippageBps = DefaultSlippageBps
	}

	quote, err := c.GetQuote(ctx, inputMint, outputMint, amount, slippageBps)
	if err != nil {
		return nil, fmt.Errorf("get quote: %w", err)
	}

	insts, err := c.GetSwapInstructions(ctx, quote, userAddress)
	if err != nil {
		return nil, fmt.Errorf("get swap instructions: %w", err)
	}

	block, err := c.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("get latest blockhash: %w", err)
	}

	feePayer, err := solana.PublicKeyFromBase58(userAddress)
	if err != nil {
		return nil, fmt.Errorf("parse fee payer address: %w", err)
	}

	var allInstructions []solana.Instruction

	for i, inst := range insts.ComputeBudgetInstructions {
		typed, err := inst.ToInstruction()
		if err != nil {
			return nil, fmt.Errorf("parse compute budget instruction %d: %w", i, err)
		}
		allInstructions = append(allInstructions, typed)
	}

	for i, inst := range insts.SetupInstructions {
		typed, err := inst.ToInstruction()
		if err != nil {
			return nil, fmt.Errorf("parse setup instruction %d: %w", i, err)
		}
		allInstructions = append(allInstructions, typed)
	}

	swapInst, err := insts.SwapInstruction.ToInstruction()
	if err != nil {
		return nil, fmt.Errorf("parse swap instruction: %w", err)
	}
	allInstructions = append(allInstructions, swapInst)

	if insts.CleanupInstruction != nil {
		cleanupInst, err := insts.CleanupInstruction.ToInstruction()
		if err != nil {
			return nil, fmt.Errorf("parse cleanup instruction: %w", err)
		}
		allInstructions = append(allInstructions, cleanupInst)
	}

	if len(insts.AddressLookupTableAddresses) > 0 {
		return nil, fmt.Errorf("versioned transactions with address lookup tables are not supported")
	}

	tx, err := solana.NewTransaction(
		allInstructions,
		block.Value.Blockhash,
		solana.TransactionPayer(feePayer),
	)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal transaction: %w", err)
	}

	outAmount, ok := new(big.Int).SetString(quote.OutAmount, 10)
	if !ok {
		return nil, fmt.Errorf("parse out amount: %s", quote.OutAmount)
	}

	var minOutput *big.Int
	if quote.OtherAmountThreshold != "" {
		minOutput, ok = new(big.Int).SetString(quote.OtherAmountThreshold, 10)
		if !ok {
			return nil, fmt.Errorf("parse minimum output amount: %s", quote.OtherAmountThreshold)
		}
	}

	return &SwapResult{
		TxBytes:       txBytes,
		OutAmount:     outAmount,
		MinimumOutput: minOutput,
		PriceImpact:   quote.PriceImpactPct,
		InputMint:     inputMint,
		OutputMint:    outputMint,
	}, nil
}

func (c *Client) GetQuote(
	ctx context.Context,
	inputMint, outputMint string,
	amount *big.Int,
	slippageBps int,
) (QuoteResponse, error) {
	params := url.Values{}
	params.Set("swapMode", "ExactIn")
	params.Set("inputMint", inputMint)
	params.Set("outputMint", outputMint)
	params.Set("amount", amount.String())
	params.Set("slippageBps", fmt.Sprintf("%d", slippageBps))
	params.Set("restrictIntermediateTokens", "true")
	params.Set("maxAccounts", fmt.Sprintf("%d", DefaultMaxAccounts))

	path := fmt.Sprintf("/swap/v1/quote?%s", params.Encode())
	body, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return QuoteResponse{}, fmt.Errorf("jupiter quote: %w", err)
	}

	var resp QuoteResponse
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return QuoteResponse{}, fmt.Errorf("parse quote response: %w", err)
	}
	return resp, nil
}

func (c *Client) GetSwapInstructions(
	ctx context.Context,
	quote QuoteResponse,
	userPublicKey string,
) (*SwapInstructionsResponse, error) {
	reqBody := swapRequest{
		UserPublicKey:           userPublicKey,
		QuoteResponse:           quote,
		WrapAndUnwrapSol:        true,
		UseSharedAccounts:       true,
		DynamicComputeUnitLimit: true,
	}

	body, err := c.doRequest(ctx, http.MethodPost, "/swap/v1/swap-instructions", reqBody)
	if err != nil {
		return nil, fmt.Errorf("jupiter swap instructions: %w", err)
	}

	var resp SwapInstructionsResponse
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, fmt.Errorf("parse swap instructions: %w", err)
	}
	return &resp, nil
}

func (c *Client) doRequest(ctx context.Context, method, path string, payload interface{}) ([]byte, error) {
	fullURL := c.apiURL + path

	var reqBody io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// Jupiter API types

type QuoteResponse struct {
	InputMint            string      `json:"inputMint"`
	InAmount             string      `json:"inAmount"`
	OutputMint           string      `json:"outputMint"`
	OutAmount            string      `json:"outAmount"`
	OtherAmountThreshold string      `json:"otherAmountThreshold"`
	SwapMode             string      `json:"swapMode"`
	SlippageBps          int         `json:"slippageBps"`
	PriceImpactPct       string      `json:"priceImpactPct"`
	RoutePlan            []RoutePlan `json:"routePlan"`
}

type RoutePlan struct {
	SwapInfo SwapInfo `json:"swapInfo"`
	Percent  int      `json:"percent"`
}

type SwapInfo struct {
	AmmKey     string `json:"ammKey"`
	Label      string `json:"label"`
	InputMint  string `json:"inputMint"`
	OutputMint string `json:"outputMint"`
	InAmount   string `json:"inAmount"`
	OutAmount  string `json:"outAmount"`
	FeeAmount  string `json:"feeAmount,omitempty"`
	FeeMint    string `json:"feeMint,omitempty"`
}

type swapRequest struct {
	UserPublicKey           string        `json:"userPublicKey"`
	QuoteResponse           QuoteResponse `json:"quoteResponse"`
	WrapAndUnwrapSol        bool          `json:"wrapAndUnwrapSol,omitempty"`
	UseSharedAccounts       bool          `json:"useSharedAccounts,omitempty"`
	AsLegacyTransaction     bool          `json:"asLegacyTransaction,omitempty"`
	DynamicComputeUnitLimit bool          `json:"dynamicComputeUnitLimit,omitempty"`
}

type SwapInstructionsResponse struct {
	ComputeBudgetInstructions   []InstructionData `json:"computeBudgetInstructions"`
	SetupInstructions           []InstructionData `json:"setupInstructions"`
	SwapInstruction             InstructionData   `json:"swapInstruction"`
	CleanupInstruction          *InstructionData  `json:"cleanupInstruction,omitempty"`
	AddressLookupTableAddresses []string          `json:"addressLookupTableAddresses"`
}

type InstructionData struct {
	ProgramId string    `json:"programId"`
	Accounts  []Account `json:"accounts"`
	Data      string    `json:"data"`
}

type Account struct {
	Pubkey     string `json:"pubkey"`
	IsSigner   bool   `json:"isSigner"`
	IsWritable bool   `json:"isWritable"`
}

func (i InstructionData) ToInstruction() (solana.Instruction, error) {
	programID, err := solana.PublicKeyFromBase58(i.ProgramId)
	if err != nil {
		return nil, fmt.Errorf("parse program id: %w", err)
	}

	accounts := make([]*solana.AccountMeta, 0, len(i.Accounts))
	for _, acc := range i.Accounts {
		pk, err := solana.PublicKeyFromBase58(acc.Pubkey)
		if err != nil {
			return nil, fmt.Errorf("parse account %s: %w", acc.Pubkey, err)
		}
		accounts = append(accounts, solana.NewAccountMeta(pk, acc.IsWritable, acc.IsSigner))
	}

	data, err := base64.StdEncoding.DecodeString(i.Data)
	if err != nil {
		return nil, fmt.Errorf("decode instruction data: %w", err)
	}

	return solana.NewInstruction(programID, accounts, data), nil
}
