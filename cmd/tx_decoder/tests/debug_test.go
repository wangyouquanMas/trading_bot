package tests

import (
	"context"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// TestDebugPrintingIssue tests for console output issues in the debugger
func TestDebugPrintingIssue(t *testing.T) {
	// First, test with regular printing
	t.Log("Testing regular println behavior")
	println("This should appear with newline")
	print("This should not have a newline")
	print("This should be on the same line")
	println("\nThis should be on a new line")

	// Now test with formatting
	t.Log("Testing fmt.Printf behavior")
	print("1 waiting for results")
	print("2 waiting for results")
	println()

	// Test with proper printing
	t.Log("Testing proper printf with newlines")
	print("1 waiting for results\n")
	print("2 waiting for results\n")
}

// TestContextTimeout tests behavior with various context timeouts
func TestContextTimeout(t *testing.T) {
	// Test very short timeout
	t.Run("ShortTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond)

		if ctx.Err() == nil {
			t.Error("Context should have timed out")
		} else {
			t.Logf("Context correctly timed out: %v", ctx.Err())
		}
	})

	// Test reasonable timeout
	t.Run("ReasonableTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		select {
		case <-ctx.Done():
			t.Error("Context should not time out yet")
		case <-time.After(10 * time.Millisecond):
			t.Log("Context did not time out yet (correct)")
		}

		time.Sleep(110 * time.Millisecond)

		if ctx.Err() == nil {
			t.Error("Context should have timed out by now")
		} else {
			t.Logf("Context correctly timed out: %v", ctx.Err())
		}
	})
}

// TestMockWebSocketResponse simulates a WebSocket response
func TestMockWebSocketResponse(t *testing.T) {
	// Create a mock WebSocket response
	type MockWebSocketResponse struct {
		Signature string
		Logs      []string
		Success   bool
	}

	mockResponse := MockWebSocketResponse{
		Signature: "5SHT9PwxFE7BNmSQwU4KjAW16LQ5aEZmUvWKqSCamXKkWQBs1DcYkEv7ujWgASRUUKqYy6VsM7iTgJkgAygCVPZB",
		Logs: []string{
			"Program 9xQeWvG816bUx9EPjHmaT23yvVM2ZWbrrpZb9PusVFin invoke [1]",
			"Program log: Instruction: Exchange",
			"Program 11111111111111111111111111111111 invoke [2]",
			"Program 11111111111111111111111111111111 success",
			"Program 9xQeWvG816bUx9EPjHmaT23yvVM2ZWbrrpZb9PusVFin success",
		},
		Success: true,
	}

	t.Logf("Mock WebSocket response with signature: %s", mockResponse.Signature)
	t.Logf("Mock logs: %v", mockResponse.Logs)

	// Simulate processing the mock response
	t.Logf("Simulation: Processing WebSocket message")

	// Check if our target account is mentioned in the logs
	targetAccount := solana.MustPublicKeyFromBase58(knownActiveAccount)
	found := false

	for _, log := range mockResponse.Logs {
		if log == "Program "+targetAccount.String()+" invoke [1]" {
			found = true
			break
		}
	}

	if found {
		t.Logf("Successfully found target account in transaction logs")
	} else {
		t.Logf("Target account not explicitly mentioned in transaction logs")
	}
}

// TestCommitmentLevelDelay tests the typical delay between different commitment levels
func TestCommitmentLevelDelay(t *testing.T) {
	// Skip in short mode as this is a longer test
	if testing.Short() {
		t.Skip("Skipping commitment level delay test in short mode")
	}

	ctx, cancel, rpcClient, _, testAccount := setupTest(t)
	defer cancel()

	// Get the most recent transaction for our test account
	sigs, err := rpcClient.GetSignaturesForAddress(ctx, testAccount)
	if err != nil || len(sigs) == 0 {
		t.Skipf("No recent transactions or error: %v", err)
		return
	}

	recentSig := sigs[0].Signature
	t.Logf("Testing with recent signature: %s", recentSig)

	// Test if the transaction is visible at different commitment levels
	levels := []rpc.CommitmentType{
		rpc.CommitmentProcessed,
		rpc.CommitmentConfirmed,
		rpc.CommitmentFinalized,
	}

	results := make(map[rpc.CommitmentType]bool)

	for _, level := range levels {
		tx, err := rpcClient.GetTransaction(ctx, recentSig, &rpc.GetTransactionOpts{
			Commitment: level,
		})

		results[level] = (err == nil && tx != nil)
		t.Logf("Commitment %s: Transaction visible = %v", level, results[level])
	}

	// Check if we see the expected pattern (processed before confirmed before finalized)
	if !results[rpc.CommitmentProcessed] && (results[rpc.CommitmentConfirmed] || results[rpc.CommitmentFinalized]) {
		t.Logf("Unusual pattern: Transaction not visible at processed but visible at higher level")
	}

	if !results[rpc.CommitmentConfirmed] && results[rpc.CommitmentFinalized] {
		t.Logf("Unusual pattern: Transaction not visible at confirmed but visible at finalized")
	}
}
