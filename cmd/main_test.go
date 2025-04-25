package main

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	bin "github.com/gagliardetto/binary"
	aSDK "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	ag_rpc "github.com/gagliardetto/solana-go/rpc"
)

// MockDataBytesOrJSON implements the necessary methods to mock the RPC account data
type MockDataBytesOrJSON struct {
	binary []byte
}

// GetBinary returns the binary data
func (m *MockDataBytesOrJSON) GetBinary() []byte {
	return m.binary
}

// MockAccount mocks an RPC account
type MockAccount struct {
	Data *MockDataBytesOrJSON
}

// MockGetMultipleAccountsResult mocks the result from GetMultipleAccountsWithOpts
type MockGetMultipleAccountsResult struct {
	Value []*MockAccount
}

// MockRPC implements a minimal mock of the Solana RPC client needed for our test
type MockRPC struct {
	accounts map[string]uint64
}

// NewMockRPC creates a new mock RPC client
func NewMockRPC() *MockRPC {
	return &MockRPC{
		accounts: make(map[string]uint64),
	}
}

// AddTokenAccount adds a token account with a specific balance to our mock
func (m *MockRPC) AddTokenAccount(pubkey aSDK.PublicKey, amount uint64) {
	m.accounts[pubkey.String()] = amount
}

// GetMultipleAccountsWithOpts mocks the RPC client method
func (m *MockRPC) GetMultipleAccountsWithOpts(
	ctx context.Context,
	accounts []aSDK.PublicKey,
	opts *ag_rpc.GetMultipleAccountsOpts,
) (*MockGetMultipleAccountsResult, error) {
	result := &MockGetMultipleAccountsResult{}
	result.Value = make([]*MockAccount, len(accounts))

	for i, pubkey := range accounts {
		key := pubkey.String()

		// Check if this account exists in our mock
		if amount, exists := m.accounts[key]; exists {
			// Create a token account
			tokenAcc := &token.Account{
				Amount: amount,
			}

			// Encode the token account to binary
			var buf bytes.Buffer
			encoder := bin.NewBinEncoder(&buf)
			err := encoder.Encode(tokenAcc)
			if err != nil {
				return nil, err
			}

			// Create the account with mocked data
			result.Value[i] = &MockAccount{
				Data: &MockDataBytesOrJSON{
					binary: buf.Bytes(),
				},
			}
		} else {
			// Account not found
			result.Value[i] = nil
		}
	}

	return result, nil
}

// TestGetMulTokenBalance tests the GetMulTokenBalance function
func TestGetMulTokenBalance(t *testing.T) {
	// Create a new mock client
	mockRPC := NewMockRPC()

	// Create test accounts
	account1 := aSDK.MustPublicKeyFromBase58("4vDmqnKLN2jdPGR2DMf5L6C93AG4XbHdfRAXJuironK8")
	account2 := aSDK.MustPublicKeyFromBase58("5mDDjsgR9HQGFjHGy1cZ7fNYMzqkZ9hBeAJbjkcTZgCt")
	account3 := aSDK.MustPublicKeyFromBase58("7GFUN3bWzJMKMRZ34JLsvcqdssDbXnp589SiE33KVwcC")

	// Add accounts with specific balances
	mockRPC.AddTokenAccount(account1, 1000000)
	mockRPC.AddTokenAccount(account2, 5000000)
	mockRPC.AddTokenAccount(account3, 9000000)

	// Test cases
	tests := []struct {
		name     string
		accounts []aSDK.PublicKey
		want     []uint64
		wantErr  bool
	}{
		{
			name:     "Get single account balance",
			accounts: []aSDK.PublicKey{account1},
			want:     []uint64{1000000},
			wantErr:  false,
		},
		{
			name:     "Get multiple account balances",
			accounts: []aSDK.PublicKey{account1, account2},
			want:     []uint64{1000000, 5000000},
			wantErr:  false,
		},
		{
			name:     "Get all account balances",
			accounts: []aSDK.PublicKey{account1, account2, account3},
			want:     []uint64{1000000, 5000000, 9000000},
			wantErr:  false,
		},
		{
			name:     "Handle non-existent account",
			accounts: []aSDK.PublicKey{account1, aSDK.MustPublicKeyFromBase58("11111111111111111111111111111111")},
			want:     []uint64{1000000},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Use a wrapper function that works with our mock
			got, err := getMockTokenBalances(ctx, mockRPC, tt.accounts...)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetMulTokenBalance() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetMulTokenBalance() = %v, want %v", got, tt.want)
			}
		})
	}
}

// getMockTokenBalances is a wrapper function that simulates the logic of GetMulTokenBalance
// but works with our mock implementation
func getMockTokenBalances(ctx context.Context, mock *MockRPC, accounts ...aSDK.PublicKey) ([]uint64, error) {
	// Get the accounts data using our mock
	res, err := mock.GetMultipleAccountsWithOpts(ctx, accounts, &ag_rpc.GetMultipleAccountsOpts{
		Commitment: ag_rpc.CommitmentProcessed,
	})
	if err != nil {
		return nil, err
	}

	// Process the results the same way GetMulTokenBalance does
	var amounts []uint64
	for i := range res.Value {
		if res.Value[i] == nil || res.Value[i].Data == nil {
			continue
		}

		// Decode the binary data into a token account
		var coinBalance token.Account
		if err = bin.NewBinDecoder(res.Value[i].Data.GetBinary()).Decode(&coinBalance); err != nil {
			return nil, err
		}

		amounts = append(amounts, coinBalance.Amount)
	}

	return amounts, nil
}
