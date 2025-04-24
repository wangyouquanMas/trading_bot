package pumpfun

import (
	"fmt"

	"github.com/dexs-k/dexs-backend/pkg/pumpfun/pump/idl/generated/pump"
	"github.com/gagliardetto/solana-go"
)

type BondingCurvePublicKeys struct {
	BondingCurve           solana.PublicKey
	AssociatedBondingCurve solana.PublicKey
}

// GetBondingCurveAndAssociatedBondingCurve returns the bonding curve and associated bonding curve, in a structured format.
func GetBondingCurveAndAssociatedBondingCurve(mint solana.PublicKey) (*BondingCurvePublicKeys, error) {
	// Derive bonding curve address.
	// define the seeds used to derive the PDA
	// getProgramDerivedAddress equivalent.
	seeds := [][]byte{
		[]byte("bonding-curve"),
		mint.Bytes(),
	}
	bondingCurve, _, err := solana.FindProgramAddress(seeds, pump.ProgramID)
	if err != nil {
		return nil, fmt.Errorf("failed to derive bonding curve address: %w", err)
	}
	// Derive associated bonding curve address.
	associatedBondingCurve, _, err := solana.FindAssociatedTokenAddress(
		bondingCurve,
		mint,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to derive associated bonding curve address: %w", err)
	}
	return &BondingCurvePublicKeys{
		BondingCurve:           bondingCurve,
		AssociatedBondingCurve: associatedBondingCurve,
	}, nil
}
