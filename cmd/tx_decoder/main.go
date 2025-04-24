package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	token "github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/mr-tron/base58"
)

// pumpSwapProgramID is the program ID for PumpSwap AMM
const pumpSwapProgramID = "pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA"

// Token metadata program ID
const tokenMetadataProgramID = "metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s"

// Fallback RPC endpoints in case the primary one fails
var fallbackRPCEndpoints = []string{
	"https://api.mainnet-beta.solana.com",
	"https://solana-api.projectserum.com",
	"https://rpc.ankr.com/solana",
	"https://solana-mainnet.g.alchemy.com/v2/demo",
	"https://mainnet.rpcpool.com",
}

// Maximum number of retry attempts for RPC requests
const maxRetries = 3

// Instruction discriminators for PumpSwap AMM (8-byte identifiers)
var (
	// These are the 8-byte discriminators for PumpSwap instructions
	// From the IDL file: Instruction_Buy = ag_binary.TypeID([8]byte{102, 6, 61, 18, 1, 218, 235, 234})
	BuyDiscriminator        = []byte{102, 6, 61, 18, 1, 218, 235, 234}
	SellDiscriminator       = []byte{143, 244, 89, 80, 224, 16, 16, 88}  // Example value - replace with correct one
	CreatePoolDiscriminator = []byte{175, 175, 109, 31, 13, 152, 155, 9} // Example value - replace with correct one
)

// TokenInfo represents detailed information about a token
type TokenInfo struct {
	Symbol      string
	Name        string
	Decimals    uint8
	Description string
	Image       string
	Website     string
	Twitter     string
	Telegram    string
}

// TokenMetadata represents token metadata from the chain
type TokenMetadata struct {
	Key        uint8    `json:"key"`
	UpdateAuth string   `json:"update_auth"`
	Mint       string   `json:"mint"`
	Data       MetaData `json:"data"`
}

// MetaData represents the core metadata fields
type MetaData struct {
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
	Uri    string `json:"uri"`
}

// TokenUriData represents the JSON structure from a token's URI
type TokenUriData struct {
	Name        string `json:"name"`
	Symbol      string `json:"symbol"`
	Description string `json:"description"`
	Image       string `json:"image"`
	Website     string `json:"website"`
	Twitter     string `json:"twitter"`
	Telegram    string `json:"telegram"`
	Extensions  struct {
		Website  string `json:"website"`
		Twitter  string `json:"twitter"`
		Telegram string `json:"telegram"`
	} `json:"extensions"`
}

// TokenCache to avoid redundant lookups during a session
var tokenCache = make(map[string]*TokenInfo)

func main() {
	// Display usage information if requested
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		printUsage()
		return
	}

	// Process commands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "decode":
			// If the decode command is provided, shift arguments to pass to decodeTxCmd
			os.Args = os.Args[1:]
			decodeTxCmd()
		case "decode-tx":
			// Decode a specific transaction by signature
			if len(os.Args) < 3 {
				fmt.Println("Error: Transaction signature required")
				printUsage()
				os.Exit(1)
			}

			// Get the transaction signature
			txSignature := os.Args[2]

			//output txSingautre
			fmt.Println("tx signature is:", txSignature)

			// Get RPC endpoint from environment or use default
			rpcEndpoint := os.Getenv("RPC_ENDPOINT")
			if rpcEndpoint == "" {
				rpcEndpoint = fallbackRPCEndpoints[0]
			}

			// Create context and decode transaction
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			err := decodeSpecificTransaction(ctx, rpcEndpoint, txSignature)
			if err != nil {
				fmt.Printf("Error with primary RPC endpoint: %v\n", err)
				fmt.Println("Trying fallback RPC endpoints...")

				// Try fallback endpoints
				var success bool
				for i, endpoint := range fallbackRPCEndpoints {
					if endpoint == rpcEndpoint {
						continue // Skip the one we already tried
					}

					fmt.Printf("Trying fallback endpoint #%d: %s\n", i+1, endpoint)
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					err := decodeSpecificTransaction(ctx, endpoint, txSignature)
					cancel()

					if err == nil {
						success = true
						break
					}
					fmt.Printf("Fallback endpoint failed: %v\n", err)
				}

				if !success {
					fmt.Println("\nAll RPC endpoints failed. Please try again later or use a custom RPC endpoint:")
					fmt.Println("export RPC_ENDPOINT=\"your-custom-endpoint\"")
					os.Exit(1)
				}
			}
		default:
			// If this is a pool address for decoding, pass it along
			if len(os.Args[1]) > 30 {
				decodeTxCmd()
			} else {
				fmt.Printf("Unknown command: %s\n", os.Args[1])
				printUsage()
				os.Exit(1)
			}
		}
	} else {
		// Run the default transaction decoder if no arguments
		decodeTxCmd()
	}
}

// decodeTxCmd decodes transactions for a given PumpFun AMM pool
func decodeTxCmd() {
	// Get RPC endpoint from environment or use default
	rpcEndpoint := os.Getenv("RPC_ENDPOINT")
	if rpcEndpoint == "" {
		rpcEndpoint = fallbackRPCEndpoints[0]
	}

	// Get the pool address to monitor from args or use default
	poolAddress := "H9d3XHfvMGfoohydEpqh4w3mopnvjCRzE9VqaiHKdqs7" // Default pool address
	if len(os.Args) > 1 {
		poolAddress = os.Args[1]
	}

	fmt.Printf("Analyzing transactions for PumpFun AMM pool: %s\n", poolAddress)

	// Create context with cancellation and timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Handle Ctrl+C to gracefully exit
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\nShutting down...")
		cancel()
		os.Exit(0)
	}()

	// Fetch and decode historical transactions
	limit := 10 // Default limit
	err := getHistoricalTransactions(ctx, rpcEndpoint, poolAddress, limit)
	if err != nil {
		fmt.Printf("Error with primary RPC endpoint: %v\n", err)
		fmt.Println("Trying fallback RPC endpoints...")

		// Try fallback endpoints
		var success bool
		for i, endpoint := range fallbackRPCEndpoints {
			if endpoint == rpcEndpoint {
				continue // Skip the one we already tried
			}

			fmt.Printf("Trying fallback endpoint #%d: %s\n", i+1, endpoint)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			err := getHistoricalTransactions(ctx, endpoint, poolAddress, limit)
			cancel()

			if err == nil {
				success = true
				break
			}
			fmt.Printf("Fallback endpoint failed: %v\n", err)
		}

		if !success {
			log.Fatalf("All RPC endpoints failed. Please try again later or use a custom RPC endpoint")
		}
	}
}

// getHistoricalTransactions fetches and processes historical transactions for an account
func getHistoricalTransactions(ctx context.Context, rpcEndpoint, accountAddress string, limit int) error {
	// Convert account address to PublicKey
	accountPubkey, err := solana.PublicKeyFromBase58(accountAddress)
	if err != nil {
		return fmt.Errorf("invalid account address: %w", err)
	}

	// Create RPC client
	client := rpc.New(rpcEndpoint)

	// Get signatures for the account with retry logic
	var signatures []*rpc.TransactionSignature
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		signatures, err = client.GetSignaturesForAddress(ctx, accountPubkey)
		if err == nil {
			break
		}

		if retryCount < maxRetries-1 {
			fmt.Printf("Failed to get signatures (attempt %d/%d): %v\nRetrying...\n",
				retryCount+1, maxRetries, err)
			time.Sleep(time.Duration(retryCount+1) * time.Second) // Exponential backoff
		}
	}

	if err != nil {
		return fmt.Errorf("failed to get signatures after %d attempts: %w", maxRetries, err)
	}

	// Limit the number of signatures to process
	if len(signatures) > limit {
		signatures = signatures[:limit]
	}

	fmt.Printf("Found %d historical transactions (processing %d)\n", len(signatures), limit)

	// Process each transaction
	for i, sig := range signatures {
		fmt.Printf("\nTransaction %d/%d: %s\n", i+1, len(signatures), sig.Signature.String())

		// Get transaction details with retry logic
		var tx *rpc.GetTransactionResult
		for retryCount := 0; retryCount < maxRetries; retryCount++ {
			tx, err = client.GetTransaction(ctx, sig.Signature, &rpc.GetTransactionOpts{
				Encoding:   solana.EncodingBase64, // Use Base64 encoding for binary data
				Commitment: rpc.CommitmentConfirmed,
			})

			if err == nil {
				break
			}

			if retryCount < maxRetries-1 {
				fmt.Printf("Failed to get transaction (attempt %d/%d): %v\nRetrying...\n",
					retryCount+1, maxRetries, err)
				time.Sleep(time.Duration(retryCount+1) * time.Second) // Exponential backoff
			}
		}

		if err != nil {
			fmt.Printf("Error getting transaction after %d attempts: %v\n", maxRetries, err)
			continue
		}

		// Process the transaction
		analyzeTransactionWithRPC(tx, sig.Signature.String(), rpcEndpoint)
	}

	return nil
}

// analyzeTransaction analyzes a transaction to identify PumpFun AMM operations
func analyzeTransaction(tx *rpc.GetTransactionResult, signature string) {
	// Call overload with default RPC endpoint
	rpcEndpoint := os.Getenv("RPC_ENDPOINT")
	if rpcEndpoint == "" {
		rpcEndpoint = fallbackRPCEndpoints[0]
	}

	analyzeTransactionWithRPC(tx, signature, rpcEndpoint)
}

// analyzeTransactionWithRPC analyzes a transaction with a specific RPC endpoint
func analyzeTransactionWithRPC(tx *rpc.GetTransactionResult, signature string, rpcEndpoint string) {
	if tx == nil {
		fmt.Println("Transaction data is nil")
		return
	}

	fmt.Printf("Transaction signature: %s\n", signature)

	// Display transaction metadata if available
	if tx.Meta != nil {
		// Check for errors
		if tx.Meta.Err != nil {
			fmt.Printf("Transaction failed: %v\n", tx.Meta.Err)
		} else {
			fmt.Println("Transaction successful")
		}

		// Display fee information
		fmt.Printf("Transaction fee: %d lamports\n", tx.Meta.Fee)

		// Initialize detection variables
		isPumpSwap := false
		isSwapInstruction := false
		isBuy := false
		isSell := false

		// Track important transaction data for optimized output
		type TransactionSummary struct {
			Operation    string
			Direction    string
			BaseMint     string
			AmountIn     uint64
			AmountOut    uint64
			BaseMintName string
			TokenInfo    *TokenInfo
		}

		summary := TransactionSummary{
			Operation: "Unknown",
			Direction: "Unknown",
			BaseMint:  "Unknown",
		}

		//outptut tx.Transaciton is nil or not
		fmt.Printf("  Transaction data: %v\n", tx.Transaction == nil)
		fmt.Println("isPumpSwap: ", isPumpSwap)

		if tx.Transaction != nil && !isPumpSwap {
			fmt.Println("\nDecoding transaction data for detailed analysis:")

			// 1. Get the transaction data
			data := tx.Transaction.GetBinary()
			if data != nil {
				// 2. Create a transaction object to decode into
				var decodedTx solana.Transaction

				// 3. Decode the transaction data
				err := decodedTx.UnmarshalWithDecoder(bin.NewBinDecoder(data))
				if err != nil {
					fmt.Printf("  Error decoding transaction: %v\n", err)
				} else {
					fmt.Printf("  Successfully decoded transaction with %d instructions\n",
						len(decodedTx.Message.Instructions))

					// 4. Analyze each instruction in the transaction
					pumpSwapProgID := solana.MustPublicKeyFromBase58(pumpSwapProgramID)

					for i, inst := range decodedTx.Message.Instructions {
						// Get the program ID for this instruction
						programIdIndex := inst.ProgramIDIndex
						if int(programIdIndex) < len(decodedTx.Message.AccountKeys) {
							programID := decodedTx.Message.AccountKeys[programIdIndex]

							// Check if this instruction is for the PumpSwap program
							if programID.Equals(pumpSwapProgID) {
								isPumpSwap = true
								fmt.Printf("  Instruction %d uses PumpSwap program\n", i)

								// The instruction data can be analyzed to determine operation type
								var instDataBytes []byte
								var currentDiscriminator []byte // Store the discriminator for later use

								// Check if inst.Data is already a byte array or needs conversion from base58
								if len(inst.Data) > 0 {
									// If inst.Data looks like a base58 string (Solana sometimes returns it this way)
									// Try to convert it from base58
									dataStr := string(inst.Data)
									var err error
									instDataBytes, err = base58.Decode(dataStr)
									if err != nil {
										// If not a valid base58 string, assume it's already in byte format
										fmt.Printf("  Data is not base58, using raw bytes\n")
										instDataBytes = inst.Data
									} else {
										fmt.Printf("  Converted data from base58 to bytes\n")
									}
								}

								//lenght of instDataByts
								fmt.Printf("  Length of instDataBytes: %d\n", len(instDataBytes))

								if len(instDataBytes) >= 8 {
									// In Anchor programs, the first 8 bytes are the instruction discriminator
									// This is a SHA256 hash of the instruction name
									currentDiscriminator = instDataBytes[:8]
									fmt.Printf("  Instruction discriminator: %v\n", currentDiscriminator)

									// Check if it matches known discriminators
									if bytes.Equal(currentDiscriminator, BuyDiscriminator) {
										isSwapInstruction = true
										isBuy = true
										summary.Operation = "Swap"
										summary.Direction = "Buy (SOL → Token)"

										// If there's enough data, try to decode the parameters
										// In Anchor format, parameters follow the 8-byte discriminator
										if len(instDataBytes) >= 24 { // 8 + 8 + 8 (discriminator + two uint64 params typically)
											// Extract parameters: In Buy instruction we typically have BaseAmountOut and MaxQuoteAmountIn
											baseAmountOut := binary.LittleEndian.Uint64(instDataBytes[8:16])
											maxQuoteAmountIn := binary.LittleEndian.Uint64(instDataBytes[16:24])

											summary.AmountOut = baseAmountOut
											summary.AmountIn = maxQuoteAmountIn

											fmt.Printf("  Buy parameters:\n")
											fmt.Printf("    Base Amount Out: %d (tokens received)\n", baseAmountOut)
											fmt.Printf("    Max Quote Amount In: %d (max SOL to spend)\n", maxQuoteAmountIn)
										}

									} else if bytes.Equal(currentDiscriminator, SellDiscriminator) {
										isSwapInstruction = true
										isSell = true
										summary.Operation = "Swap"
										summary.Direction = "Sell (Token → SOL)"

										// Similar parameter decoding for Sell
										if len(instDataBytes) >= 24 {
											// Extract parameters: In Sell instruction we typically have BaseAmountIn and MinQuoteAmountOut
											baseAmountIn := binary.LittleEndian.Uint64(instDataBytes[8:16])
											minQuoteAmountOut := binary.LittleEndian.Uint64(instDataBytes[16:24])

											summary.AmountIn = baseAmountIn
											summary.AmountOut = minQuoteAmountOut

											fmt.Printf("  Sell parameters:\n")
											fmt.Printf("    Base Amount In: %d (tokens to sell)\n", baseAmountIn)
											fmt.Printf("    Min Quote Amount Out: %d (min SOL to receive)\n", minQuoteAmountOut)
										}

									} else if bytes.Equal(currentDiscriminator, CreatePoolDiscriminator) {
										summary.Operation = "CreatePool"
										fmt.Printf("  Detected CreatePool instruction (matched discriminator)\n")
										// CreatePool parameters would follow a similar pattern if needed
									} else {
										fmt.Printf("  Unknown instruction type with discriminator: %v\n", currentDiscriminator)
										// For debugging, print more of the data
										maxPrint := len(instDataBytes)
										if maxPrint > 32 {
											maxPrint = 32
										}
										fmt.Printf("  Instruction data (first %d bytes): %v\n", maxPrint, instDataBytes[:maxPrint])
									}
								} else {
									fmt.Printf("  Instruction data too short to contain discriminator (len: %d)\n", len(instDataBytes))
								}

								// Print the accounts involved in this instruction
								fmt.Printf("  Accounts involved in this instruction:\n")

								// Define account role names based on the IDL
								var accountRoles = []string{
									"Pool",
									"User",
									"Global Config",
									"Base Mint (Token)",
									"Quote Mint (SOL)",
									"User Base Token Account",
									"User Quote Token Account",
									"Pool Base Token Account",
									"Pool Quote Token Account",
									"Protocol Fee Recipient",
									"Protocol Fee Recipient Token Account",
									"Base Token Program",
									"Quote Token Program",
									"System Program",
									"Associated Token Program",
									"Event Authority",
									"Program",
								}

								// Check if it's a Buy or Sell instruction to provide better context
								instructionType := "Unknown"
								if bytes.Equal(currentDiscriminator, BuyDiscriminator) {
									instructionType = "Buy"
								} else if bytes.Equal(currentDiscriminator, SellDiscriminator) {
									instructionType = "Sell"
								} else if bytes.Equal(currentDiscriminator, CreatePoolDiscriminator) {
									instructionType = "CreatePool"
								}

								fmt.Printf("  Instruction type: %s\n", instructionType)

								// Create a context for token info retrieval
								ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
								defer cancel()

								// Display accounts with their roles
								for j, accIdx := range inst.Accounts {
									if int(accIdx) < len(decodedTx.Message.AccountKeys) {
										account := decodedTx.Message.AccountKeys[accIdx]

										// Add role name if available (within the known roles array bounds)
										if j < len(accountRoles) {
											fmt.Printf("    Account %d [%s]: %s\n", j, accountRoles[j], account.String())

											// Capture the Base Mint for the summary
											if accountRoles[j] == "Base Mint (Token)" {
												tokenAddress := account.String()
												summary.BaseMint = tokenAddress

												// Get token info if we haven't already
												if summary.TokenInfo == nil {
													// Try to get token info from RPC endpoint
													tokenInfo, err := getTokenInfo(ctx, rpcEndpoint, tokenAddress)
													if err != nil {
														fmt.Printf("  Error getting token info: %v\n", err)
													} else if tokenInfo != nil {
														summary.TokenInfo = tokenInfo
														summary.BaseMintName = tokenInfo.Name
														if summary.BaseMintName == "" {
															summary.BaseMintName = tokenInfo.Symbol
														}
													}
												}
											}
										} else {
											fmt.Printf("    Account %d: %s\n", j, account.String())
										}
									}
								}
							}
						}
					}
				}
			} else {
				fmt.Println("  No binary transaction data available")
			}
		}

		// Summarize what we found from both approaches
		if isPumpSwap {
			fmt.Println("\nSummary: This is a PumpSwap transaction")
			if isSwapInstruction {
				fmt.Println("Operation: Swap")
				if isBuy {
					fmt.Println("Direction: Buy (SOL → Token)")
				} else if isSell {
					fmt.Println("Direction: Sell (Token → SOL)")
				} else {
					fmt.Println("Direction: Unknown")
				}
			} else {
				fmt.Println("Operation: Other PumpSwap operation (not a swap)")
			}
		} else {
			fmt.Println("Summary: Not a PumpSwap transaction or could not detect PumpSwap operations")
		}

		// Display optimized transaction summary in a clear, formatted box
		fmt.Println("\n┌────────────────── TRANSACTION SUMMARY ──────────────────┐")

		// Display token details if available
		tokenName := "Unknown Token"
		if summary.TokenInfo != nil {
			if summary.TokenInfo.Name != "" {
				tokenName = summary.TokenInfo.Name
			} else if summary.TokenInfo.Symbol != "" {
				tokenName = summary.TokenInfo.Symbol
			}
		}

		fmt.Printf("│ Operation:  %-47s │\n", summary.Operation)
		fmt.Printf("│ Direction:  %-47s │\n", summary.Direction)
		fmt.Printf("│ Base Mint:  %-47s │\n", summary.BaseMint)
		fmt.Printf("│ Token Name: %-47s │\n", tokenName)

		// Show token symbol if available
		if summary.TokenInfo != nil && summary.TokenInfo.Symbol != "" {
			fmt.Printf("│ Symbol:     %-47s │\n", summary.TokenInfo.Symbol)
		}

		// Format amounts differently based on whether it's Buy or Sell
		if isBuy {
			// For Buy, SOL amount in (lamports) and token amount out
			solAmount := float64(summary.AmountIn) / 1_000_000_000 // Convert lamports to SOL
			fmt.Printf("│ Amount In:  %-12.9f SOL %-30s │\n", solAmount, "")

			// Use token decimals if available, otherwise default to 6
			tokenDecimals := 6
			if summary.TokenInfo != nil && summary.TokenInfo.Decimals > 0 {
				tokenDecimals = int(summary.TokenInfo.Decimals)
			}
			tokenAmount := float64(summary.AmountOut) / math.Pow(10, float64(tokenDecimals))
			symbolStr := "Token"
			if summary.TokenInfo != nil && summary.TokenInfo.Symbol != "" {
				symbolStr = strings.TrimSpace(summary.TokenInfo.Symbol)
			}
			fmt.Printf("│ Amount Out: %-12.*f %s %-26s │\n", tokenDecimals, tokenAmount, symbolStr, "")
		} else if isSell {
			// For Sell, token amount in and SOL amount out (lamports)
			tokenDecimals := 6
			if summary.TokenInfo != nil && summary.TokenInfo.Decimals > 0 {
				tokenDecimals = int(summary.TokenInfo.Decimals)
			}
			tokenAmount := float64(summary.AmountIn) / math.Pow(10, float64(tokenDecimals))
			symbolStr := "Token"
			if summary.TokenInfo != nil && summary.TokenInfo.Symbol != "" {
				symbolStr = strings.TrimSpace(summary.TokenInfo.Symbol)
			}
			fmt.Printf("│ Amount In:  %-12.*f %s %-26s │\n", tokenDecimals, tokenAmount, symbolStr, "")

			solAmount := float64(summary.AmountOut) / 1_000_000_000
			fmt.Printf("│ Amount Out: %-12.9f SOL %-30s │\n", solAmount, "")
		} else {
			// Unknown or other operation type
			fmt.Printf("│ Amount In:  %-47d │\n", summary.AmountIn)
			fmt.Printf("│ Amount Out: %-47d │\n", summary.AmountOut)
		}

		// Add separator for token information section if we have any social info
		hasTokenInfo := summary.TokenInfo != nil &&
			(summary.TokenInfo.Description != "" ||
				summary.TokenInfo.Website != "" ||
				summary.TokenInfo.Twitter != "" ||
				summary.TokenInfo.Telegram != "" ||
				summary.TokenInfo.Image != "")

		if hasTokenInfo {
			fmt.Println("├──────────────── TOKEN SOCIAL INFORMATION ───────────────┤")

			// Add token description if available
			if summary.TokenInfo.Description != "" {
				desc := summary.TokenInfo.Description
				// Handle multi-line description by truncating and adding ellipsis
				if len(desc) > 47 {
					// Print first line with ellipsis
					fmt.Printf("│ Description: %-46s │\n", desc[:44]+"...")

					// Print additional lines if really long
					if len(desc) > 90 {
						fmt.Printf("│             %-47s │\n", desc[44:90]+"...")
					} else if len(desc) > 44 {
						fmt.Printf("│             %-47s │\n", desc[44:])
					}
				} else {
					fmt.Printf("│ Description: %-46s │\n", desc)
				}
			} else {
				fmt.Printf("│ Description: %-46s │\n", "Not available")
			}

			// Add image if available
			if summary.TokenInfo.Image != "" {
				// Truncate long image URLs
				imgUrl := summary.TokenInfo.Image
				if len(imgUrl) > 47 {
					imgUrl = imgUrl[:44] + "..."
				}
				fmt.Printf("│ Image:       %-46s │\n", imgUrl)
			}

			// Add website if available
			if summary.TokenInfo.Website != "" {
				website := summary.TokenInfo.Website
				// Format website URL
				if len(website) > 47 {
					website = website[:44] + "..."
				}
				fmt.Printf("│ Website:     %-46s │\n", website)
			} else {
				fmt.Printf("│ Website:     %-46s │\n", "Not available")
			}

			// Add social links with clear formatting
			twitterInfo := "Not available"
			if summary.TokenInfo.Twitter != "" {
				twitterInfo = summary.TokenInfo.Twitter
			}
			fmt.Printf("│ Twitter:     %-46s │\n", twitterInfo)

			telegramInfo := "Not available"
			if summary.TokenInfo.Telegram != "" {
				telegramInfo = summary.TokenInfo.Telegram
			}
			fmt.Printf("│ Telegram:    %-46s │\n", telegramInfo)
		}

		fmt.Println("└──────────────────────────────────────────────────────────┘")

		// If we have token info but didn't show it in the summary (maybe there was a lot),
		// display additional details here
		if summary.TokenInfo != nil && summary.TokenInfo.Description != "" && len(summary.TokenInfo.Description) > 90 {
			fmt.Println("\nFull Token Description:")
			fmt.Println(summary.TokenInfo.Description)
		}

		if summary.TokenInfo != nil && summary.TokenInfo.Image != "" {
			fmt.Println("\nToken Image URL:")
			fmt.Println(summary.TokenInfo.Image)
		}
	} else {
		fmt.Println("No transaction metadata available")
	}

	fmt.Printf("\nView transaction: https://solscan.io/tx/%s\n", signature)
	fmt.Println("--------------------------------------------------")
}

// decodeSpecificTransaction decodes a specific transaction by signature
func decodeSpecificTransaction(ctx context.Context, rpcEndpoint, signatureStr string) error {
	// Parse signature string to Signature type
	signature, err := solana.SignatureFromBase58(signatureStr)
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	client := rpc.New(rpcEndpoint)

	// Get transaction with retry logic
	var tx *rpc.GetTransactionResult
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		tx, err = client.GetTransaction(ctx, signature, &rpc.GetTransactionOpts{
			Encoding:   solana.EncodingBase64,
			Commitment: rpc.CommitmentConfirmed,
		})

		if err == nil {
			break
		}

		if retryCount < maxRetries-1 {
			fmt.Printf("Failed to get transaction (attempt %d/%d): %v\nRetrying...\n",
				retryCount+1, maxRetries, err)
			time.Sleep(time.Duration(retryCount+1) * time.Second) // Exponential backoff
		}
	}

	if err != nil {
		return fmt.Errorf("failed to get transaction after %d attempts: %w", maxRetries, err)
	}

	fmt.Printf("Decoding transaction: %s\n", signatureStr)
	analyzeTransactionWithRPC(tx, signatureStr, rpcEndpoint)
	return nil
}

// printUsage displays the program's usage information
func printUsage() {
	fmt.Println(`
PumpFun AMM Transaction Decoder

Usage:
  tx_decoder [command] [options]

Commands:
  decode [pool_address]       Analyze transactions for a PumpFun AMM pool
                              Default pool: H9d3XHfvMGfoohydEpqh4w3mopnvjCRzE9VqaiHKdqs7
  
  decode-tx <tx_signature>    Decode a specific transaction by signature

Options:
  -h, --help                  Show this help message

Environment Variables:
  RPC_ENDPOINT                Solana RPC endpoint (default: https://api.mainnet-beta.solana.com)
                              If the default endpoint fails, the program will try several
                              fallback public endpoints automatically.

Examples:
  tx_decoder decode H9d3XHfvMGfoohydEpqh4w3mopnvjCRzE9VqaiHKdqs7
  tx_decoder decode-tx 5SHT9PwxFE7BNmSQwU4KjAW16LQ5aEZmUvWKqSCamXKkWQBs1DcYkEv7ujWgASRUUKqYy6VsM7iTgJkgAygCVPZB
`)
}

// getTokenInfo retrieves detailed token information by mint address
func getTokenInfo(ctx context.Context, rpcEndpoint string, mintAddress string) (*TokenInfo, error) {
	// Check cache first
	if cachedInfo, exists := tokenCache[mintAddress]; exists {
		return cachedInfo, nil
	}

	// Create RPC client
	client := rpc.New(rpcEndpoint)

	// Get token mint account info
	mintPubkey, err := solana.PublicKeyFromBase58(mintAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid mint address: %w", err)
	}

	// Get mint account data
	mintAccount, err := client.GetAccountInfo(ctx, mintPubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to get mint account: %w", err)
	}

	if mintAccount.Value == nil || len(mintAccount.Value.Data.GetBinary()) == 0 {
		return nil, fmt.Errorf("mint account data is empty")
	}

	// Parse token mint data
	var mint token.Mint
	err = bin.NewBinDecoder(mintAccount.Value.Data.GetBinary()).Decode(&mint)
	if err != nil {
		return nil, fmt.Errorf("failed to decode mint data: %w", err)
	}

	// Create token info with decimals
	tokenInfo := &TokenInfo{
		Decimals: mint.Decimals,
	}

	// Get token metadata account
	metadataPDA, err := findTokenMetadataAddress(mintPubkey)
	if err != nil {
		// Just return basic token info if metadata can't be found
		tokenCache[mintAddress] = tokenInfo
		return tokenInfo, nil
	}

	// Get metadata account info
	metadataAccount, err := client.GetAccountInfo(ctx, metadataPDA)
	if err != nil || metadataAccount.Value == nil {
		// Just return basic token info if metadata account can't be found
		tokenCache[mintAddress] = tokenInfo
		return tokenInfo, nil
	}

	// Parse metadata
	if len(metadataAccount.Value.Data.GetBinary()) > 0 {
		metadata, err := decodeTokenMetadata(metadataAccount.Value.Data.GetBinary())

		if err == nil && metadata != nil {
			tokenInfo.Name = metadata.Data.Name
			tokenInfo.Symbol = metadata.Data.Symbol

			// Try to fetch extended metadata from URI if available
			if metadata.Data.Uri != "" {
				extendedInfo, err := fetchTokenUriData(metadata.Data.Uri)
				fmt.Printf("extendedInfo: %v\n", extendedInfo)
				if err == nil && extendedInfo != nil {
					// Update with extended info
					if tokenInfo.Name == "" {
						tokenInfo.Name = extendedInfo.Name
					}
					if tokenInfo.Symbol == "" {
						tokenInfo.Symbol = extendedInfo.Symbol
					}
					tokenInfo.Description = extendedInfo.Description
					tokenInfo.Image = extendedInfo.Image
					tokenInfo.Website = extendedInfo.Website
					tokenInfo.Twitter = extendedInfo.Twitter
					tokenInfo.Telegram = extendedInfo.Telegram

					// Check extensions if main fields are empty
					if tokenInfo.Website == "" {
						tokenInfo.Website = extendedInfo.Extensions.Website
					}
					if tokenInfo.Twitter == "" {
						tokenInfo.Twitter = extendedInfo.Extensions.Twitter
					}
					if tokenInfo.Telegram == "" {
						tokenInfo.Telegram = extendedInfo.Extensions.Telegram
					}
				}
			}
		}
	}

	// Trim whitespace from fields
	tokenInfo.Name = strings.TrimSpace(tokenInfo.Name)
	tokenInfo.Symbol = strings.TrimSpace(tokenInfo.Symbol)

	// Cache the result
	tokenCache[mintAddress] = tokenInfo
	return tokenInfo, nil
}

// findTokenMetadataAddress calculates the PDA for a token's metadata account
func findTokenMetadataAddress(mint solana.PublicKey) (solana.PublicKey, error) {
	metadataProgramID := solana.MustPublicKeyFromBase58(tokenMetadataProgramID)
	seeds := [][]byte{
		[]byte("metadata"),
		metadataProgramID.Bytes(),
		mint.Bytes(),
	}

	addr, _, err := solana.FindProgramAddress(seeds, metadataProgramID)
	if err != nil {
		return solana.PublicKey{}, fmt.Errorf("failed to find PDA: %w", err)
	}

	return addr, nil
}

// decodeTokenMetadata decodes the binary metadata into a structured format
func decodeTokenMetadata(data []byte) (*TokenMetadata, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("metadata too short")
	}

	// The data format follows this pattern:
	// byte 0: key (1 byte)
	// next 32 bytes: update authority
	// next 32 bytes: mint
	// then variable length name, symbol, uri

	// This is a simplified decoder that may not work for all tokens
	// A complete implementation would use the proper layout from the metaplex codebase
	metadata := &TokenMetadata{
		Key: data[0],
	}

	if len(data) < 65 {
		return metadata, nil
	}

	updateAuth := solana.PublicKey{}
	copy(updateAuth[:], data[1:33])
	metadata.UpdateAuth = updateAuth.String()

	mint := solana.PublicKey{}
	copy(mint[:], data[33:65])
	metadata.Mint = mint.String()

	// Attempt to extract name, symbol, URI
	// This is very simplified and may not work for all tokens
	if len(data) > 69 {
		nameLen := binary.LittleEndian.Uint32(data[65:69])
		startPos := 69

		if len(data) >= startPos+int(nameLen) {
			metadata.Data.Name = string(data[startPos : startPos+int(nameLen)])
			startPos += int(nameLen)

			if len(data) >= startPos+4 {
				symbolLen := binary.LittleEndian.Uint32(data[startPos : startPos+4])
				startPos += 4

				if len(data) >= startPos+int(symbolLen) {
					metadata.Data.Symbol = string(data[startPos : startPos+int(symbolLen)])
					startPos += int(symbolLen)

					if len(data) >= startPos+4 {
						uriLen := binary.LittleEndian.Uint32(data[startPos : startPos+4])
						startPos += 4

						if len(data) >= startPos+int(uriLen) {
							metadata.Data.Uri = string(data[startPos : startPos+int(uriLen)])
						}
					}
				}
			}
		}
	}

	return metadata, nil
}

// fetchTokenUriData retrieves extended token metadata from URI
func fetchTokenUriData(uri string) (*TokenUriData, error) {
	if uri == "" {
		return nil, fmt.Errorf("empty URI")
	}

	// Clean up the URI
	uri = strings.TrimSpace(uri)

	// Handle IPFS URIs
	if strings.HasPrefix(uri, "ipfs://") {
		uri = strings.Replace(uri, "ipfs://", "https://ipfs.io/ipfs/", 1)
	}

	// Set up a timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URI: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if it's a JSON response
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/json") || strings.Contains(uri, ".json") {
		var data TokenUriData
		err = json.Unmarshal(body, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		return &data, nil
	} else if strings.HasPrefix(contentType, "image/") {
		// Just a simple image, return minimal data
		return &TokenUriData{
			Image: uri,
		}, nil
	} else {
		// Try to parse as JSON anyway
		var data TokenUriData
		err = json.Unmarshal(body, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse unknown content type: %w", err)
		}
		return &data, nil
	}
}
