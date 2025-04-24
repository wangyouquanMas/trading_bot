package pumpfun

import (
	"fmt"
	"math/big"

	"github.com/dexs-k/dexs-backend/pkg/pumpfun/pump/idl/generated/pump"
	"github.com/dexs-k/dexs-backend/pkg/trade"
	aSDK "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// CalculateBuyQuote calculates how many tokens can be purchased given a specific amount of SOL, bonding curve data, and percentage.
// solAmount is the amount of sol that you want to buy
// bondingCurve is the BondingCurveData, that includes the real, virtual token/sol reserves, in order to calculate the price.
// percentage is what you want to use to set the slippage. For 2% slippage, you want to set the percentage to 0.98.
func CalculateBuyQuote(solAmount uint64, bondingCurve *BondingCurveData, percentage float64) uint64 {
	// Convert solAmount to *big.Int
	solAmountBig := big.NewInt(int64(solAmount))

	// Clone bonding curve data to avoid mutations
	virtualSolReserves := new(big.Int).Set(bondingCurve.VirtualSolReserves)
	virtualTokenReserves := new(big.Int).Set(bondingCurve.VirtualTokenReserves)

	// Compute the new virtual reserves
	newVirtualSolReserves := new(big.Int).Add(virtualSolReserves, solAmountBig)
	invariant := new(big.Int).Mul(virtualSolReserves, virtualTokenReserves)
	newVirtualTokenReserves := new(big.Int).Div(invariant, newVirtualSolReserves)

	// Calculate the tokens to buy
	tokensToBuy := new(big.Int).Sub(virtualTokenReserves, newVirtualTokenReserves)

	// Apply the percentage reduction (e.g., 95% or 0.95)
	// Convert the percentage to a multiplier (0.95) and apply to tokensToBuy
	percentageMultiplier := big.NewFloat(percentage)
	tokensToBuyFloat := new(big.Float).SetInt(tokensToBuy)
	finalTokens := new(big.Float).Mul(tokensToBuyFloat, percentageMultiplier)

	// Convert the result back to *big.Int
	finalTokensBig, _ := finalTokens.Int(nil)

	return finalTokensBig.Uint64()
}

func BuildBuyInstruction(user aSDK.PublicKey, tokenMint aSDK.PublicKey,
	solAmountUint64 uint64, slippageBasisPoint uint32, rpcClient *rpc.Client,
	price float64, inDecimal, outDecimal uint8) (aSDK.Instruction, error) {

	/////////Going to build pumpfun buy instrustions /////
	bondingCurveData, err := GetBondingCurveAndAssociatedBondingCurve(tokenMint)
	if err != nil {
		return nil, fmt.Errorf("failed to get bonding curve data: %w", err)
	}
	var minAmountOut uint64
	// 如果价格不为空 那么按照价格走而不是恒乘积走
	if price != 0 {
		minAmountOut, _, err = trade.CalcMinAmountOutByPrice(slippageBasisPoint, solAmountUint64, true, price, inDecimal, outDecimal, trade.PumpFee)
		if err != nil {
			return nil, err
		}
	} else {
		bondingCurve, err := FetchBondingCurve(rpcClient, bondingCurveData.BondingCurve)
		if err != nil {
			return nil, fmt.Errorf("can't fetch bonding curve: %w", err)
		}

		slippage := big.NewFloat(float64(1))
		slippage = slippage.Quo(big.NewFloat(float64(slippageBasisPoint)), big.NewFloat(float64(1e4)))

		slippageF64, _ := slippage.Float64()
		percentage := float64(1.0 - slippageF64)
		minAmountOut = CalculateBuyQuote(solAmountUint64, bondingCurve, percentage)
	}

	ata, _, err := aSDK.FindAssociatedTokenAddress(
		user,
		tokenMint,
	)
	if nil != err {
		return nil, err
	}

	buyInstr := pump.NewBuyInstruction(
		minAmountOut,
		solAmountUint64,
		GlobalPumpFunAddress,
		PumpFunFeeRecipient,
		tokenMint,
		bondingCurveData.BondingCurve,
		bondingCurveData.AssociatedBondingCurve,
		ata,
		user,
		system.ProgramID,
		token.ProgramID,
		aSDK.SysVarRentPubkey,
		PumpFunEventAuthority,
		pump.ProgramID,
	)

	return buyInstr.Build(), nil
}
