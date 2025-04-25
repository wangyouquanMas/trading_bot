package utils

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// GetMulTokenBalance retrieves token balances for multiple accounts
// Returns a slice of balances in the same order as the input accounts
func GetMulTokenBalance(ctx context.Context, client *rpc.Client, accounts ...solana.PublicKey) ([]uint64, error) {
	if len(accounts) == 0 {
		return nil, fmt.Errorf("no accounts provided")
	}

	balances := make([]uint64, len(accounts))

	for i, account := range accounts {
		// Get token account balance
		resp, err := client.GetTokenAccountBalance(ctx, account)
		if err != nil {
			return nil, fmt.Errorf("failed to get balance for account %s: %w", account.String(), err)
		}

		// Parse the amount string to uint64
		balance, err := strconv.ParseUint(resp.Value.Amount, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse balance amount for account %s: %w", account.String(), err)
		}

		balances[i] = balance
	}

	return balances, nil
}
