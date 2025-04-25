package utils

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
)

func TestGetMulTokenBalance(t *testing.T) {
	// Skip the test if running in CI environment without proper setup
	if os.Getenv("CI") == "true" && os.Getenv("TEST_SETUP") == "" {
		t.Skip("Skipping test in CI environment without proper setup")
	}

	// Get RPC endpoint from environment or use default
	rpcEndpoint := os.Getenv("TEST_RPC_ENDPOINT")
	if rpcEndpoint == "" {
		rpcEndpoint = "https://api.devnet.solana.com"
	}

	// Create RPC client
	client := rpc.New(rpcEndpoint)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test cases
	testCases := []struct {
		name             string
		accountAddresses []string
		expectedError    bool
		expectedNonZero  bool // If true, we expect at least one non-zero balance
	}{
		{
			name: "Valid Token Accounts",
			// Use known devnet token accounts with balances for testing
			accountAddresses: []string{
				"4vDmqnKLN2jdPGR2DMf5L6C93AG4XbHdfRAXJuironK8", // Example PDA account
				"5mDDjsgR9HQGFjHGy1cZ7fNYMzqkZ9hBeAJbjkcTZgCt", // Example PDA account
			},
			expectedError:   false,
			expectedNonZero: false, // Might be zero on devnet
		},
		{
			name: "Invalid Account Addresses",
			accountAddresses: []string{
				"invalid_address_format",
				"another_invalid_address",
			},
			expectedError:   true,
			expectedNonZero: false,
		},
		{
			name:             "Empty Account List",
			accountAddresses: []string{},
			expectedError:    true,
			expectedNonZero:  false,
		},
		{
			name: "Mixing Valid and Invalid",
			accountAddresses: []string{
				"4vDmqnKLN2jdPGR2DMf5L6C93AG4XbHdfRAXJuironK8", // Valid
				"invalid_address_format",                       // Invalid
			},
			expectedError:   true,
			expectedNonZero: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert string addresses to PublicKey objects
			accounts := make([]solana.PublicKey, 0, len(tc.accountAddresses))

			// This might fail for invalid addresses, which is part of our test
			var conversionError bool
			for _, addr := range tc.accountAddresses {
				pubKey, err := solana.PublicKeyFromBase58(addr)
				if err != nil {
					conversionError = true
					break
				}
				accounts = append(accounts, pubKey)
			}

			// If we expect an error and already hit a conversion error, we can skip the actual function call
			if tc.expectedError && conversionError {
				t.Log("Address conversion failed as expected")
				return
			}

			// Call the function under test
			balances, err := GetMulTokenBalance(ctx, client, accounts...)

			// Check error expectation
			if tc.expectedError {
				assert.Error(t, err, "Expected an error but got none")
				return
			}

			// If we don't expect an error, verify the result
			assert.NoError(t, err, "Unexpected error")
			assert.NotNil(t, balances, "Expected non-nil balances")
			assert.Equal(t, len(accounts), len(balances), "Expected same number of balances as accounts")

			// Check if at least one balance is non-zero when expected
			if tc.expectedNonZero {
				hasNonZero := false
				for _, balance := range balances {
					if balance > 0 {
						hasNonZero = true
						break
					}
				}
				assert.True(t, hasNonZero, "Expected at least one non-zero balance")
			}

			// Log the results for informational purposes
			for i, balance := range balances {
				t.Logf("Account %s balance: %d", accounts[i].String(), balance)
			}
		})
	}
}

// TestGetMulTokenBalanceMock demonstrates how to test with mocked dependencies
func TestGetMulTokenBalanceMock(t *testing.T) {
	t.Skip("Skipping mocked test until implementation is available")

	// This would be implemented with mocking libraries like:
	// - github.com/golang/mock/gomock
	// - github.com/stretchr/testify/mock

	// Example structure:
	/*
		// Setup mock controller
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Create mock RPC client
		mockClient := mock_rpc.NewMockClient(ctrl)

		// Set up expectations
		mockClient.EXPECT().GetTokenAccountBalance(gomock.Any(), gomock.Any()).
			Return(&rpc.GetTokenAccountBalanceResult{
				Value: rpc.TokenAccountBalance{
					Amount: "1000000",
					Decimals: 6,
				},
			}, nil).Times(2)

		// Create test accounts
		account1 := solana.MustPublicKeyFromBase58("4vDmqnKLN2jdPGR2DMf5L6C93AG4XbHdfRAXJuironK8")
		account2 := solana.MustPublicKeyFromBase58("5mDDjsgR9HQGFjHGy1cZ7fNYMzqkZ9hBeAJbjkcTZgCt")

		// Call function under test
		balances, err := GetMulTokenBalance(context.Background(), mockClient, account1, account2)

		// Assertions
		assert.NoError(t, err)
		assert.Equal(t, 2, len(balances))
		assert.Equal(t, uint64(1000000), balances[0])
		assert.Equal(t, uint64(1000000), balances[1])
	*/
}
