package swapper

import (
	"context"
	"fmt"
	"solana-pumpswap-demo/idl/pumpfun/amm"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	ag_solanago "github.com/gagliardetto/solana-go"
	ata "github.com/gagliardetto/solana-go/programs/associated-token-account"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	agc "github.com/gagliardetto/solana-go/rpc"

	"github.com/shopspring/decimal"
)

// Constants for PumpSwap protocol
const (
	PumpSwapProgramID = "pAMMBay6oceH9fJKBRHGP5D4bD4sWpmSwMn52FMfXEA"
	WrappedSOL        = "So11111111111111111111111111111111111111112"
	PumpFunSwapCU     = 300_000
)

// Valid protocol fee recipients - must use one of these
var validProtocolFeeRecipients = []string{
	"62qc2CNXwrYqQScmEdiZFFAnJR262PxWEuNQtxfafNgV",
	"7VtfL8fvgNfhz17qKRMjzQEXgbdpnHHHQRh54R9jP2RJ",
	"7hTckgnGnLQR6sdH7YkqFTAA7VwTfYFaZ6EhEsU3saCX",
	"9rPYyANsfQZw3DnDmKE3YCQF5E8oD89UXoHn9JFEhJUz",
	"AVmoTthdrX6tKt4nDjco2D775W2YK3sDhxPcMmzUAmTY",
	"FWsW1xNtWscwNmKv6wVsU1iTzRN6wmmk3MjxRP5tT7hz",
	"G5UZAVbAf46s7cKWoyKu8kYTip9DGTpbLZ2qa9Aq69dP",
}

// PumpSwapPoolInfo represents the essential pool information
type PumpSwapPoolInfo struct {
	PoolAddress                      string
	BaseMint                         string // Token mint (e.g., PUMP token)
	QuoteMint                        string // WSOL mint
	PoolBaseTokenAccount             string // Pool's token account
	PoolQuoteTokenAccount            string // Pool's SOL account
	ProtocolFeeRecipient             string // Must be one of the valid addresses
	ProtocolFeeRecipientTokenAccount string // Token account for fee recipient
}

// ExecutePumpSwap executes a PumpSwap transaction
func ExecutePumpSwap(
	ctx context.Context,
	rpcEndpoint string,
	privateKeyStr string,
	poolInfo PumpSwapPoolInfo,
	amountInStr string,
	slippage uint64,
	isBuy bool,
) (string, error) {
	// Check if required fields are provided
	if poolInfo.PoolAddress == "" || poolInfo.BaseMint == "" || poolInfo.QuoteMint == "" {
		return "", fmt.Errorf("missing required pool information")
	}

	// 1. Set up RPC client
	client := rpc.New(rpcEndpoint)

	// 2. Parse private key and get public key
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyStr)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}
	publicKey := privateKey.PublicKey()
	fmt.Printf("Using wallet: %s\n", publicKey.String())

	// 3. Determine input and output tokens based on swap direction
	var inMint, outMint solana.PublicKey
	if isBuy {
		// Buying tokens with SOL
		inMint = solana.MustPublicKeyFromBase58(poolInfo.QuoteMint) // SOL
		outMint = solana.MustPublicKeyFromBase58(poolInfo.BaseMint) // Token
	} else {
		// Selling tokens for SOL
		inMint = solana.MustPublicKeyFromBase58(poolInfo.BaseMint)   // Token
		outMint = solana.MustPublicKeyFromBase58(poolInfo.QuoteMint) // SOL
	}

	// 4. Find Associated Token Accounts
	inATA, _, err := solana.FindAssociatedTokenAddress(publicKey, inMint)
	if err != nil {
		return "", fmt.Errorf("failed to find input token account: %w", err)
	}

	outATA, _, err := solana.FindAssociatedTokenAddress(publicKey, outMint)
	if err != nil {
		return "", fmt.Errorf("failed to find output token account: %w", err)
	}

	// 5. Build transaction instructions
	var instructions []solana.Instruction

	// 5.1 Add compute budget instructions
	computeUnitPriceIx, err := computebudget.NewSetComputeUnitPriceInstruction(150000).ValidateAndBuild()
	if err != nil {
		return "", fmt.Errorf("failed to build compute unit price instruction: %w", err)
	}
	fmt.Println("instruction 1")
	instructions = append(instructions, computeUnitPriceIx)

	// #2 - Compute Budget: SetComputeUnitLimit
	instructionNew, err := computebudget.NewSetComputeUnitLimitInstruction(PumpFunSwapCU).ValidateAndBuild()
	if nil != err {
		return "", fmt.Errorf("failed to build compute unit limit instruction: %w", err)
	}
	fmt.Println("instruction 2")
	instructions = append(instructions, instructionNew)

	// 5.2 Create ATA for the token (if needed)
	// First check if the out token ATA exists
	outATAInfo, err := client.GetAccountInfo(ctx, outATA)
	if err != nil || outATAInfo.Value == nil || outATAInfo.Value.Owner.IsZero() {
		// ATA doesn't exist, create it
		createATAIx, err := ata.NewCreateInstruction(
			publicKey, // Funding account
			publicKey, // Wallet address
			outMint,   // Token mint
		).ValidateAndBuild()
		if err != nil {
			return "", fmt.Errorf("failed to build create ATA instruction: %w", err)
		}
		fmt.Println("instruction 3")
		instructions = append(instructions, createATAIx)
	}

	// 5.3 If input is SOL, add instruction to wrap SOL
	var closeIx solana.Instruction
	if isBuy {
		// Convert amount string to lamports
		amountDecimal, err := decimal.NewFromString(amountInStr)
		if err != nil {
			return "", fmt.Errorf("invalid amount: %w", err)
		}
		// SOL has 9 decimals
		amountLamports := amountDecimal.Mul(decimal.New(1, 9)).BigInt().Uint64()

		// Create WSOL account if it doesn't exist
		inATAInfo, err := client.GetAccountInfo(ctx, inATA)
		if err != nil || inATAInfo.Value == nil || inATAInfo.Value.Owner.IsZero() {
			// Create associated token account for WSOL
			createATAIx, err := ata.NewCreateInstruction(
				publicKey,
				publicKey,
				inMint,
			).ValidateAndBuild()
			if err != nil {
				return "", fmt.Errorf("failed to build create WSOL ATA instruction: %w", err)
			}
			fmt.Println("instruction 4")
			instructions = append(instructions, createATAIx)
		}

		// Transfer SOL to wrapped SOL account
		transferIx, err := system.NewTransferInstruction(
			amountLamports,
			publicKey,
			inATA,
		).ValidateAndBuild()
		if err != nil {
			return "", fmt.Errorf("failed to build SOL transfer instruction: %w", err)
		}
		fmt.Println("instruction 3: transfer")
		instructions = append(instructions, transferIx)

		// Sync native instruction to update wrapped SOL balance
		syncNativeData := []byte{17} // SyncNative instruction code is 17

		// Create the AccountMetaSlice properly
		accountMetas := solana.AccountMetaSlice{
			{PublicKey: inATA, IsSigner: false, IsWritable: true},
		}

		syncNativeIx := solana.NewInstruction(
			token.ProgramID,
			accountMetas,
			syncNativeData,
		)
		fmt.Println("instruction 4: syncNativeIx")
		instructions = append(instructions, syncNativeIx)

		// Add close wrapped SOL at the end of transaction to recover rent
		closeIx, err = token.NewCloseAccountInstruction(
			inATA,     // The account to close
			publicKey, // Rent destination
			publicKey, // Owner
			[]solana.PublicKey{},
		).ValidateAndBuild()
		if err != nil {
			return "", fmt.Errorf("failed to build close account instruction: %w", err)
		}

		// We'll append this after the swap instruction
	}

	// Convert amount string to proper unit
	amountDecimal, err := decimal.NewFromString(amountInStr)
	if err != nil {
		return "", fmt.Errorf("invalid amount: %w", err)
	}

	var amountInLamports uint64
	if isBuy {
		// If buying tokens with SOL, convert SOL to lamports (9 decimals)
		amountInLamports = amountDecimal.Mul(decimal.New(1, 9)).BigInt().Uint64()
	} else {
		// If selling tokens, use token's decimals (usually 6 for most SPL tokens)
		amountInLamports = amountDecimal.Mul(decimal.New(1, 6)).BigInt().Uint64()
	}

	// Define the standard fee rate for PumpFun AMM
	feeRate := uint64(2500) // 0.25%

	// Get token balances (reserves) for the pool

	// Replace hardcoded accounts with the ones from poolInfo
	poolBaseAccount := solana.MustPublicKeyFromBase58(poolInfo.PoolBaseTokenAccount)
	poolQuoteAccount := solana.MustPublicKeyFromBase58(poolInfo.PoolQuoteTokenAccount)

	// Get the reserves
	fmt.Println("poolBaseAccount", poolBaseAccount)
	reserves, err := GetMultipleTokenBalances(ctx, client, poolBaseAccount, poolQuoteAccount)
	if err != nil {
		return "", fmt.Errorf("failed to get pool reserves: %w", err)
	}

	fmt.Println("reserves: ", reserves)

	if len(reserves) < 2 {
		return "", fmt.Errorf("failed to get both pool reserves")
	}

	// Calculate minimum amount out based on slippage
	// The direction parameter should be true for buy, false for sell
	minAmountOut, _, err := CalculateMinAmountOut(uint32(slippage), amountInLamports, isBuy, reserves[0], reserves[1], feeRate)
	if err != nil {
		return "", fmt.Errorf("failed to calculate minimum amount out: %w", err)
	}

	//
	fmt.Println("outATA is:", outATA)
	fmt.Println("inATA is:", inATA)

	// Create the swap instruction
	swapIx, err := createPumpSwapInstruction(
		solana.MustPublicKeyFromBase58(poolInfo.PoolAddress),
		publicKey,
		solana.MustPublicKeyFromBase58(poolInfo.BaseMint),
		solana.MustPublicKeyFromBase58(poolInfo.QuoteMint),
		outATA,
		inATA,
		solana.MustPublicKeyFromBase58(poolInfo.PoolBaseTokenAccount),
		solana.MustPublicKeyFromBase58(poolInfo.PoolQuoteTokenAccount),
		solana.MustPublicKeyFromBase58(poolInfo.ProtocolFeeRecipient),
		solana.MustPublicKeyFromBase58(poolInfo.ProtocolFeeRecipientTokenAccount),
		minAmountOut,
		amountInLamports,
	)

	if err != nil {
		return "", fmt.Errorf("failed to create swap instruction: %w", err)
	}

	instructions = append(instructions, swapIx)

	// Add the close instruction for wrapped SOL if this is a buy
	if isBuy && closeIx != nil {
		instructions = append(instructions, closeIx)
	}

	// Build, sign and send the transaction
	recent, err := client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("failed to get latest blockhash: %w", err)
	}

	tx, err := solana.NewTransaction(
		instructions,
		recent.Value.Blockhash,
		solana.TransactionPayer(publicKey),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(publicKey) {
			return &privateKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send the transaction
	sig, err := client.SendTransaction(ctx, tx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	return sig.String(), nil
}

// GetMultipleTokenBalances gets token balances for multiple accounts
func GetMultipleTokenBalances(ctx context.Context, client *rpc.Client, accounts ...solana.PublicKey) ([]uint64, error) {
	if len(accounts) == 0 {
		return []uint64{}, nil
	}

	res, err := client.GetMultipleAccountsWithOpts(ctx, accounts, &agc.GetMultipleAccountsOpts{
		Commitment: agc.CommitmentProcessed,
	})
	if err != nil {
		return nil, err
	}

	var amounts []uint64
	for i := range res.Value {
		if res.Value[i] == nil || res.Value[i].Data == nil {
			continue
		}
		var coinBalance token.Account
		if err = bin.NewBinDecoder(res.Value[i].Data.GetBinary()).Decode(&coinBalance); nil != err {
			return nil, err
		}

		amounts = append(amounts, coinBalance.Amount)
	}

	return amounts, nil
}

// func GetMulTokenBalance(ctx context.Context, cli *ag_rpc.Client, accounts ...aSDK.PublicKey) ([]uint64, error) {
// 	res, err := cli.GetMultipleAccountsWithOpts(ctx, accounts, &ag_rpc.GetMultipleAccountsOpts{
// 		Commitment: ag_rpc.CommitmentProcessed,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	var amounts []uint64
// 	for i := range res.Value {
// 		if res.Value[i] == nil || res.Value[i].Data == nil {
// 			continue
// 		}
// 		var coinBalance token.Account
// 		if err = bin.NewBinDecoder(res.Value[i].Data.GetBinary()).Decode(&coinBalance); nil != err {
// 			return nil, err
// 		}

// 		amounts = append(amounts, coinBalance.Amount)
// 	}

// 	return amounts, nil
// }

// CalculateMinAmountOut calculates minimum amount out based on slippage
func CalculateMinAmountOut(slippageBP uint32, amountIn uint64, isBuy bool, tokenAmount, baseAmount, feeRate uint64) (uint64, uint64, error) {
	// Convert everything to decimal for precision
	amountInDec := decimal.NewFromUint64(amountIn)
	tokenAmountDec := decimal.NewFromUint64(tokenAmount)
	baseAmountDec := decimal.NewFromUint64(baseAmount)
	feeRateDec := decimal.NewFromUint64(feeRate).Div(decimal.NewFromUint64(1000000)) // Assuming fee rate denominator is 1000000

	// Calculate fee
	feeDec := amountInDec.Mul(feeRateDec)
	amountInAfterFee := amountInDec.Sub(feeDec)

	var inReserve, outReserve decimal.Decimal
	if isBuy {
		// Buying token with SOL
		inReserve = baseAmountDec
		outReserve = tokenAmountDec
	} else {
		// Selling token for SOL
		inReserve = tokenAmountDec
		outReserve = baseAmountDec
	}

	// Calculate constant product
	k := inReserve.Mul(outReserve)

	// Calculate new in reserve after swap
	newInReserve := inReserve.Add(amountInAfterFee)

	// Calculate new out reserve using constant product formula
	newOutReserve := k.Div(newInReserve)

	// Calculate amount out
	amountOut := outReserve.Sub(newOutReserve)

	// Apply slippage to get minimum amount out
	minAmountOut := amountOut.Mul(decimal.NewFromUint64(10000).Sub(decimal.NewFromUint64(uint64(slippageBP)))).Div(decimal.NewFromUint64(10000))

	if minAmountOut.LessThanOrEqual(decimal.Zero) || amountOut.LessThanOrEqual(decimal.Zero) {
		return 0, 0, fmt.Errorf("calculated amount out is zero or negative")
	}

	return uint64(minAmountOut.IntPart()), uint64(amountOut.IntPart()), nil
}

// createPumpSwapInstruction creates a PumpSwap instruction
func createPumpSwapInstruction(
	pool solana.PublicKey,
	user solana.PublicKey,
	baseMint solana.PublicKey,
	quoteMint solana.PublicKey,
	userBaseTokenAccount solana.PublicKey,
	userQuoteTokenAccount solana.PublicKey,
	poolBaseTokenAccount solana.PublicKey,
	poolQuoteTokenAccount solana.PublicKey,
	protocolFeeRecipient solana.PublicKey,
	protocolFeeRecipientTokenAccount solana.PublicKey,
	minAmountOut uint64,
	amountIn uint64,
) (solana.Instruction, error) {

	baseTokenProgram := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	quoteTokenProgram := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	// pumpswap的base是meme quote是sol
	swapParam := &amm.SwapParam{
		TokenAmount1:                     minAmountOut,
		TokenAmount2:                     amountIn,
		Direction:                        amm.BuyDirection,
		Pool:                             pool,
		User:                             user, // Use the actual public key directly
		BaseMint:                         baseMint,
		QuoteMint:                        quoteMint,
		UserBaseTokenAccount:             userBaseTokenAccount,
		UserQuoteTokenAccount:            userQuoteTokenAccount,
		PoolBaseTokenAccount:             poolBaseTokenAccount,
		PoolQuoteTokenAccount:            poolQuoteTokenAccount,
		ProtocolFeeRecipient:             protocolFeeRecipient,
		ProtocolFeeRecipientTokenAccount: protocolFeeRecipientTokenAccount,
		BaseTokenProgram:                 ag_solanago.MustPublicKeyFromBase58(baseTokenProgram),
		QuoteTokenProgram:                ag_solanago.MustPublicKeyFromBase58(quoteTokenProgram),
	}

	swapInstruction, _ := amm.NewSwapInstruction(swapParam)
	return swapInstruction, nil
}

// encodeU64 encodes a uint64 into a little-endian byte array
func encodeU64(val uint64) []byte {
	buf := make([]byte, 8)
	for i := 0; i < 8; i++ {
		buf[i] = byte(val >> (i * 8))
	}
	return buf
}
