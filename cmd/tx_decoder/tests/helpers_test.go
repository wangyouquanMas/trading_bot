package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

// Constants for testing
const (
	defaultWSEndpoint  = "wss://api.mainnet-beta.solana.com"
	defaultRPCEndpoint = "https://api.mainnet-beta.solana.com"
	// Serum DEX program ID - frequently used and should have recent transactions
	knownActiveAccount = "9xQeWvG816bUx9EPjHmaT23yvVM2ZWbrrpZb9PusVFin"
	// A known transaction signature - update this if it becomes too old
	knownTransaction = "5SHT9PwxFE7BNmSQwU4KjAW16LQ5aEZmUvWKqSCamXKkWQBs1DcYkEv7ujWgASRUUKqYy6VsM7iTgJkgAygCVPZB"
)

// Helper function to get the WebSocket endpoint from environment or use default
func getWSEndpoint() string {
	if endpoint := os.Getenv("TEST_WS_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	return defaultWSEndpoint
}

// Helper function to get the RPC endpoint from environment or use default
func getRPCEndpoint() string {
	if endpoint := os.Getenv("TEST_RPC_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	return defaultRPCEndpoint
}

// Helper function to get a test account public key
func getTestAccount() solana.PublicKey {
	if account := os.Getenv("TEST_ACCOUNT"); account != "" {
		pubkey, err := solana.PublicKeyFromBase58(account)
		if err == nil {
			return pubkey
		}
	}
	return solana.MustPublicKeyFromBase58(knownActiveAccount)
}

// Helper function to create a WebSocket client
func createWSClient(t *testing.T, ctx context.Context) *ws.Client {
	wsEndpoint := getWSEndpoint()
	t.Logf("Using WebSocket endpoint: %s", wsEndpoint)

	client, err := ws.Connect(ctx, wsEndpoint)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	return client
}

// Helper function to create an RPC client
func createRPCClient(t *testing.T) *rpc.Client {
	rpcEndpoint := getRPCEndpoint()
	t.Logf("Using RPC endpoint: %s", rpcEndpoint)
	return rpc.New(rpcEndpoint)
}

// Helper to test all commitment levels
func testAllCommitmentLevels(t *testing.T, testFunc func(t *testing.T, commitment rpc.CommitmentType)) {
	commitmentLevels := []rpc.CommitmentType{
		rpc.CommitmentProcessed,
		rpc.CommitmentConfirmed,
		rpc.CommitmentFinalized,
	}

	for _, commitment := range commitmentLevels {
		t.Run(string(commitment), func(t *testing.T) {
			testFunc(t, commitment)
		})
	}
}

// Helper to wait for channel with timeout
func waitForChannelWithTimeout(t *testing.T, ch <-chan bool, timeout time.Duration, successMsg, timeoutMsg string) bool {
	select {
	case <-ch:
		t.Log(successMsg)
		return true
	case <-time.After(timeout):
		t.Log(timeoutMsg)
		return false
	}
}

// Helper function to setup a common test environment
func setupTest(t *testing.T) (context.Context, context.CancelFunc, *rpc.Client, *ws.Client, solana.PublicKey) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	// Get test account
	testAccount := getTestAccount()
	t.Logf("Using test account: %s", testAccount.String())

	// Create clients
	rpcClient := createRPCClient(t)
	wsClient := createWSClient(t, ctx)

	return ctx, cancel, rpcClient, wsClient, testAccount
}
