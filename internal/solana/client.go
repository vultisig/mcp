package solana

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

type Client struct {
	rpc *rpc.Client
}

func NewClient(rpcURL string) *Client {
	return &Client{
		rpc: rpc.New(rpcURL),
	}
}

func ParsePublicKey(addr string) (solana.PublicKey, error) {
	return solana.PublicKeyFromBase58(addr)
}

func FindAssociatedTokenAddress(wallet, mint, tokenProgram solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{
			wallet[:],
			tokenProgram[:],
			mint[:],
		},
		solana.SPLAssociatedTokenAccountProgramID,
	)
}

func FormatLamports(lamports uint64) string {
	whole := lamports / 1_000_000_000
	frac := lamports % 1_000_000_000
	if frac == 0 {
		return fmt.Sprintf("%d", whole)
	}
	fracStr := fmt.Sprintf("%09d", frac)
	fracStr = strings.TrimRight(fracStr, "0")
	return fmt.Sprintf("%d.%s", whole, fracStr)
}

func (c *Client) GetNativeBalance(ctx context.Context, account solana.PublicKey) (uint64, error) {
	result, err := c.rpc.GetBalance(ctx, account, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("get sol balance: %w", err)
	}
	return result.Value, nil
}

func (c *Client) GetTokenProgram(ctx context.Context, mint solana.PublicKey) (solana.PublicKey, uint8, error) {
	accountInfo, err := c.rpc.GetAccountInfo(ctx, mint)
	if err != nil {
		return solana.PublicKey{}, 0, fmt.Errorf("get mint account info: %w", err)
	}
	if accountInfo.Value == nil {
		return solana.PublicKey{}, 0, fmt.Errorf("mint account not found: %s", mint)
	}

	owner := accountInfo.Value.Owner
	if owner != solana.TokenProgramID && owner != solana.Token2022ProgramID {
		return solana.PublicKey{}, 0, fmt.Errorf("mint account not owned by token program: %s", owner)
	}

	data := accountInfo.Value.Data.GetBinary()
	var mintData token.Mint
	err = mintData.UnmarshalWithDecoder(bin.NewBinDecoder(data))
	if err != nil {
		return solana.PublicKey{}, 0, fmt.Errorf("deserialize mint data: %w", err)
	}

	return owner, mintData.Decimals, nil
}

func (c *Client) GetTokenBalance(ctx context.Context, tokenAccount solana.PublicKey) (uint64, error) {
	balance, err := c.rpc.GetTokenAccountBalance(ctx, tokenAccount, rpc.CommitmentFinalized)
	if err != nil {
		if errors.Is(err, rpc.ErrNotFound) {
			return 0, nil
		}
		errStr := err.Error()
		if strings.Contains(errStr, "could not find account") {
			return 0, nil
		}
		return 0, fmt.Errorf("get token balance: %w", err)
	}

	if balance.Value == nil || balance.Value.Amount == "" {
		return 0, nil
	}

	var amount uint64
	_, err = fmt.Sscanf(balance.Value.Amount, "%d", &amount)
	if err != nil {
		return 0, fmt.Errorf("parse token amount: %w", err)
	}

	return amount, nil
}

func (c *Client) CheckAccountExists(ctx context.Context, account solana.PublicKey) (bool, error) {
	accountInfo, err := c.rpc.GetAccountInfo(ctx, account)
	if err != nil {
		if errors.Is(err, rpc.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("check account exists: %w", err)
	}
	return accountInfo.Value != nil, nil
}

func (c *Client) BuildNativeTransfer(
	ctx context.Context,
	from solana.PublicKey,
	to solana.PublicKey,
	amount uint64,
) ([]byte, error) {
	accountInfo, err := c.rpc.GetAccountInfo(ctx, to)
	if err != nil && !errors.Is(err, rpc.ErrNotFound) {
		return nil, fmt.Errorf("check destination account: %w", err)
	}

	accountExists := accountInfo != nil && accountInfo.Value != nil

	if !accountExists {
		rentExempt, err := c.rpc.GetMinimumBalanceForRentExemption(ctx, 0, rpc.CommitmentFinalized)
		if err != nil {
			return nil, fmt.Errorf("get rent exemption: %w", err)
		}

		if amount < rentExempt {
			return nil, fmt.Errorf(
				"transfer amount %d lamports is below rent-exempt minimum %d lamports for new account",
				amount,
				rentExempt,
			)
		}
	}

	block, err := c.rpc.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("get recent blockhash: %w", err)
	}

	transferInst := system.NewTransferInstruction(
		amount,
		from,
		to,
	).Build()

	tx, err := solana.NewTransaction(
		[]solana.Instruction{transferInst},
		block.Value.Blockhash,
		solana.TransactionPayer(from),
	)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal transaction: %w", err)
	}

	return txBytes, nil
}

func (c *Client) BuildTokenTransfer(
	ctx context.Context,
	mint solana.PublicKey,
	fromOwner solana.PublicKey,
	toOwner solana.PublicKey,
	amount uint64,
	decimals uint8,
	tokenProgram solana.PublicKey,
) ([]byte, error) {
	sourceATA, _, err := FindAssociatedTokenAddress(fromOwner, mint, tokenProgram)
	if err != nil {
		return nil, fmt.Errorf("find source ATA: %w", err)
	}

	destATA, _, err := FindAssociatedTokenAddress(toOwner, mint, tokenProgram)
	if err != nil {
		return nil, fmt.Errorf("find destination ATA: %w", err)
	}

	var instructions []solana.Instruction

	destExists, err := c.CheckAccountExists(ctx, destATA)
	if err != nil {
		return nil, fmt.Errorf("check dest ATA: %w", err)
	}

	if !destExists {
		createATAInst := buildCreateATAInstruction(fromOwner, toOwner, mint, destATA, tokenProgram)
		instructions = append(instructions, createATAInst)
	}

	transferData := make([]byte, 10)
	transferData[0] = 12 // TransferChecked instruction discriminator
	binary.LittleEndian.PutUint64(transferData[1:9], amount)
	transferData[9] = decimals

	transferInst := solana.NewInstruction(
		tokenProgram,
		[]*solana.AccountMeta{
			{PublicKey: sourceATA, IsSigner: false, IsWritable: true},
			{PublicKey: mint, IsSigner: false, IsWritable: false},
			{PublicKey: destATA, IsSigner: false, IsWritable: true},
			{PublicKey: fromOwner, IsSigner: true, IsWritable: false},
		},
		transferData,
	)
	instructions = append(instructions, transferInst)

	block, err := c.rpc.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("get recent blockhash: %w", err)
	}

	tx, err := solana.NewTransaction(
		instructions,
		block.Value.Blockhash,
		solana.TransactionPayer(fromOwner),
	)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal transaction: %w", err)
	}

	return txBytes, nil
}

func buildCreateATAInstruction(payer, owner, mint, ataAddress, tokenProgram solana.PublicKey) solana.Instruction {
	return solana.NewInstruction(
		solana.SPLAssociatedTokenAccountProgramID,
		[]*solana.AccountMeta{
			{PublicKey: payer, IsSigner: true, IsWritable: true},
			{PublicKey: ataAddress, IsSigner: false, IsWritable: true},
			{PublicKey: owner, IsSigner: false, IsWritable: false},
			{PublicKey: mint, IsSigner: false, IsWritable: false},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
			{PublicKey: tokenProgram, IsSigner: false, IsWritable: false},
		},
		[]byte{0},
	)
}
