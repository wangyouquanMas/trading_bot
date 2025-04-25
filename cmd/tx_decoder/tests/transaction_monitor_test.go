package tests

import (
	"context"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

// TestTransactionVisibility checks if recent transactions for a known account are retrievable
func TestTransactionVisibility(t *testing.T) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a well-known account (e.g., Serum DEX)
	testAccount := solana.MustPublicKeyFromBase58("Csd779Qwsrf1FH1eeLQnNDcxknyCvJteJtVW2MLFr4y3")

	// Create RPC client
	rpcClient := rpc.New("https://api.mainnet-beta.solana.com")

	// Try to get recent signatures
	sigs, err := rpcClient.GetSignaturesForAddress(ctx, testAccount)
	if err != nil {
		t.Fatalf("Failed to get signatures for address: %v", err)
	}

	if len(sigs) == 0 {
		t.Logf("No signatures found for account. This is unusual for an active account.")
	} else {
		t.Logf("Found %d signatures for account", len(sigs))

		// Check if we can get the transaction for the most recent signature
		if len(sigs) > 0 {
			mostRecentSig := sigs[0].Signature
			t.Logf("Most recent signature: %s", mostRecentSig.String())

			tx, err := rpcClient.GetTransaction(ctx, mostRecentSig, &rpc.GetTransactionOpts{
				Commitment: rpc.CommitmentConfirmed,
			})

			if err != nil {
				t.Errorf("Failed to get transaction details: %v", err)
			} else if tx == nil {
				t.Errorf("Transaction returned nil")
			} else {
				t.Logf("Successfully retrieved transaction details")
			}
		}
	}
}

// TestCompareCommitmentLevels compares different commitment levels for transaction visibility
func TestCompareCommitmentLevels(t *testing.T) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a well-known account
	testAccount := solana.MustPublicKeyFromBase58("9xQeWvG816bUx9EPjHmaT23yvVM2ZWbrrpZb9PusVFin")

	// Create RPC client
	rpcClient := rpc.New("https://api.mainnet-beta.solana.com")

	// Try different commitment levels
	commitmentLevels := []rpc.CommitmentType{
		rpc.CommitmentProcessed,
		rpc.CommitmentConfirmed,
		rpc.CommitmentFinalized,
	}

	for _, commitment := range commitmentLevels {
		t.Logf("Testing with commitment level: %s", commitment)

		// Try to get recent signatures with this commitment level
		opts := &rpc.GetSignaturesForAddressOpts{
			Commitment: commitment,
		}

		sigs, err := rpcClient.GetSignaturesForAddressWithOpts(ctx, testAccount, opts)
		if err != nil {
			t.Errorf("Failed to get signatures with commitment %s: %v", commitment, err)
			continue
		}

		t.Logf("Found %d signatures with commitment %s", len(sigs), commitment)
	}
}

// TestCombinedApproach tests a combined approach of WebSocket monitoring and RPC verification
func TestCombinedApproach(t *testing.T) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a well-known account
	testAccount := solana.MustPublicKeyFromBase58("9xQeWvG816bUx9EPjHmaT23yvVM2ZWbrrpZb9PusVFin")

	// Create RPC client
	rpcClient := rpc.New("https://api.mainnet-beta.solana.com")

	// Connect to WebSocket
	wsClient, err := ws.Connect(ctx, "wss://api.mainnet-beta.solana.com")
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer wsClient.Close()

	// Try to get recent signatures first (RPC approach)
	sigs, err := rpcClient.GetSignaturesForAddress(ctx, testAccount)
	if err != nil {
		t.Fatalf("Failed to get signatures for address: %v", err)
	}

	if len(sigs) == 0 {
		t.Logf("No signatures found for account via RPC")
	} else {
		t.Logf("Found %d signatures for account via RPC", len(sigs))
	}

	// Now try WebSocket subscription
	sub, err := wsClient.LogsSubscribeMentions(
		testAccount,
		rpc.CommitmentConfirmed,
	)
	if err != nil {
		t.Fatalf("Failed to subscribe to logs: %v", err)
	}
	defer sub.Unsubscribe()

	t.Logf("Successfully subscribed to logs for account via WebSocket")

	// Try to receive some logs within a short timeout
	wsCtx, wsCancel := context.WithTimeout(ctx, 5*time.Second)
	defer wsCancel()

	receivedLog := make(chan bool, 1)

	go func() {
		result, err := sub.Recv(wsCtx)
		if err == nil {
			t.Logf("Received transaction via WebSocket: %s", result.Value.Signature.String())
			receivedLog <- true
		} else if err != context.DeadlineExceeded {
			t.Logf("Error receiving log via WebSocket: %v", err)
		}
	}()

	// Wait to see if we receive any logs
	select {
	case <-receivedLog:
		t.Logf("Successfully received a transaction via WebSocket")
	case <-wsCtx.Done():
		t.Logf("No transactions received via WebSocket within timeout period (expected)")
	}
}

// TestTransactionRetrieval tests retrieving a known transaction
func TestTransactionRetrieval(t *testing.T) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create RPC client
	rpcClient := rpc.New("https://api.mainnet-beta.solana.com")

	// Use a known transaction signature - this should be updated to a recent one
	knownSig, err := solana.SignatureFromBase58("5SHT9PwxFE7BNmSQwU4KjAW16LQ5aEZmUvWKqSCamXKkWQBs1DcYkEv7ujWgASRUUKqYy6VsM7iTgJkgAygCVPZB")
	if err != nil {
		t.Fatalf("Invalid signature format: %v", err)
	}

	// Try different commitment levels
	commitmentLevels := []rpc.CommitmentType{
		rpc.CommitmentConfirmed,
		rpc.CommitmentFinalized,
	}

	for _, commitment := range commitmentLevels {
		t.Logf("Testing transaction retrieval with commitment level: %s", commitment)

		tx, err := rpcClient.GetTransaction(ctx, knownSig, &rpc.GetTransactionOpts{
			Commitment: commitment,
		})

		if err != nil {
			t.Errorf("Failed to get transaction with commitment %s: %v", commitment, err)
		} else if tx == nil {
			t.Errorf("Transaction returned nil with commitment %s", commitment)
		} else {
			t.Logf("Successfully retrieved transaction with commitment %s", commitment)

			// Check if transaction has metadata
			if tx.Meta == nil {
				t.Logf("Transaction has no metadata")
			} else {
				t.Logf("Transaction has metadata - Fee: %d lamports", tx.Meta.Fee)
			}
		}
	}
}
