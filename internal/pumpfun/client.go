package pumpfun

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	solanaclient "github.com/vultisig/mcp/internal/solana"
)

var (
	// ProgramID is the pump.fun on-chain program.
	ProgramID = solana.MustPublicKeyFromBase58("6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P")

	// graduationThresholdLamports is the approximate real SOL reserve level
	// at which a pump.fun token graduates to PumpSwap/Raydium (~85 SOL).
	graduationThresholdLamports = uint64(85_000_000_000)

	// Anchor account discriminator: sha256("account:BondingCurve")[:8]
	bondingCurveDiscriminator = [8]byte{0x17, 0xb7, 0xf8, 0x37, 0x60, 0xd8, 0xac, 0x60}

	bondingCurveDataLen = 8 + 5*8 + 1 // discriminator + 5 uint64 + 1 bool = 49
)

// Client reads pump.fun bonding curve state from Solana RPC.
type Client struct {
	rpc *rpc.Client
}

// NewClient creates a pump.fun client wrapping the given Solana RPC client.
func NewClient(rpcClient *rpc.Client) *Client {
	return &Client{rpc: rpcClient}
}

// BondingCurveState holds the raw on-chain state of a pump.fun bonding curve account.
type BondingCurveState struct {
	VirtualTokenReserves uint64
	VirtualSolReserves   uint64
	RealTokenReserves    uint64
	RealSolReserves      uint64
	TokenTotalSupply     uint64
	Complete             bool
}

// TokenInfo contains bonding curve state plus computed market metrics.
type TokenInfo struct {
	Mint                 string
	BondingCurveAddress  string
	VirtualTokenReserves uint64
	VirtualSolReserves   uint64
	RealTokenReserves    uint64
	RealSolReserves      uint64
	TokenTotalSupply     uint64
	Complete             bool
	PricePerTokenSOL     string
	MarketCapSOL         string
	GraduationProgress   float64
}

// DeriveBondingCurvePDA derives the bonding curve PDA for the given token mint.
func DeriveBondingCurvePDA(mint solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{[]byte("bonding-curve"), mint[:]},
		ProgramID,
	)
}

// DeriveBondingCurveATA derives the associated token account for a bonding curve PDA.
func DeriveBondingCurveATA(bondingCurve, mint solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solanaclient.FindAssociatedTokenAddress(bondingCurve, mint, solana.TokenProgramID)
}

// GetBondingCurveState reads and parses the bonding curve account data from Solana.
func (c *Client) GetBondingCurveState(ctx context.Context, bondingCurve solana.PublicKey) (*BondingCurveState, error) {
	resp, err := c.rpc.GetAccountInfo(ctx, bondingCurve)
	if err != nil {
		return nil, fmt.Errorf("get account info: %w", err)
	}
	if resp == nil || resp.Value == nil {
		return nil, fmt.Errorf("bonding curve account not found — token may not be a pump.fun token")
	}

	owner := resp.Value.Owner
	if owner != ProgramID {
		return nil, fmt.Errorf("account not owned by pump.fun program (owner: %s)", owner)
	}

	data := resp.Value.Data.GetBinary()
	if len(data) < bondingCurveDataLen {
		return nil, fmt.Errorf("bonding curve data too short: %d bytes, need at least %d", len(data), bondingCurveDataLen)
	}

	var disc [8]byte
	copy(disc[:], data[:8])
	if disc != bondingCurveDiscriminator {
		return nil, fmt.Errorf("unexpected discriminator: %x", disc)
	}

	state := &BondingCurveState{
		VirtualTokenReserves: binary.LittleEndian.Uint64(data[8:16]),
		VirtualSolReserves:   binary.LittleEndian.Uint64(data[16:24]),
		RealTokenReserves:    binary.LittleEndian.Uint64(data[24:32]),
		RealSolReserves:      binary.LittleEndian.Uint64(data[32:40]),
		TokenTotalSupply:     binary.LittleEndian.Uint64(data[40:48]),
		Complete:             data[48] != 0,
	}
	return state, nil
}

// GetTokenInfo fetches the bonding curve state for the given mint and computes market metrics.
func (c *Client) GetTokenInfo(ctx context.Context, mintAddr string) (*TokenInfo, error) {
	mint, err := solanaclient.ParsePublicKey(mintAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid mint address: %w", err)
	}

	bondingCurvePDA, _, err := DeriveBondingCurvePDA(mint)
	if err != nil {
		return nil, fmt.Errorf("derive bonding curve PDA: %w", err)
	}

	state, err := c.GetBondingCurveState(ctx, bondingCurvePDA)
	if err != nil {
		return nil, err
	}

	var pricePerTokenLamports float64
	if state.VirtualTokenReserves > 0 {
		pricePerTokenLamports = float64(state.VirtualSolReserves) / float64(state.VirtualTokenReserves)
	}

	marketCapFloat := pricePerTokenLamports * float64(state.TokenTotalSupply)
	marketCapLamports := uint64(math.Min(math.Round(marketCapFloat), float64(math.MaxUint64)))

	graduationProgress := float64(state.RealSolReserves) / float64(graduationThresholdLamports) * 100.0
	if graduationProgress > 100.0 {
		graduationProgress = 100.0
	}

	return &TokenInfo{
		Mint:                 mintAddr,
		BondingCurveAddress:  bondingCurvePDA.String(),
		VirtualTokenReserves: state.VirtualTokenReserves,
		VirtualSolReserves:   state.VirtualSolReserves,
		RealTokenReserves:    state.RealTokenReserves,
		RealSolReserves:      state.RealSolReserves,
		TokenTotalSupply:     state.TokenTotalSupply,
		Complete:             state.Complete,
		PricePerTokenSOL:     solanaclient.FormatLamports(uint64(math.Round(pricePerTokenLamports))),
		MarketCapSOL:         solanaclient.FormatLamports(marketCapLamports),
		GraduationProgress:   graduationProgress,
	}, nil
}
