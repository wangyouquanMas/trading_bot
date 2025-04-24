package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"solana-pumpswap-demo/idl/pumpfun/amm"

	// "github.com/blocto/solana-go-sdk/rpc"

	"github.com/gagliardetto/solana-go"

	bin "github.com/gagliardetto/binary"
	aSDK "github.com/gagliardetto/solana-go"
	ag_solanago "github.com/gagliardetto/solana-go"
	ata "github.com/gagliardetto/solana-go/programs/associated-token-account"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	ag_rpc "github.com/gagliardetto/solana-go/rpc"
	"github.com/shopspring/decimal"
)

// Constants for PumpSwap protocol
const (
	PumpSwapProgramID          = "5JQ8Mhdp2wv3HWcfjU9iyiEufkQHKzs7GxwULFTcTKvm" // PumpSwap program ID
	PumpAmmGlobalConfigAddress = "ADyA8hdefvWN2dbGGWFotbzWxrAvLW83WG6QCVXvJKqw" // Global config address
	PumpAmmEventAuthority      = "GS4CU59F31iL7aR2Q8zVS8DRrcRnXX1yjQ66TqNVQnaR" // Event authority
	WrappedSOL                 = "So11111111111111111111111111111111111111112"  // Wrapped SOL address
	PumpFunSwapCU              = 300_000
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

var (
	FeeRateDenominatorValue = decimal.NewFromUint64(1000000)
	AllBpDecimal            = decimal.NewFromInt(10000)
)

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

func main() {
	// Get RPC endpoint and private key from environment or use defaults
	rpcEndpoint := os.Getenv("RPC_ENDPOINT")
	if rpcEndpoint == "" {
		rpcEndpoint = "https://api.mainnet-beta.solana.com"
	}

	privateKeyStr := os.Getenv("PRIVATE_KEY")
	if privateKeyStr == "" {
		log.Fatal("PRIVATE_KEY environment variable is required")
	}

	// Initialize hardcoded pool information
	// In a real implementation, you would fetch this from an API or database
	poolInfo := PumpSwapPoolInfo{
		PoolAddress:                      "H9d3XHfvMGfoohydEpqh4w3mopnvjCRzE9VqaiHKdqs7",
		BaseMint:                         "4TBi66vi32S7J8X1A6eWfaLHYmUXu7CStcEmsJQdpump", // PUMP token
		QuoteMint:                        WrappedSOL,                                     // Wrapped SOL
		PoolBaseTokenAccount:             "HLuXpKVQUcXrCzBYPisB7LVj8Piaaii6qv4EbB9qgvPt", // Example
		PoolQuoteTokenAccount:            "9Eqe1Lm6yvvNq8JymWPo5JgKJQK8Dz4xA1NzQxPBLZjs", // Example
		ProtocolFeeRecipient:             validProtocolFeeRecipients[0],                  // Using the first valid recipient
		ProtocolFeeRecipientTokenAccount: "7NXr6RhzBFo4Ki9pUEVyD3fULvTw7PzGiwzxNk3gboYh", // Example
	}

	// Transaction parameters
	amountIn := "0.001"     // SOL amount to swap
	slippage := uint64(100) // 1% slippage (in basis points)
	isBuy := true           // We're buying PUMP tokens with SOL

	// Execute the swap
	txSignature, err := executePumpSwap(
		context.Background(),
		rpcEndpoint,
		privateKeyStr,
		poolInfo,
		amountIn,
		slippage,
		isBuy,
	)
	if err != nil {
		log.Fatalf("Failed to execute swap: %v", err)
	}

	fmt.Printf("Transaction successful! Signature: %s\n", txSignature)
	fmt.Printf("Check the transaction: https://solscan.io/tx/%s\n", txSignature)
}

type CreateMarketTx struct {
	UserId            uint64
	ChainId           uint64
	UserWalletId      uint32
	UserWalletAddress string
	AmountIn          string
	IsAntiMev         bool
	IsAutoSlippage    bool
	Slippage          uint32
	GasType           int32
	TradePoolName     string
	InDecimal         uint8
	OutDecimal        uint8
	InTokenCa         string
	OutTokenCa        string
	PairAddr          string
	Price             string
	UsePriceLimit     bool
	InTokenProgram    string
	OutTokenProgram   string
}

// executePumpSwap executes a PumpSwap transaction
func executePumpSwap(
	ctx context.Context,
	rpcEndpoint string,
	privateKeyStr string,
	poolInfo PumpSwapPoolInfo,
	amountInStr string,
	slippage uint64,
	isBuy bool,
) (string, error) {
	var minAmountOut uint64

	// 1. Set up RPC client
	client := ag_rpc.New(rpcEndpoint)

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
	instructions = append(instructions, computeUnitPriceIx)

	// #2 - Compute Budget: SetComputeUnitLimit
	instructionNew, err := computebudget.NewSetComputeUnitLimitInstruction(PumpFunSwapCU).ValidateAndBuild()
	if nil != err {
		return "", fmt.Errorf("failed to build compute unit limit instruction: %w", err)
	}
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
		instructions = append(instructions, createATAIx)
	}

	// 5.3 If input is SOL, add instruction to wrap SOL
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
		instructions = append(instructions, transferIx)

		// Get the Token program ID
		tokenProgramID := token.ProgramID

		// Sync native instruction to update wrapped SOL balance
		syncNativeData := []byte{17} // SyncNative instruction code is 17

		// Create the AccountMetaSlice properly
		accountMetas := solana.AccountMetaSlice{
			{PublicKey: inATA, IsSigner: false, IsWritable: true},
		}

		syncNativeIx := ag_solanago.NewInstruction(
			tokenProgramID,
			accountMetas,
			syncNativeData,
		)

		instructions = append(instructions, syncNativeIx)

		// Add close wrapped SOL at the end of transaction to recover rent
		closeIx, err := token.NewCloseAccountInstruction(
			inATA,     // The account to close
			publicKey, // Rent destination
			publicKey, // Owner
			[]ag_solanago.PublicKey{},
		).ValidateAndBuild()
		if err != nil {
			return "", fmt.Errorf("failed to build close account instruction: %w", err)
		}

		// Add this at the end of all instructions
		// We'll append this after the swap instruction
		defer func() {
			instructions = append(instructions, closeIx)
		}()
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

	// Replace hardcoded wallet address with the actual public key of the signer
	in := &CreateMarketTx{
		AmountIn:   "0.0001",
		Slippage:   10, //TODO: what's the meaning of  it
		InTokenCa:  "So11111111111111111111111111111111111111112",
		OutTokenCa: "4TBi66vi32S7J8X1A6eWfaLHYmUXu7CStcEmsJQdpump",
		PairAddr:   "H9d3XHfvMGfoohydEpqh4w3mopnvjCRzE9VqaiHKdqs7",
	}

	amtDecimal, _ := decimal.NewFromString(in.AmountIn)

	amtDecimal = amtDecimal.Mul(decimal.NewFromInt(1e9))
	amtUint64 := uint64(amtDecimal.IntPart())

	cli := ag_rpc.New(rpcEndpoint)

	//TODO: query these two accounts / RPC [?] /Solana Explorer
	//TODO: how to get it online ?
	PoolBaseTokenAccount := "4vDmqnKLN2jdPGR2DMf5L6C93AG4XbHdfRAXJuironK8"
	PoolQuoteTokenAccount := "5mDDjsgR9HQGFjHGy1cZ7fNYMzqkZ9hBeAJbjkcTZgCt"

	//TODO: PoolBaseTokenAccount   PoolQuoteTokenAccount are PDA account
	poolTokenAccount, _ := ag_solanago.PublicKeyFromBase58(PoolBaseTokenAccount)
	poolSolAccount, _ := ag_solanago.PublicKeyFromBase58(PoolQuoteTokenAccount)

	//TODO: It fetches the current token balances(reserves)
	amounts, err := GetMulTokenBalance(ctx, cli, poolTokenAccount, poolSolAccount)

	//TODO: feeRecipientAccount
	feeRecipientAccount := "7VtfL8fvgNfhz17qKRMjzQEXgbdpnHHHQRh54R9jP2RJ"
	feeRecipientTokenAccount := "7GFUN3bWzJMKMRZ34JLsvcqdssDbXnp589SiE33KVwcC"

	//TODO: baseTOkenProgram
	baseTokenProgram := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	quoteTokenProgram := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"

	//TODO: What parameters are requried for this function
	minAmountOut, _, err = CalcMinAmountOutByAmm(in.Slippage, amtUint64, isBuy, amounts[0], amounts[1], 2500)
	if nil != err {
		return "", err
	}
	// Since we're not using the actual PumpSwap program's instruction data format,
	// we'll create a placeholder for the swap instruction
	// In a real implementation, you would use the PumpSwap program's IDL to build this properly
	// pumpswap的base是meme quote是sol
	swapParam := &amm.SwapParam{
		TokenAmount1:                     minAmountOut,
		TokenAmount2:                     amtUint64,
		Direction:                        amm.BuyDirection,
		Pool:                             ag_solanago.MustPublicKeyFromBase58(in.PairAddr),
		User:                             publicKey, // Use the actual public key directly
		BaseMint:                         ag_solanago.MustPublicKeyFromBase58(in.OutTokenCa),
		QuoteMint:                        ag_solanago.MustPublicKeyFromBase58(in.InTokenCa),
		UserBaseTokenAccount:             outATA,
		UserQuoteTokenAccount:            inATA,
		PoolBaseTokenAccount:             ag_solanago.MustPublicKeyFromBase58(PoolBaseTokenAccount),
		PoolQuoteTokenAccount:            ag_solanago.MustPublicKeyFromBase58(PoolQuoteTokenAccount),
		ProtocolFeeRecipient:             ag_solanago.MustPublicKeyFromBase58(feeRecipientAccount),
		ProtocolFeeRecipientTokenAccount: ag_solanago.MustPublicKeyFromBase58(feeRecipientTokenAccount),
		BaseTokenProgram:                 ag_solanago.MustPublicKeyFromBase58(baseTokenProgram),
		QuoteTokenProgram:                ag_solanago.MustPublicKeyFromBase58(quoteTokenProgram),
	}
	// Create swap instruction data (placeholder - would need actual instruction data format)
	var instructionData []byte
	if isBuy {
		// For buy, instruction index is 0
		instructionData = append([]byte{0}, encodeU64(amountInLamports)...)
		instructionData = append(instructionData, encodeU64(slippage)...)
	} else {
		// For sell, instruction index is 1
		instructionData = append([]byte{1}, encodeU64(amountInLamports)...)
		instructionData = append(instructionData, encodeU64(slippage)...)
	}

	swapInstruction, _ := amm.NewSwapInstruction(swapParam)
	instructions = append(instructions, swapInstruction)

	// 6. Build, sign and send the transaction
	recent, err := client.GetLatestBlockhash(ctx, ag_rpc.CommitmentFinalized)
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

// encodeU64 encodes a uint64 into a little-endian byte array
func encodeU64(val uint64) []byte {
	buf := make([]byte, 8)
	buf[0] = byte(val)
	buf[1] = byte(val >> 8)
	buf[2] = byte(val >> 16)
	buf[3] = byte(val >> 24)
	buf[4] = byte(val >> 32)
	buf[5] = byte(val >> 40)
	buf[6] = byte(val >> 48)
	buf[7] = byte(val >> 56)
	return buf
}

func GetMulTokenBalance(ctx context.Context, cli *ag_rpc.Client, accounts ...aSDK.PublicKey) ([]uint64, error) {
	res, err := cli.GetMultipleAccountsWithOpts(ctx, accounts, &ag_rpc.GetMultipleAccountsOpts{
		Commitment: ag_rpc.CommitmentProcessed,
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

func CalcMinAmountOutByAmm(slipPageBP uint32, amountIn uint64, isBuy bool, tokenAmount, baseAmount uint64, feeRate uint64) (uint64, uint64, error) {
	if isBuy {
		return CalcMinAmountOutBySwap(slipPageBP, amountIn, baseAmount, tokenAmount, feeRate)
	}
	return CalcMinAmountOutBySwap(slipPageBP, amountIn, tokenAmount, baseAmount, feeRate)
}

func CalcMinAmountOutBySwap(slipPageBP uint32, amountIn uint64, totalInTokenAmount, totalOutTokenAmount uint64, feeRate uint64) (uint64, uint64, error) {
	amountDecimal := decimal.NewFromUint64(amountIn)
	feeRateDecimal := decimal.NewFromUint64(feeRate).Div(FeeRateDenominatorValue)
	// 根据交易费率直接将amountIn扣除
	// raydiumv4 看其源码逻辑是放在前 https://github.com/raydium-io/raydium-amm/blob/master/program/src/processor.rs#L2399
	// pumpfun的代码没有官方宣布的开源，但是有自称fork版本的，里面的计算逻辑不是这个amm的方式，而是另一种奇怪的方式 https://github.com/Rust-Sol-Dev/Pump.fun-Smart-Contract/blob/main/programs/bonding_curve/src/state.rs#L101
	feeDecimal := amountDecimal.Mul(feeRateDecimal)
	amountDecimal = amountDecimal.Sub(feeDecimal)

	totalInTokenAmountDecimal := decimal.NewFromUint64(totalInTokenAmount)
	totalOutTokenAmountDecimal := decimal.NewFromUint64(totalOutTokenAmount)

	productDecimal := totalInTokenAmountDecimal.Mul(totalOutTokenAmountDecimal)

	outDecimal := totalOutTokenAmountDecimal.Sub(productDecimal.Div(totalInTokenAmountDecimal.Add(amountDecimal)))

	//根据滑点计算最小输出
	minOut := outDecimal.Mul(AllBpDecimal.Sub(decimal.NewFromUint64(uint64(slipPageBP))).Div(AllBpDecimal))
	if !minOut.IsPositive() || !outDecimal.IsPositive() {
		return 0, 0, nil
	}
	return uint64(minOut.IntPart()), uint64(outDecimal.IntPart()), nil
}
