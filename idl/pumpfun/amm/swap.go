package amm

import (
	"fmt"
	"solana-pumpswap-demo/idl/pumpfun/amm/idl/generated/amm"

	ag_solanago "github.com/gagliardetto/solana-go"
)

type SwapDirection int

const (
	BuyDirection SwapDirection = iota
	SellDirection
	ProgramStrToken = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
)

type SwapParam struct {
	// Direction
	Direction SwapDirection
	// Parameters:
	TokenAmount1 uint64 // BaseAmountOut(Buy) Or BaseAmountIn(Sell)
	TokenAmount2 uint64 // MaxQuoteAmountIn(Buy) Or MinQuoteAmountOut(Sell)
	// Accounts:
	Pool                             ag_solanago.PublicKey
	User                             ag_solanago.PublicKey
	BaseMint                         ag_solanago.PublicKey
	QuoteMint                        ag_solanago.PublicKey
	UserBaseTokenAccount             ag_solanago.PublicKey
	UserQuoteTokenAccount            ag_solanago.PublicKey
	PoolBaseTokenAccount             ag_solanago.PublicKey
	PoolQuoteTokenAccount            ag_solanago.PublicKey
	ProtocolFeeRecipient             ag_solanago.PublicKey
	ProtocolFeeRecipientTokenAccount ag_solanago.PublicKey
	BaseTokenProgram                 ag_solanago.PublicKey
	QuoteTokenProgram                ag_solanago.PublicKey
}

// NewSwapInstruction
func NewSwapInstruction(para *SwapParam) (ag_solanago.Instruction, error) {
	switch para.Direction {
	case BuyDirection:
		if err := ValidateProtocolFeeRecipient(para.ProtocolFeeRecipient); err != nil {
			return nil, err
		}
		buy := amm.NewBuyInstruction(
			para.TokenAmount1, // BaseAmountOut
			para.TokenAmount2, // MaxQuoteAmountIn
			para.Pool,
			para.User,
			PumpAmmGlobalConfigAddress,
			para.BaseMint,
			para.QuoteMint,
			para.UserBaseTokenAccount,
			para.UserQuoteTokenAccount,
			para.PoolBaseTokenAccount,
			para.PoolQuoteTokenAccount,
			para.ProtocolFeeRecipient,
			para.ProtocolFeeRecipientTokenAccount,
			para.BaseTokenProgram,
			para.QuoteTokenProgram,
			ag_solanago.SystemProgramID,
			ag_solanago.SPLAssociatedTokenAccountProgramID,
			PumpAmmEventAuthorityAddress,
			amm.ProgramID,
		)
		return buy.ValidateAndBuild()
	case SellDirection:
		if err := ValidateProtocolFeeRecipient(para.ProtocolFeeRecipient); err != nil {
			return nil, err
		}
		sell := amm.NewSellInstruction(
			para.TokenAmount1, // BaseAmountIn
			para.TokenAmount2, // MinQuoteAmountOut
			para.Pool,
			para.User,
			PumpAmmGlobalConfigAddress,
			para.BaseMint,
			para.QuoteMint,
			para.UserBaseTokenAccount,
			para.UserQuoteTokenAccount,
			para.PoolBaseTokenAccount,
			para.PoolQuoteTokenAccount,
			para.ProtocolFeeRecipient,
			para.ProtocolFeeRecipientTokenAccount,
			para.BaseTokenProgram,
			para.QuoteTokenProgram,
			ag_solanago.SystemProgramID,
			ag_solanago.SPLAssociatedTokenAccountProgramID,
			PumpAmmEventAuthorityAddress,
			amm.ProgramID,
		)
		return sell.ValidateAndBuild()
	default:
		return nil, fmt.Errorf("unknown swap direction: %d", para.Direction)
	}
}

func ValidateProtocolFeeRecipient(recipient ag_solanago.PublicKey) error {
	if !recipient.Equals(ProtocolFeeRecipients[0]) &&
		!recipient.Equals(ProtocolFeeRecipients[1]) &&
		!recipient.Equals(ProtocolFeeRecipients[2]) &&
		!recipient.Equals(ProtocolFeeRecipients[3]) &&
		!recipient.Equals(ProtocolFeeRecipients[4]) &&
		!recipient.Equals(ProtocolFeeRecipients[5]) &&
		!recipient.Equals(ProtocolFeeRecipients[6]) &&
		!recipient.Equals(ProtocolFeeRecipients[7]) {
		return fmt.Errorf("invalid protocol fee recipient: %s", recipient.String())
	}
	return nil
}

func BuildBuyInstruction(
	baseAmountOut uint64,
	maxQuoteAmountIn uint64,
	user ag_solanago.PublicKey,
	pool ag_solanago.PublicKey,
	baseMint ag_solanago.PublicKey,
	quoteMint ag_solanago.PublicKey,
	userBaseTokenAccount ag_solanago.PublicKey,
	userQuoteTokenAccount ag_solanago.PublicKey,
	poolBaseTokenAccount ag_solanago.PublicKey,
	poolQuoteTokenAccount ag_solanago.PublicKey,
	protocolFeeRecipient ag_solanago.PublicKey,
	protocolFeeRecipientTokenAccount ag_solanago.PublicKey,
) (ag_solanago.Instruction, error) {
	const ProgramStrToken = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"

	swapParam := &SwapParam{
		Direction:                        BuyDirection,
		TokenAmount1:                     baseAmountOut,
		TokenAmount2:                     maxQuoteAmountIn,
		Pool:                             pool,
		User:                             user,
		BaseMint:                         baseMint,
		QuoteMint:                        quoteMint,
		UserBaseTokenAccount:             userBaseTokenAccount,
		UserQuoteTokenAccount:            userQuoteTokenAccount,
		PoolBaseTokenAccount:             poolBaseTokenAccount,
		PoolQuoteTokenAccount:            poolQuoteTokenAccount,
		ProtocolFeeRecipient:             protocolFeeRecipient,
		ProtocolFeeRecipientTokenAccount: protocolFeeRecipientTokenAccount,
		BaseTokenProgram:                 ag_solanago.MustPublicKeyFromBase58(ProgramStrToken),
		QuoteTokenProgram:                ag_solanago.MustPublicKeyFromBase58(ProgramStrToken),
	}

	return NewSwapInstruction(swapParam)
}

func BuildSellInstruction(
	baseAmountIn uint64,
	minQuoteAmountOut uint64,
	user ag_solanago.PublicKey,
	pool ag_solanago.PublicKey,
	baseMint ag_solanago.PublicKey,
	quoteMint ag_solanago.PublicKey,
	userBaseTokenAccount ag_solanago.PublicKey,
	userQuoteTokenAccount ag_solanago.PublicKey,
	poolBaseTokenAccount ag_solanago.PublicKey,
	poolQuoteTokenAccount ag_solanago.PublicKey,
	protocolFeeRecipient ag_solanago.PublicKey,
	protocolFeeRecipientTokenAccount ag_solanago.PublicKey,
) (ag_solanago.Instruction, error) {
	swapParam := &SwapParam{
		Direction:                        SellDirection,
		TokenAmount1:                     baseAmountIn,
		TokenAmount2:                     minQuoteAmountOut,
		Pool:                             pool,
		User:                             user,
		BaseMint:                         baseMint,
		QuoteMint:                        quoteMint,
		UserBaseTokenAccount:             userBaseTokenAccount,
		UserQuoteTokenAccount:            userQuoteTokenAccount,
		PoolBaseTokenAccount:             poolBaseTokenAccount,
		PoolQuoteTokenAccount:            poolQuoteTokenAccount,
		ProtocolFeeRecipient:             protocolFeeRecipient,
		ProtocolFeeRecipientTokenAccount: protocolFeeRecipientTokenAccount,
		BaseTokenProgram:                 ag_solanago.MustPublicKeyFromBase58(ProgramStrToken),
		QuoteTokenProgram:                ag_solanago.MustPublicKeyFromBase58(ProgramStrToken),
	}
	return NewSwapInstruction(swapParam)
}
