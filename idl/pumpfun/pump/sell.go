package pumpfun

import (
	"context"
	"fmt"

	"math/big"
	"strconv"

	"github.com/dexs-k/dexs-backend/pkg/pumpfun/pump/idl/generated/pump"
	"github.com/dexs-k/dexs-backend/pkg/trade"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// BuildSellInstruction is a function that returns the pump.fun instructions to sell the token
func BuildSellInstruction(ata, user, mint solana.PublicKey, sellTokenAmount uint64, slippageBasisPoint uint32,
	all bool, rpcClient *rpc.Client, price float64, inDecimal, outDecimal uint8) (*pump.Instruction, uint64, error) {
	if all {
		tokenAccounts, err := rpcClient.GetTokenAccountBalance(context.TODO(), ata, rpc.CommitmentConfirmed)
		if err != nil {
			return nil, 0, fmt.Errorf("can't get amount of token in balance: %w", err)
		}
		amount, err := strconv.Atoi(tokenAccounts.Value.Amount)
		if err != nil {
			return nil, 0, fmt.Errorf("can't convert token amount to integer: %w", err)
		}
		sellTokenAmount = uint64(amount)
	}
	bondingCurveData, err := GetBondingCurveAndAssociatedBondingCurve(mint)
	if err != nil {
		return nil, 0, fmt.Errorf("can't get bonding curve data: %w", err)
	}

	var minSolOutputUint64, solOutput uint64
	// 如果价格不为空 那么按照价格走而不是恒乘积走
	if price != 0 {
		minSolOutputUint64, solOutput, err = trade.CalcMinAmountOutByPrice(slippageBasisPoint, sellTokenAmount, false, price, inDecimal, outDecimal, trade.PumpFee)
		if err != nil {
			return nil, 0, err
		}
	} else {
		bondingCurve, err := FetchBondingCurve(rpcClient, bondingCurveData.BondingCurve)
		if err != nil {
			return nil, 0, fmt.Errorf("can't fetch bonding curve: %w", err)
		}

		//percentage := float64(1.0 - (slippageBasisPoint / 10e3))

		slippage := big.NewFloat(float64(1))
		slippage = slippage.Quo(big.NewFloat(float64(slippageBasisPoint)), big.NewFloat(float64(1e4)))

		slippageF64, _ := slippage.Float64()
		percentage := float64(1.0 - slippageF64)

		minSolOutputUint64, solOutput = calculateSellQuote(sellTokenAmount, bondingCurve, percentage)
	}

	sellInstr := pump.NewSellInstruction(
		sellTokenAmount,
		minSolOutputUint64,
		GlobalPumpFunAddress,
		PumpFunFeeRecipient,
		mint,
		bondingCurveData.BondingCurve,
		bondingCurveData.AssociatedBondingCurve,
		ata,
		user,
		system.ProgramID,
		associatedtokenaccount.ProgramID,
		token.ProgramID,
		PumpFunEventAuthority,
		pump.ProgramID,
	)
	sell, err := sellInstr.ValidateAndBuild()
	if err != nil {
		return nil, 0, fmt.Errorf("can't validate and build sell instruction: %w", err)
	}
	return sell, solOutput, nil
}

// calculateSellQuote calculates how many SOL should be received for selling a specific amount of tokens, given a specific amount of token, bonding curve data, and percentage.
// tokenAmount is the amount of token you want to sell
// bondingCurve is the bonding curve data, that will help to calculate the number of sol to get
// percentage is the slippage, 0.98 means 2% slippage
func calculateSellQuote(tokenAmount uint64, bondingCurve *BondingCurveData, percentage float64) (uint64, uint64) {
	amount := big.NewInt(int64(tokenAmount))

	// Clone bonding curve data to avoid mutations
	virtualSolReserves := new(big.Int).Set(bondingCurve.VirtualSolReserves)
	virtualTokenReserves := new(big.Int).Set(bondingCurve.VirtualTokenReserves)

	// Compute the new virtual reserves
	x := new(big.Int).Mul(virtualSolReserves, virtualTokenReserves)
	y := new(big.Int).Add(virtualTokenReserves, amount)
	a := new(big.Int).Div(x, y)
	out := new(big.Int).Sub(virtualSolReserves, a)
	percentageMultiplier := big.NewFloat(percentage)

	outFloat := new(big.Float).SetInt(out)
	number := new(big.Float).Mul(outFloat, percentageMultiplier)
	final, _ := number.Int(nil)
	return final.Uint64(), out.Uint64()
}
