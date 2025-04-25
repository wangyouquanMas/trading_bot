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

// TestWebSocketConnection tests the connection to the Solana WebSocket endpoint
func TestWebSocketConnection(t *testing.T) {
	// Define test timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use test endpoint or a real endpoint that's reliable
	wsEndpoint := "wss://api.mainnet-beta.solana.com"

	// Try to connect
	wsClient, err := ws.Connect(ctx, wsEndpoint)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket endpoint %s: %v", wsEndpoint, err)
	}
	defer wsClient.Close()

	// If we reach here, connection was successful
	t.Logf("Successfully connected to WebSocket endpoint: %s", wsEndpoint)
}

// TestLogSubscribeMentions tests the subscription to logs mentioning a specific account
func TestLogSubscribeMentions(t *testing.T) {
	// Use a longer timeout to allow for real transaction detection
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get account from environment or use default
	accountAddress := os.Getenv("TEST_ACCOUNT")
	if accountAddress == "" {
		accountAddress = "Csd779Qwsrf1FH1eeLQnNDcxknyCvJteJtVW2MLFr4y3"
	}
	testAccount := solana.MustPublicKeyFromBase58(accountAddress)
	t.Logf("Monitoring account: %s", testAccount.String())

	// Connect to WebSocket
	wsEndpoint := getWSEndpoint()
	t.Logf("Using WebSocket endpoint: %s", wsEndpoint)

	wsClient, err := ws.Connect(ctx, wsEndpoint)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket endpoint: %v", err)
	}
	defer wsClient.Close()

	// Create a channel to signal when we've received a transaction
	transactionReceived := make(chan bool, 1)

	// Try with confirmed commitment - most reliable for this test
	commitment := rpc.CommitmentFinalized
	t.Logf("Subscribing with commitment level: %s", commitment)

	// Subscribe to logs
	sub, err := wsClient.LogsSubscribeMentions(
		testAccount,
		commitment,
	)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	t.Logf("Successfully subscribed - waiting for transactions...")

	// Start a goroutine to receive messages
	go func() {
		for {
			result, err := sub.Recv(ctx)
			if err != nil {
				if err == context.DeadlineExceeded || err == context.Canceled {
					t.Logf("Context closed: %v", err)
					return
				}
				t.Logf("Error receiving log: %v", err)
				continue
			}

			t.Logf("TRANSACTION DETECTED! Signature: %s", result.Value.Signature.String())
			if len(result.Value.Logs) > 0 {
				t.Logf("First log entry: %s", result.Value.Logs[0])
			}

			// Signal that we've received a transaction
			transactionReceived <- true
			return
		}
	}()

	// Wait for either a transaction to be received or a timeout
	select {
	case <-transactionReceived:
		t.Logf("✅ Successfully received a transaction for the monitored account")
	case <-time.After(45 * time.Second):
		t.Errorf("❌ No transactions received within timeout period. Try sending a transaction involving the account %s", testAccount.String())
	case <-ctx.Done():
		t.Logf("Test context closed: %v", ctx.Err())
	}
}

// TestHeartbeat tests if the WebSocket connection stays alive
func TestHeartbeat(t *testing.T) {
	// Define test timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use test endpoint
	wsEndpoint := "wss://api.mainnet-beta.solana.com"

	// Connect to WebSocket
	wsClient, err := ws.Connect(ctx, wsEndpoint)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket endpoint %s: %v", wsEndpoint, err)
	}
	defer wsClient.Close()

	// Subscribe to slot updates as a heartbeat
	slotSub, err := wsClient.SlotSubscribe()
	if err != nil {
		t.Fatalf("Failed to subscribe to slot updates: %v", err)
	}
	defer slotSub.Unsubscribe()

	// Try to receive slot updates for a short period
	heartbeatCtx, heartbeatCancel := context.WithTimeout(ctx, 10*time.Second)
	defer heartbeatCancel()

	// Create a channel to track if we received any updates
	receivedUpdate := make(chan bool, 1)

	go func() {
		result, err := slotSub.Recv(heartbeatCtx)
		if err == nil {
			t.Logf("Received slot update: %+v", result)
			receivedUpdate <- true
		} else if err != context.DeadlineExceeded {
			t.Logf("Error receiving slot update: %v", err)
		}
	}()

	// Wait to see if we receive any updates
	select {
	case <-receivedUpdate:
		t.Logf("Heartbeat successful - received slot update")
	case <-heartbeatCtx.Done():
		t.Logf("No slot updates received within timeout period")
	}
}

// TestAccountSubscribe tests the subscription to account updates
func TestAccountSubscribe(t *testing.T) {
	// Define test timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Use test endpoint
	wsEndpoint := "wss://api.mainnet-beta.solana.com"

	// Connect to WebSocket
	wsClient, err := ws.Connect(ctx, wsEndpoint)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket endpoint %s: %v", wsEndpoint, err)
	}
	defer wsClient.Close()

	// Use a well-known account (e.g., Serum DEX)
	testAccount := solana.MustPublicKeyFromBase58("9xQeWvG816bUx9EPjHmaT23yvVM2ZWbrrpZb9PusVFin")

	// Subscribe to account updates
	sub, err := wsClient.AccountSubscribe(
		testAccount,
		rpc.CommitmentConfirmed,
	)
	if err != nil {
		t.Fatalf("Failed to subscribe to account: %v", err)
	}
	defer sub.Unsubscribe()

	// Successfully subscribed
	t.Logf("Successfully subscribed to account %s", testAccount.String())
}

// TestLogSubscribeAll tests the subscription to all logs
func TestLogSubscribeAll(t *testing.T) {
	// Define test timeout
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Use test endpoint
	wsEndpoint := "wss://api.mainnet-beta.solana.com"

	// Connect to WebSocket
	wsClient, err := ws.Connect(ctx, wsEndpoint)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket endpoint %s: %v", wsEndpoint, err)
	}
	defer wsClient.Close()

	// Subscribe to all logs
	sub, err := wsClient.LogsSubscribe(
		ws.LogsSubscribeFilterAll,
		rpc.CommitmentConfirmed,
	)
	if err != nil {
		t.Fatalf("Failed to subscribe to all logs: %v", err)
	}
	defer sub.Unsubscribe()

	// Successfully subscribed
	t.Logf("Successfully subscribed to all logs")

	// Try to receive some logs within a short timeout
	logCtx, logCancel := context.WithTimeout(ctx, 5*time.Second)
	defer logCancel()

	receivedLog := make(chan bool, 1)

	go func() {
		result, err := sub.Recv(logCtx)
		if err == nil {
			t.Logf("Received log: Signature %s", result.Value.Signature.String())
			receivedLog <- true
		} else if err != context.DeadlineExceeded {
			t.Logf("Error receiving log: %v", err)
		}
	}()

	// Wait to see if we receive any logs
	select {
	case <-receivedLog:
		t.Logf("Successfully received a log")
	case <-logCtx.Done():
		t.Logf("No logs received within timeout period (expected for short timeouts)")
	}
}
