package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	confirm "github.com/gagliardetto/solana-go/rpc/sendAndConfirmTransaction"
	"github.com/gagliardetto/solana-go/rpc/ws"
)

// Constants for PumpFun AMM swap
const (
	pumpSwapProgramID = "pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA"
)

// TestSendPumpFunTransaction sends a real transaction to test WebSocket monitoring
// This test requires a funded account with SOL for transaction fees
// Run with: go test -v -run TestSendPumpFunTransaction
func TestSendPumpFunTransaction(t *testing.T) {
	// Skip this test by default so it doesn't run during regular test runs
	// Remove this line when you want to send an actual transaction
	t.Skip("Skipping transaction sending test - remove this line to enable")

	// Define timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get private key from environment variable
	// Format should be base58 encoded private key
	privateKeyBase58 := os.Getenv("TEST_WALLET_PRIVATE_KEY")
	if privateKeyBase58 == "" {
		t.Fatalf("TEST_WALLET_PRIVATE_KEY environment variable not set. " +
			"This test requires a funded account private key to send transactions.")
	}

	// Parse private key
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyBase58)
	if err != nil {
		t.Fatalf("Invalid private key: %v", err)
	}

	// Create wallet from private key
	wallet := solana.Wallet{
		PrivateKey: privateKey,
	}

	// Get the monitored account from environment or use default
	monitoredAccount := os.Getenv("TEST_ACCOUNT")
	if monitoredAccount == "" {
		monitoredAccount = "Csd779Qwsrf1FH1eeLQnNDcxknyCvJteJtVW2MLFr4y3"
	}

	// Create RPC client
	rpcClient := rpc.New(getRPCEndpoint())

	// Create and send a PumpFun swap transaction
	// This is a simplified example - in a real implementation,
	// you'll need the actual PumpFun AMM contract interface
	tx, err := sendSimpleTransaction(ctx, rpcClient, &wallet, monitoredAccount)
	if err != nil {
		t.Fatalf("Failed to send transaction: %v", err)
	}

	t.Logf("✅ Transaction sent! Signature: %s", tx.String())
	t.Logf("View on Solana Explorer: https://explorer.solana.com/tx/%s", tx.String())
}

// TestSendSimpleSOLTransfer sends a simple SOL transfer to the monitored account
// This is a simpler alternative if you can't interact with PumpFun directly
func TestSendSimpleSOLTransfer(t *testing.T) {
	// Skip this test by default so it doesn't run during regular test runs
	// Remove this line when you want to send an actual transaction
	t.Skip("Skipping SOL transfer test - remove this line to enable")

	// Get private key from environment variable
	privateKeyBase58 := os.Getenv("TEST_WALLET_PRIVATE_KEY")
	if privateKeyBase58 == "" {
		t.Fatalf("TEST_WALLET_PRIVATE_KEY environment variable not set")
	}

	// Parse private key
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyBase58)
	if err != nil {
		t.Fatalf("Invalid private key: %v", err)
	}

	// Get the monitored account from environment or use default
	monitoredAccountStr := os.Getenv("TEST_ACCOUNT")
	if monitoredAccountStr == "" {
		monitoredAccountStr = "Csd779Qwsrf1FH1eeLQnNDcxknyCvJteJtVW2MLFr4y3"
	}
	monitoredAccount, err := solana.PublicKeyFromBase58(monitoredAccountStr)
	if err != nil {
		t.Fatalf("Invalid monitored account: %v", err)
	}

	// Create RPC client
	rpcClient := rpc.New(getRPCEndpoint())
	wsClient, err := ws.Connect(context.Background(), getWSEndpoint())
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer wsClient.Close()

	// Create a simple SOL transfer transaction
	recentBlockhash, err := rpcClient.GetRecentBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		t.Fatalf("Failed to get recent blockhash: %v", err)
	}

	// Create a transfer instruction (0.001 SOL = 1,000,000 lamports)
	transferAmount := uint64(1000000) // 0.001 SOL
	transferIx := system.NewTransferInstruction(
		transferAmount,
		privateKey.PublicKey(),
		monitoredAccount,
	).Build()

	// Build the transaction
	tx, err := solana.NewTransaction(
		[]solana.Instruction{transferIx},
		recentBlockhash.Value.Blockhash,
		solana.TransactionPayer(privateKey.PublicKey()),
	)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Sign the transaction
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if privateKey.PublicKey().Equals(key) {
			return &privateKey
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to sign transaction: %v", err)
	}

	// Send the transaction
	sig, err := confirm.SendAndConfirmTransaction(
		context.Background(),
		rpcClient,
		wsClient,
		tx,
	)
	if err != nil {
		t.Fatalf("Failed to send and confirm transaction: %v", err)
	}

	t.Logf("✅ Transaction sent! Signature: %s", sig.String())
	t.Logf("✅ SOL transferred to %s", monitoredAccount.String())
	t.Logf("View on Solana Explorer: https://explorer.solana.com/tx/%s", sig.String())
}

// A simplified transaction sending function
// In a real implementation, you'd use the actual PumpFun AMM API
func sendSimpleTransaction(ctx context.Context, client *rpc.Client, sender *solana.Wallet, monitoredAccount string) (solana.Signature, error) {
	// This is a placeholder - in a real implementation, replace this with actual
	// PumpFun AMM contract interaction code

	// For this example, we'll just send a small SOL transfer to simulate activity
	// Get a recent blockhash
	recentBlockhash, err := client.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to get recent blockhash: %w", err)
	}

	// Parse monitored account
	receiverPubkey, err := solana.PublicKeyFromBase58(monitoredAccount)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("invalid monitored account: %w", err)
	}

	// Create a transfer instruction (0.001 SOL = 1,000,000 lamports)
	transferAmount := uint64(1000000) // 0.001 SOL
	transferIx := system.NewTransferInstruction(
		transferAmount,
		sender.PublicKey(),
		receiverPubkey,
	).Build()

	// Build the transaction
	tx, err := solana.NewTransaction(
		[]solana.Instruction{transferIx},
		recentBlockhash.Value.Blockhash,
		solana.TransactionPayer(sender.PublicKey()),
	)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to create transaction: %w", err)
	}

	// Sign the transaction
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if sender.PublicKey().Equals(key) {
			privateKey := sender.PrivateKey
			return &privateKey
		}
		return nil
	})
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send the transaction
	wsClient, err := ws.Connect(ctx, getWSEndpoint())
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	defer wsClient.Close()

	sig, err := confirm.SendAndConfirmTransaction(
		ctx,
		client,
		wsClient,
		tx,
	)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to send and confirm transaction: %w", err)
	}

	return sig, nil
}

// TestMonitorTransactionAfterSending combines the transaction sending and monitoring
// This test sends a transaction and then monitors for it using WebSocket
func TestMonitorTransactionAfterSending(t *testing.T) {
	// Skip this test by default
	t.Skip("Skipping combined test - remove this line to enable")

	// Define timeout
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Get private key from environment variable
	privateKeyBase58 := os.Getenv("TEST_WALLET_PRIVATE_KEY")
	if privateKeyBase58 == "" {
		t.Fatalf("TEST_WALLET_PRIVATE_KEY environment variable not set")
	}

	// Parse private key
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyBase58)
	if err != nil {
		t.Fatalf("Invalid private key: %v", err)
	}

	// Get the monitored account
	monitoredAccountStr := os.Getenv("TEST_ACCOUNT")
	if monitoredAccountStr == "" {
		monitoredAccountStr = "Csd779Qwsrf1FH1eeLQnNDcxknyCvJteJtVW2MLFr4y3"
	}
	monitoredAccount, err := solana.PublicKeyFromBase58(monitoredAccountStr)
	if err != nil {
		t.Fatalf("Invalid monitored account: %v", err)
	}

	// Create clients
	rpcClient := rpc.New(getRPCEndpoint())
	wsClient, err := ws.Connect(ctx, getWSEndpoint())
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer wsClient.Close()

	// Setup WebSocket monitoring first
	t.Log("Setting up WebSocket monitoring...")

	// Create channel to receive transaction notifications
	transactionReceived := make(chan solana.Signature, 1)

	// Subscribe to logs for the monitored account
	sub, err := wsClient.LogsSubscribeMentions(
		monitoredAccount,
		rpc.CommitmentConfirmed,
	)
	if err != nil {
		t.Fatalf("Failed to subscribe to account logs: %v", err)
	}
	defer sub.Unsubscribe()

	// Start monitoring in a goroutine
	go func() {
		for {
			result, err := sub.Recv(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return // Context cancelled
				}
				t.Logf("Error receiving logs: %v", err)
				continue
			}

			t.Logf("Transaction detected via WebSocket! Signature: %s", result.Value.Signature)
			transactionReceived <- result.Value.Signature
			return
		}
	}()

	// Wait a moment to ensure subscription is active
	time.Sleep(2 * time.Second)

	// Now send the transaction
	t.Log("Sending test transaction...")

	// Create a simple SOL transfer
	recentBlockhash, err := rpcClient.GetRecentBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		t.Fatalf("Failed to get recent blockhash: %v", err)
	}

	// Create a transfer instruction (0.001 SOL)
	transferAmount := uint64(1000000)
	transferIx := system.NewTransferInstruction(
		transferAmount,
		privateKey.PublicKey(),
		monitoredAccount,
	).Build()

	// Build transaction
	tx, err := solana.NewTransaction(
		[]solana.Instruction{transferIx},
		recentBlockhash.Value.Blockhash,
		solana.TransactionPayer(privateKey.PublicKey()),
	)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Sign transaction
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if privateKey.PublicKey().Equals(key) {
			return &privateKey
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to sign transaction: %v", err)
	}

	// Send transaction
	sig, err := confirm.SendAndConfirmTransaction(
		ctx,
		rpcClient,
		wsClient,
		tx,
	)
	if err != nil {
		t.Fatalf("Failed to send transaction: %v", err)
	}

	t.Logf("Transaction sent! Signature: %s", sig.String())
	t.Logf("View on Solana Explorer: https://explorer.solana.com/tx/%s", sig.String())

	// Wait for our monitoring to detect the transaction
	select {
	case receivedSig := <-transactionReceived:
		if receivedSig.Equals(sig) {
			t.Logf("✅ SUCCESS: WebSocket detected the transaction we just sent!")
		} else {
			t.Logf("WebSocket detected a different transaction: %s", receivedSig.String())
		}
	case <-time.After(60 * time.Second):
		t.Errorf("❌ WebSocket did not detect the transaction within timeout period")
	case <-ctx.Done():
		t.Logf("Test cancelled: %v", ctx.Err())
	}
}
